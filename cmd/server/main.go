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
	"github.com/traceanalysis/rag-temporal/pkg/agents"
	"github.com/traceanalysis/rag-temporal/pkg/collectors"
	"github.com/traceanalysis/rag-temporal/pkg/llm"
	"github.com/traceanalysis/rag-temporal/pkg/rag"
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

	// Initialize components
	server, err := NewServer(ctx, cfg)
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}

	// Handle shutdown gracefully
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("Shutting down...")
		cancel()
		server.Shutdown(context.Background())
	}()

	// Start server
	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	log.Printf("Starting server on %s", addr)
	if err := server.Start(addr); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Server error: %v", err)
	}
}

// Server represents the API server
type Server struct {
	config         *config.Config
	httpServer     *http.Server
	workflowClient *workflow.WorkflowClient
	registry       *agents.AgentRegistry
	ragStore       rag.Store
	llmClient      llm.Client
}

// NewServer creates a new API server
func NewServer(ctx context.Context, cfg *config.Config) (*Server, error) {
	// Initialize embedder
	var embedder rag.EmbeddingProvider
	if cfg.LLM.Provider == "ollama" {
		embedder = rag.NewOllamaEmbedder(cfg.LLM.BaseURL, cfg.RAG.EmbeddingModel, cfg.RAG.EmbeddingDim)
	} else {
		embedder = rag.NewNoOpEmbedder(cfg.RAG.EmbeddingDim)
	}

	// Initialize RAG store
	ragStore := rag.NewMemoryStore(embedder)

	// Load knowledge base if configured
	if cfg.RAG.KnowledgeBase != "" {
		kb := rag.NewKnowledgeBase(ragStore, embedder, cfg.RAG.KnowledgeBase)
		if err := kb.LoadFromDirectory(ctx, cfg.RAG.KnowledgeBase); err != nil {
			log.Printf("Warning: failed to load knowledge base: %v", err)
		}
	}

	// Initialize LLM client
	llmClient, err := llm.NewClient(
		cfg.LLM.Provider,
		cfg.LLM.BaseURL,
		cfg.LLM.APIKey,
		cfg.LLM.DefaultModel,
		cfg.LLM.Timeout,
		cfg.LLM.Temperature,
		cfg.LLM.MaxTokens,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create LLM client: %w", err)
	}

	// Initialize workflow client
	workflowClient, err := workflow.NewWorkflowClient(&cfg.Temporal)
	if err != nil {
		log.Printf("Warning: failed to create workflow client: %v", err)
	}

	server := &Server{
		config:         cfg,
		workflowClient: workflowClient,
		registry:       agents.NewAgentRegistry(),
		ragStore:       ragStore,
		llmClient:      llmClient,
	}

	// Register routes
	mux := http.NewServeMux()
	server.registerRoutes(mux)

	server.httpServer = &http.Server{
		Handler:      mux,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}

	return server, nil
}

// Start starts the HTTP server
func (s *Server) Start(addr string) error {
	s.httpServer.Addr = addr
	return s.httpServer.ListenAndServe()
}

// Shutdown shuts down the server
func (s *Server) Shutdown(ctx context.Context) error {
	if s.workflowClient != nil {
		s.workflowClient.Close()
	}
	return s.httpServer.Shutdown(ctx)
}

func (s *Server) registerRoutes(mux *http.ServeMux) {
	// Health check
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/ready", s.handleReady)

	// Analysis endpoints
	mux.HandleFunc("/api/v1/analyze", s.handleAnalyze)
	mux.HandleFunc("/api/v1/analyze/incident", s.handleIncidentAnalysis)
	mux.HandleFunc("/api/v1/analyze/status", s.handleAnalysisStatus)
	mux.HandleFunc("/api/v1/analyze/result", s.handleAnalysisResult)

	// Monitoring endpoints
	mux.HandleFunc("/api/v1/monitor/start", s.handleStartMonitoring)
	mux.HandleFunc("/api/v1/monitor/stop", s.handleStopMonitoring)

	// RAG endpoints
	mux.HandleFunc("/api/v1/knowledge/query", s.handleRAGQuery)
	mux.HandleFunc("/api/v1/knowledge/add", s.handleRAGAdd)

	// Query endpoint (natural language)
	mux.HandleFunc("/api/v1/query", s.handleNaturalLanguageQuery)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})
}

