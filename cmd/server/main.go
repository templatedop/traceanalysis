// Package main provides the API server for the observability analysis framework
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/traceanalysis/rag-temporal/internal/config"
	"github.com/traceanalysis/rag-temporal/internal/types"
	"github.com/traceanalysis/rag-temporal/pkg/framework"
	"github.com/traceanalysis/rag-temporal/pkg/metrics"
	"github.com/traceanalysis/rag-temporal/pkg/workflow"
)

var (
	configPath = flag.String("config", "config.yaml", "Path to configuration file")
	port       = flag.Int("port", 0, "Override server port")
)

func main() {
	flag.Parse()

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	if *port != 0 {
		cfg.Server.Port = *port
	}

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create framework manager
	manager := framework.NewManager(cfg)

	// Initialize framework
	if err := manager.Initialize(ctx); err != nil {
		log.Fatalf("Failed to initialize framework: %v", err)
	}

	// Create server
	server := NewServer(cfg, manager)

	// Handle shutdown gracefully
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("Shutting down...")
		cancel()
		manager.Shutdown(context.Background())
		server.Shutdown(context.Background())
	}()

	// Start server
	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	log.Printf("Starting server on %s", addr)
	log.Printf("Enabled features: %v", cfg.GetEnabledFeatures())
	log.Printf("Enabled agents: %v", cfg.GetEnabledAgents())
	log.Printf("Enabled collectors: %v", cfg.GetEnabledCollectors())

	if err := server.Start(addr); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Server error: %v", err)
	}
}

// Server represents the API server
type Server struct {
	config     *config.Config
	httpServer *http.Server
	manager    *framework.Manager
}

// NewServer creates a new API server
func NewServer(cfg *config.Config, manager *framework.Manager) *Server {
	server := &Server{
		config:  cfg,
		manager: manager,
	}

	// Register routes
	mux := http.NewServeMux()
	server.registerRoutes(mux)

	server.httpServer = &http.Server{
		Handler:      mux,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}

	return server
}

// Start starts the HTTP server
func (s *Server) Start(addr string) error {
	s.httpServer.Addr = addr
	return s.httpServer.ListenAndServe()
}

// Shutdown shuts down the server
func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}

func (s *Server) registerRoutes(mux *http.ServeMux) {
	// Health check endpoints
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/ready", s.handleReady)

	// Metrics endpoint (if enabled)
	if s.config.Features.EnableMetrics {
		mux.Handle(s.config.Metrics.Path, metrics.Handler())
	}

	// Feature info endpoint
	mux.HandleFunc("/api/v1/features", s.handleFeatures)

	// Analysis endpoints
	mux.HandleFunc("/api/v1/analyze", s.handleAnalyze)
	mux.HandleFunc("/api/v1/analyze/incident", s.handleIncidentAnalysis)

	// Temporal workflow endpoints (only if Temporal is enabled)
	if s.config.Features.EnableTemporal {
		mux.HandleFunc("/api/v1/analyze/status", s.handleAnalysisStatus)
		mux.HandleFunc("/api/v1/analyze/result", s.handleAnalysisResult)
		mux.HandleFunc("/api/v1/monitor/start", s.handleStartMonitoring)
		mux.HandleFunc("/api/v1/monitor/stop", s.handleStopMonitoring)
	}

	// RAG endpoints (only if RAG is enabled)
	if s.config.Features.EnableRAG {
		mux.HandleFunc("/api/v1/knowledge/query", s.handleRAGQuery)
		mux.HandleFunc("/api/v1/knowledge/add", s.handleRAGAdd)
		mux.HandleFunc("/api/v1/knowledge/stats", s.handleRAGStats)
	}

	// Query endpoint (natural language)
	mux.HandleFunc("/api/v1/query", s.handleNaturalLanguageQuery)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	health := s.manager.Health(r.Context())
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(health)
}

func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
	health := s.manager.Health(r.Context())
	if !health["initialized"].(bool) {
		http.Error(w, "Not ready", http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ready"})
}

func (s *Server) handleFeatures(w http.ResponseWriter, r *http.Request) {
	features := map[string]interface{}{
		"enabled_features":   s.config.GetEnabledFeatures(),
		"enabled_agents":     s.config.GetEnabledAgents(),
		"enabled_collectors": s.config.GetEnabledCollectors(),
		"feature_flags": map[string]bool{
			"temporal":    s.config.Features.EnableTemporal,
			"rag":         s.config.Features.EnableRAG,
			"caching":     s.config.Features.EnableCaching,
			"metrics":     s.config.Features.EnableMetrics,
			"alerting":    s.config.Features.EnableAlerting,
			"predictions": s.config.Features.EnablePredictions,
		},
		"agents": map[string]bool{
			"trace":  s.config.Agents.Trace.Enabled,
			"metric": s.config.Agents.Metric.Enabled,
			"log":    s.config.Agents.Log.Enabled,
		},
		"collectors": map[string]bool{
			"prometheus": s.config.Collectors.Prometheus.Enabled,
			"jaeger":     s.config.Collectors.Jaeger.Enabled,
			"loki":       s.config.Collectors.Loki.Enabled,
			"tempo":      s.config.Collectors.Tempo.Enabled,
			"otel":       s.config.Collectors.OTEL.Enabled,
		},
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(features)
}

// AnalyzeRequest is the request body for analysis
type AnalyzeRequest struct {
	Query            string   `json:"query"`
	Services         []string `json:"services,omitempty"`
	TimeRangeMinutes int      `json:"time_range_minutes,omitempty"`
	AnalysisType     string   `json:"analysis_type,omitempty"` // trace, metric, log, all
	IncludeRAG       bool     `json:"include_rag"`
	Notify           bool     `json:"notify"`
	Async            bool     `json:"async"`
}

func (s *Server) handleAnalyze(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req AnalyzeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request: %v", err), http.StatusBadRequest)
		return
	}

	// Set defaults
	if req.TimeRangeMinutes == 0 {
		req.TimeRangeMinutes = 60
	}
	if req.AnalysisType == "" {
		req.AnalysisType = "all"
	}

	now := time.Now()
	analysisReq := &types.AnalysisRequest{
		ID:       fmt.Sprintf("req-%d", now.UnixNano()),
		Query:    req.Query,
		Services: req.Services,
		TimeRange: types.TimeRange{
			Start: now.Add(-time.Duration(req.TimeRangeMinutes) * time.Minute),
			End:   now,
		},
		IncludeRAG: req.IncludeRAG,
	}

	// Use framework manager to run analysis
	result, err := s.manager.Analyze(r.Context(), analysisReq)
	if err != nil {
		http.Error(w, fmt.Sprintf("Analysis failed: %v", err), http.StatusInternalServerError)
		return
	}

	// Send alerts if enabled and requested
	if req.Notify && s.config.Features.EnableAlerting {
		for i := range result.Findings {
			finding := &result.Findings[i]
			if finding.Severity == types.SeverityHigh || finding.Severity == types.SeverityCritical {
				if err := s.manager.SendAlert(r.Context(), finding); err != nil {
					log.Printf("Failed to send alert: %v", err)
				}
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// IncidentRequest is the request for incident analysis
type IncidentRequest struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Services    []string `json:"services,omitempty"`
	StartTime   string   `json:"start_time,omitempty"` // RFC3339
	Symptoms    []string `json:"symptoms,omitempty"`
}

func (s *Server) handleIncidentAnalysis(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if !s.config.Features.EnableTemporal {
		http.Error(w, "Incident analysis requires Temporal to be enabled", http.StatusBadRequest)
		return
	}

	workflowClient := s.manager.GetWorkflowClient()
	if workflowClient == nil {
		http.Error(w, "Workflow client not available", http.StatusServiceUnavailable)
		return
	}

	var req IncidentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request: %v", err), http.StatusBadRequest)
		return
	}

	startTime := time.Now().Add(-1 * time.Hour)
	if req.StartTime != "" {
		var err error
		startTime, err = time.Parse(time.RFC3339, req.StartTime)
		if err != nil {
			http.Error(w, fmt.Sprintf("Invalid start_time: %v", err), http.StatusBadRequest)
			return
		}
	}

	input := workflow.IncidentAnalysisInput{
		Title:       req.Title,
		Description: req.Description,
		Services:    req.Services,
		StartTime:   startTime,
		Symptoms:    req.Symptoms,
	}

	workflowID, err := workflowClient.StartIncidentAnalysis(r.Context(), input)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to start analysis: %v", err), http.StatusInternalServerError)
		return
	}

	result, err := workflowClient.GetResult(r.Context(), workflowID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get result: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func (s *Server) handleAnalysisStatus(w http.ResponseWriter, r *http.Request) {
	workflowID := r.URL.Query().Get("workflow_id")
	if workflowID == "" {
		http.Error(w, "workflow_id required", http.StatusBadRequest)
		return
	}

	workflowClient := s.manager.GetWorkflowClient()
	if workflowClient == nil {
		http.Error(w, "Workflow client not available", http.StatusServiceUnavailable)
		return
	}

	status, err := workflowClient.GetStatus(r.Context(), workflowID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get status: %v", err), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]string{
		"workflow_id": workflowID,
		"status":      status,
	})
}

func (s *Server) handleAnalysisResult(w http.ResponseWriter, r *http.Request) {
	workflowID := r.URL.Query().Get("workflow_id")
	if workflowID == "" {
		http.Error(w, "workflow_id required", http.StatusBadRequest)
		return
	}

	workflowClient := s.manager.GetWorkflowClient()
	if workflowClient == nil {
		http.Error(w, "Workflow client not available", http.StatusServiceUnavailable)
		return
	}

	result, err := workflowClient.GetResult(r.Context(), workflowID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get result: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// MonitorRequest is the request for continuous monitoring
type MonitorRequest struct {
	Services        []string `json:"services,omitempty"`
	IntervalMinutes int      `json:"interval_minutes,omitempty"`
	LookbackMinutes int      `json:"lookback_minutes,omitempty"`
}

func (s *Server) handleStartMonitoring(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	workflowClient := s.manager.GetWorkflowClient()
	if workflowClient == nil {
		http.Error(w, "Workflow client not available", http.StatusServiceUnavailable)
		return
	}

	var req MonitorRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request: %v", err), http.StatusBadRequest)
		return
	}

	if req.IntervalMinutes == 0 {
		req.IntervalMinutes = 5
	}
	if req.LookbackMinutes == 0 {
		req.LookbackMinutes = 15
	}

	input := workflow.ContinuousMonitoringInput{
		Services:       req.Services,
		Interval:       time.Duration(req.IntervalMinutes) * time.Minute,
		LookbackWindow: time.Duration(req.LookbackMinutes) * time.Minute,
		AlertThreshold: types.SeverityMedium,
	}

	workflowID, err := workflowClient.StartContinuousMonitoring(r.Context(), input)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to start monitoring: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]string{
		"workflow_id": workflowID,
		"status":      "started",
	})
}

func (s *Server) handleStopMonitoring(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	workflowClient := s.manager.GetWorkflowClient()
	if workflowClient == nil {
		http.Error(w, "Workflow client not available", http.StatusServiceUnavailable)
		return
	}

	workflowID := r.URL.Query().Get("workflow_id")
	if workflowID == "" {
		http.Error(w, "workflow_id required", http.StatusBadRequest)
		return
	}

	if err := workflowClient.CancelWorkflow(r.Context(), workflowID); err != nil {
		http.Error(w, fmt.Sprintf("Failed to stop monitoring: %v", err), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]string{
		"workflow_id": workflowID,
		"status":      "stopped",
	})
}

// RAGQueryRequest is the request for RAG queries
type RAGQueryRequest struct {
	Query string `json:"query"`
	TopK  int    `json:"top_k,omitempty"`
}

func (s *Server) handleRAGQuery(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req RAGQueryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request: %v", err), http.StatusBadRequest)
		return
	}

	if req.TopK == 0 {
		req.TopK = s.config.RAG.TopK
	}

	vectorStore := s.manager.GetVectorStore()
	if vectorStore == nil {
		http.Error(w, "RAG not available", http.StatusServiceUnavailable)
		return
	}

	ragResult, err := vectorStore.Query(r.Context(), &types.RAGQuery{
		Query:    req.Query,
		TopK:     req.TopK,
		MinScore: s.config.RAG.MinScore,
	})
	if err != nil {
		http.Error(w, fmt.Sprintf("Query failed: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"query":     req.Query,
		"documents": ragResult.Documents,
		"count":     ragResult.TotalFound,
	})
}

// RAGAddRequest is the request for adding documents
type RAGAddRequest struct {
	ID       string            `json:"id,omitempty"`
	Type     string            `json:"type,omitempty"`
	Title    string            `json:"title,omitempty"`
	Content  string            `json:"content"`
	Tags     []string          `json:"tags,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

func (s *Server) handleRAGAdd(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req RAGAddRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request: %v", err), http.StatusBadRequest)
		return
	}

	vectorStore := s.manager.GetVectorStore()
	if vectorStore == nil {
		http.Error(w, "RAG not available", http.StatusServiceUnavailable)
		return
	}

	docID := req.ID
	if docID == "" {
		docID = fmt.Sprintf("doc-%d", time.Now().UnixNano())
	}

	doc := &types.RAGDocument{
		ID:       docID,
		Type:     req.Type,
		Title:    req.Title,
		Content:  req.Content,
		Tags:     req.Tags,
		Metadata: req.Metadata,
	}

	if err := vectorStore.Add(r.Context(), doc); err != nil {
		http.Error(w, fmt.Sprintf("Failed to add document: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{
		"id":     docID,
		"status": "created",
	})
}

func (s *Server) handleRAGStats(w http.ResponseWriter, r *http.Request) {
	vectorStore := s.manager.GetVectorStore()
	if vectorStore == nil {
		http.Error(w, "RAG not available", http.StatusServiceUnavailable)
		return
	}

	count, err := vectorStore.Count(r.Context())
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get stats: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"document_count":  count,
		"vector_store":    s.config.RAG.VectorStore.Type,
		"embedding_model": s.config.RAG.EmbeddingModel,
		"dimension":       s.config.RAG.EmbeddingDim,
	})
}

// NaturalLanguageQueryRequest is the request for natural language queries
type NaturalLanguageQueryRequest struct {
	Query            string   `json:"query"`
	Services         []string `json:"services,omitempty"`
	TimeRangeMinutes int      `json:"time_range_minutes,omitempty"`
}

func (s *Server) handleNaturalLanguageQuery(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req NaturalLanguageQueryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request: %v", err), http.StatusBadRequest)
		return
	}

	// Use the framework manager's Query method which handles RAG and caching
	response, err := s.manager.Query(r.Context(), req.Query)
	if err != nil {
		http.Error(w, fmt.Sprintf("Query failed: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"query":           req.Query,
		"response":        response,
		"rag_enabled":     s.config.Features.EnableRAG,
		"caching_enabled": s.config.Features.EnableCaching,
	})
}