func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
	// Check LLM health
	if err := s.llmClient.Health(r.Context()); err != nil {
		http.Error(w, fmt.Sprintf("LLM not ready: %v", err), http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ready"})
}

// AnalyzeRequest is the request body for analysis
type AnalyzeRequest struct {
	Query       string   `json:"query"`
	Services    []string `json:"services,omitempty"`
	TimeRangeMinutes int `json:"time_range_minutes,omitempty"`
	AnalysisType string  `json:"analysis_type,omitempty"` // trace, metric, log, all
	IncludeRAG  bool     `json:"include_rag"`
	Notify      bool     `json:"notify"`
	Async       bool     `json:"async"`
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
	timeRange := types.TimeRange{
		Start: now.Add(-time.Duration(req.TimeRangeMinutes) * time.Minute),
		End:   now,
	}

	input := workflow.ObservabilityAnalysisInput{
		Query:        req.Query,
		Services:     req.Services,
		TimeRange:    timeRange,
		AnalysisType: req.AnalysisType,
		IncludeRAG:   req.IncludeRAG,
		Notify:       req.Notify,
	}

	if s.workflowClient == nil {
		http.Error(w, "Workflow client not available", http.StatusServiceUnavailable)
		return
	}

	workflowID, err := s.workflowClient.StartAnalysis(r.Context(), input)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to start analysis: %v", err), http.StatusInternalServerError)
		return
	}

	if req.Async {
		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(map[string]string{
			"workflow_id": workflowID,
			"status":      "started",
		})
		return
	}

	// Wait for result
	result, err := s.workflowClient.GetResult(r.Context(), workflowID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get result: %v", err), http.StatusInternalServerError)
		return
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

	if s.workflowClient == nil {
		http.Error(w, "Workflow client not available", http.StatusServiceUnavailable)
		return
	}

	workflowID, err := s.workflowClient.StartIncidentAnalysis(r.Context(), input)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to start analysis: %v", err), http.StatusInternalServerError)
		return
	}

	result, err := s.workflowClient.GetResult(r.Context(), workflowID)
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

	if s.workflowClient == nil {
		http.Error(w, "Workflow client not available", http.StatusServiceUnavailable)
		return
	}

	status, err := s.workflowClient.GetStatus(r.Context(), workflowID)
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

	if s.workflowClient == nil {
		http.Error(w, "Workflow client not available", http.StatusServiceUnavailable)
		return
	}

	result, err := s.workflowClient.GetResult(r.Context(), workflowID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get result: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// MonitorRequest is the request for continuous monitoring
type MonitorRequest struct {
	Services       []string `json:"services,omitempty"`
	IntervalMinutes int     `json:"interval_minutes,omitempty"`
	LookbackMinutes int     `json:"lookback_minutes,omitempty"`
}

func (s *Server) handleStartMonitoring(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
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

	if s.workflowClient == nil {
		http.Error(w, "Workflow client not available", http.StatusServiceUnavailable)
		return
	}

	workflowID, err := s.workflowClient.StartContinuousMonitoring(r.Context(), input)
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

	workflowID := r.URL.Query().Get("workflow_id")
	if workflowID == "" {
		http.Error(w, "workflow_id required", http.StatusBadRequest)
		return
	}

	if s.workflowClient == nil {
		http.Error(w, "Workflow client not available", http.StatusServiceUnavailable)
		return
	}

	if err := s.workflowClient.CancelWorkflow(r.Context(), workflowID); err != nil {
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
	Query string   `json:"query"`
	TopK  int      `json:"top_k,omitempty"`
	Types []string `json:"types,omitempty"`
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
		req.TopK = 5
	}

	result, err := s.ragStore.Query(r.Context(), &types.RAGQuery{
		Query: req.Query,
		TopK:  req.TopK,
		Types: req.Types,
	})
	if err != nil {
		http.Error(w, fmt.Sprintf("Query failed: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// RAGAddRequest is the request for adding documents
type RAGAddRequest struct {
	Type     string            `json:"type"`
	Title    string            `json:"title"`
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

	doc := &types.RAGDocument{
		Type:     req.Type,
		Title:    req.Title,
		Content:  req.Content,
		Tags:     req.Tags,
		Metadata: req.Metadata,
	}

	if err := s.ragStore.Add(r.Context(), doc); err != nil {
		http.Error(w, fmt.Sprintf("Failed to add document: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{
		"id":     doc.ID,
		"status": "created",
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

	// Get RAG context
	ragResult, _ := s.ragStore.Query(r.Context(), &types.RAGQuery{
		Query: req.Query,
		TopK:  5,
	})

	var ragContext string
	if ragResult != nil {
		for _, doc := range ragResult.Documents {
			ragContext += fmt.Sprintf("--- %s ---\n%s\n\n", doc.Title, doc.Content)
		}
	}

	// Query LLM
	systemPrompt := llm.NaturalLanguageQueryPrompt
	userPrompt := fmt.Sprintf("Query: %s\n\nKnowledge Base Context:\n%s", req.Query, ragContext)

	resp, err := s.llmClient.Complete(r.Context(), &types.LLMRequest{
		SystemPrompt: systemPrompt,
		UserPrompt:   userPrompt,
		Temperature:  0.3,
	})
	if err != nil {
		http.Error(w, fmt.Sprintf("Query failed: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"query":    req.Query,
		"response": resp.Content,
		"sources":  ragResult.TotalFound,
	})
}

// Ensure collectors are not unused
var _ = collectors.NewPrometheusCollector
var _ = collectors.NewJaegerCollector
var _ = collectors.NewLokiCollector
