// Package framework provides the main application framework with configurable features
package framework

import (
	"context"
	"fmt"
	"log"
	"sync"

	"github.com/traceanalysis/rag-temporal/internal/config"
	"github.com/traceanalysis/rag-temporal/internal/types"
	"github.com/traceanalysis/rag-temporal/pkg/agents"
	"github.com/traceanalysis/rag-temporal/pkg/alerting"
	"github.com/traceanalysis/rag-temporal/pkg/collectors"
	"github.com/traceanalysis/rag-temporal/pkg/llm"
	"github.com/traceanalysis/rag-temporal/pkg/rag"
	"github.com/traceanalysis/rag-temporal/pkg/workflow"
)

// Manager handles initialization and lifecycle of framework components
type Manager struct {
	cfg *config.Config

	// Core components (always initialized)
	llmClient llm.Client

	// Optional components (based on feature flags)
	ragStore       rag.Store
	knowledgeBase  *rag.KnowledgeBase
	cache          *llm.MemoryCache
	cachedClient   *llm.CachedClient
	workflowClient *workflow.WorkflowClient
	alertManager   *alerting.AlertManager

	// Agents
	traceAgent  *agents.TraceAgent
	metricAgent *agents.MetricAgent
	logAgent    *agents.LogAgent

	// Collectors
	prometheusCollector *collectors.PrometheusCollector
	jaegerCollector     *collectors.JaegerCollector
	lokiCollector       *collectors.LokiCollector

	// State
	initialized bool
	mu          sync.RWMutex
}

// NewManager creates a new framework manager
func NewManager(cfg *config.Config) *Manager {
	return &Manager{
		cfg: cfg,
	}
}

// Initialize initializes all enabled components
func (m *Manager) Initialize(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.initialized {
		return nil
	}

	log.Printf("Initializing framework with features: %v", m.cfg.GetEnabledFeatures())
	log.Printf("Enabled agents: %v", m.cfg.GetEnabledAgents())
	log.Printf("Enabled collectors: %v", m.cfg.GetEnabledCollectors())

	// Initialize LLM client (always needed)
	if err := m.initLLM(); err != nil {
		return fmt.Errorf("failed to initialize LLM client: %w", err)
	}

	// Initialize caching if enabled
	if m.cfg.Features.EnableCaching {
		if err := m.initCache(ctx); err != nil {
			return fmt.Errorf("failed to initialize cache: %w", err)
		}
	}

	// Initialize RAG if enabled
	if m.cfg.Features.EnableRAG {
		if err := m.initRAG(ctx); err != nil {
			return fmt.Errorf("failed to initialize RAG: %w", err)
		}
	}

	// Initialize collectors
	if err := m.initCollectors(); err != nil {
		return fmt.Errorf("failed to initialize collectors: %w", err)
	}

	// Initialize agents
	if err := m.initAgents(); err != nil {
		return fmt.Errorf("failed to initialize agents: %w", err)
	}

	// Initialize Temporal if enabled
	if m.cfg.Features.EnableTemporal {
		if err := m.initTemporal(); err != nil {
			log.Printf("Warning: failed to initialize Temporal: %v", err)
			// Don't fail - Temporal is optional
		}
	}

	// Initialize alerting if enabled
	if m.cfg.Features.EnableAlerting {
		if err := m.initAlerting(); err != nil {
			return fmt.Errorf("failed to initialize alerting: %w", err)
		}
	}

	m.initialized = true
	log.Println("Framework initialization complete")
	return nil
}

// initLLM initializes the LLM client
func (m *Manager) initLLM() error {
	log.Printf("Initializing LLM client (provider: %s, model: %s)...", m.cfg.LLM.Provider, m.cfg.LLM.DefaultModel)
	client, err := llm.NewClient(
		m.cfg.LLM.Provider,
		m.cfg.LLM.BaseURL,
		m.cfg.LLM.APIKey,
		m.cfg.LLM.DefaultModel,
		m.cfg.LLM.Timeout,
		m.cfg.LLM.Temperature,
		m.cfg.LLM.MaxTokens,
	)
	if err != nil {
		return err
	}
	m.llmClient = client
	return nil
}

// initCache initializes the LLM response cache
func (m *Manager) initCache(ctx context.Context) error {
	log.Printf("Initializing LLM cache (max entries: %d, TTL: %s)...", m.cfg.Caching.MaxEntries, m.cfg.Caching.TTL)
	m.cache = llm.NewMemoryCache(m.cfg.Caching.MaxEntries)
	m.cachedClient = llm.NewCachedClient(m.llmClient, m.cache, m.cfg.Caching.TTL)

	// Start cleanup routine
	m.cache.StartCleanupRoutine(ctx, m.cfg.Caching.CleanupInterval)

	return nil
}

// initRAG initializes the RAG system
func (m *Manager) initRAG(ctx context.Context) error {
	log.Printf("Initializing RAG system (store type: %s)...", m.cfg.RAG.VectorStore.Type)

	// Create embedder
	var embedder rag.EmbeddingProvider
	switch m.cfg.RAG.EmbeddingProvider {
	case "ollama":
		embedder = rag.NewOllamaEmbedder(m.cfg.LLM.BaseURL, m.cfg.RAG.EmbeddingModel, m.cfg.RAG.EmbeddingDim)
	case "openai":
		embedder = rag.NewOpenAIEmbedder("https://api.openai.com/v1", m.cfg.LLM.APIKey, m.cfg.RAG.EmbeddingModel, m.cfg.RAG.EmbeddingDim)
	default:
		embedder = rag.NewOllamaEmbedder(m.cfg.LLM.BaseURL, m.cfg.RAG.EmbeddingModel, m.cfg.RAG.EmbeddingDim)
	}

	// Create vector store based on type
	switch m.cfg.RAG.VectorStore.Type {
	case "memory":
		m.ragStore = rag.NewMemoryStore(embedder)
	case "qdrant":
		store, err := rag.NewQdrantStore(&m.cfg.RAG.VectorStore, embedder)
		if err != nil {
			return fmt.Errorf("failed to create Qdrant store: %w", err)
		}
		m.ragStore = store
	default:
		log.Printf("Unknown vector store type '%s', falling back to memory", m.cfg.RAG.VectorStore.Type)
		m.ragStore = rag.NewMemoryStore(embedder)
	}

	// Create knowledge base
	m.knowledgeBase = rag.NewKnowledgeBase(m.ragStore, embedder, m.cfg.RAG.KnowledgeBase)

	// Index knowledge base on startup if enabled
	if m.cfg.RAG.IndexOnStartup && m.cfg.RAG.KnowledgeBase != "" {
		log.Printf("Indexing knowledge base from %s...", m.cfg.RAG.KnowledgeBase)
		if err := m.knowledgeBase.LoadFromDirectory(ctx, m.cfg.RAG.KnowledgeBase); err != nil {
			log.Printf("Warning: failed to load knowledge base: %v", err)
		}
	}

	return nil
}

// initCollectors initializes data collectors
func (m *Manager) initCollectors() error {
	var err error

	if m.cfg.Collectors.Prometheus.Enabled {
		log.Printf("Initializing Prometheus collector (URL: %s)...", m.cfg.Collectors.Prometheus.URL)
		m.prometheusCollector, err = collectors.NewPrometheusCollector(&m.cfg.Collectors.Prometheus)
		if err != nil {
			log.Printf("Warning: failed to initialize Prometheus collector: %v", err)
		}
	}

	if m.cfg.Collectors.Jaeger.Enabled {
		log.Printf("Initializing Jaeger collector (URL: %s)...", m.cfg.Collectors.Jaeger.QueryURL)
		m.jaegerCollector, err = collectors.NewJaegerCollector(&m.cfg.Collectors.Jaeger)
		if err != nil {
			log.Printf("Warning: failed to initialize Jaeger collector: %v", err)
		}
	}

	if m.cfg.Collectors.Loki.Enabled {
		log.Printf("Initializing Loki collector (URL: %s)...", m.cfg.Collectors.Loki.URL)
		m.lokiCollector, err = collectors.NewLokiCollector(&m.cfg.Collectors.Loki)
		if err != nil {
			log.Printf("Warning: failed to initialize Loki collector: %v", err)
		}
	}

	return nil
}

// initAgents initializes analysis agents
func (m *Manager) initAgents() error {
	if m.cfg.Agents.Trace.Enabled && m.jaegerCollector != nil {
		log.Println("Initializing trace agent...")
		m.traceAgent = agents.NewTraceAgent(m.llmClient, m.ragStore, m.jaegerCollector)
	}

	if m.cfg.Agents.Metric.Enabled && m.prometheusCollector != nil {
		log.Println("Initializing metric agent...")
		m.metricAgent = agents.NewMetricAgent(m.llmClient, m.ragStore, m.prometheusCollector)
	}

	if m.cfg.Agents.Log.Enabled && m.lokiCollector != nil {
		log.Println("Initializing log agent...")
		m.logAgent = agents.NewLogAgent(m.llmClient, m.ragStore, m.lokiCollector)
	}

	return nil
}

// initTemporal initializes Temporal workflow client
func (m *Manager) initTemporal() error {
	log.Printf("Initializing Temporal client (host: %s, namespace: %s)...", m.cfg.Temporal.HostPort, m.cfg.Temporal.Namespace)
	client, err := workflow.NewWorkflowClient(&m.cfg.Temporal)
	if err != nil {
		return err
	}
	m.workflowClient = client
	return nil
}

// initAlerting initializes alerting
func (m *Manager) initAlerting() error {
	log.Println("Initializing alerting...")
	m.alertManager = alerting.NewAlertManager(&m.cfg.Alerting)
	return nil
}

// Shutdown gracefully shuts down all components
func (m *Manager) Shutdown(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	log.Println("Shutting down framework...")

	if m.workflowClient != nil {
		m.workflowClient.Close()
	}

	m.initialized = false
	log.Println("Framework shutdown complete")
	return nil
}

// Analyze performs analysis using the configured components
func (m *Manager) Analyze(ctx context.Context, req *types.AnalysisRequest) (*types.AnalysisResult, error) {
	m.mu.RLock()
	if !m.initialized {
		m.mu.RUnlock()
		return nil, fmt.Errorf("framework not initialized")
	}
	m.mu.RUnlock()

	// Use Temporal workflow if enabled, otherwise run synchronously
	if m.cfg.Features.EnableTemporal && m.workflowClient != nil {
		return m.analyzeWithTemporal(ctx, req)
	}
	return m.analyzeSynchronous(ctx, req)
}

// analyzeWithTemporal runs analysis through Temporal workflows
func (m *Manager) analyzeWithTemporal(ctx context.Context, req *types.AnalysisRequest) (*types.AnalysisResult, error) {
	input := workflow.ObservabilityAnalysisInput{
		Query:        req.Query,
		Services:     req.Services,
		TimeRange:    req.TimeRange,
		AnalysisType: "all",
		IncludeRAG:   m.cfg.Features.EnableRAG,
	}

	workflowID, err := m.workflowClient.StartAnalysis(ctx, input)
	if err != nil {
		return nil, err
	}

	return m.workflowClient.GetResult(ctx, workflowID)
}

// analyzeSynchronous runs analysis without Temporal
// This is a simplified fallback that runs agents directly
func (m *Manager) analyzeSynchronous(ctx context.Context, req *types.AnalysisRequest) (*types.AnalysisResult, error) {
	result := &types.AnalysisResult{
		RequestID: req.ID,
		Type:      types.AnalysisTypeAll,
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	errChan := make(chan error, 3)

	// Run enabled agents concurrently using their Analyze methods
	if m.cfg.Agents.Trace.Enabled && m.traceAgent != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			agentResult, err := m.traceAgent.Analyze(ctx, req)
			if err != nil {
				errChan <- fmt.Errorf("trace analysis error: %w", err)
				return
			}
			if agentResult != nil {
				mu.Lock()
				result.Findings = append(result.Findings, agentResult.Findings...)
				mu.Unlock()
			}
		}()
	}

	if m.cfg.Agents.Metric.Enabled && m.metricAgent != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			agentResult, err := m.metricAgent.Analyze(ctx, req)
			if err != nil {
				errChan <- fmt.Errorf("metric analysis error: %w", err)
				return
			}
			if agentResult != nil {
				mu.Lock()
				result.Findings = append(result.Findings, agentResult.Findings...)
				if len(agentResult.Predictions) > 0 {
					result.Predictions = append(result.Predictions, agentResult.Predictions...)
				}
				mu.Unlock()
			}
		}()
	}

	if m.cfg.Agents.Log.Enabled && m.logAgent != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			agentResult, err := m.logAgent.Analyze(ctx, req)
			if err != nil {
				errChan <- fmt.Errorf("log analysis error: %w", err)
				return
			}
			if agentResult != nil {
				mu.Lock()
				result.Findings = append(result.Findings, agentResult.Findings...)
				mu.Unlock()
			}
		}()
	}

	wg.Wait()
	close(errChan)

	// Collect any errors
	var errs []error
	for err := range errChan {
		errs = append(errs, err)
	}

	if len(errs) > 0 && len(result.Findings) == 0 {
		return nil, fmt.Errorf("all analyses failed: %v", errs)
	}

	return result, nil
}

// Query performs a RAG-augmented query
func (m *Manager) Query(ctx context.Context, query string) (string, error) {
	client := m.getLLMClient()

	if !m.cfg.Features.EnableRAG || m.ragStore == nil {
		// Direct LLM query without RAG
		resp, err := client.Complete(ctx, &types.LLMRequest{
			UserPrompt: query,
		})
		if err != nil {
			return "", err
		}
		return resp.Content, nil
	}

	// Query vector store for context
	ragResult, err := m.ragStore.Query(ctx, &types.RAGQuery{
		Query: query,
		TopK:  m.cfg.RAG.TopK,
	})
	if err != nil {
		// Fall back to direct query
		resp, err := client.Complete(ctx, &types.LLMRequest{
			UserPrompt: query,
		})
		if err != nil {
			return "", err
		}
		return resp.Content, nil
	}

	// Build augmented prompt
	var contextBuilder string
	for _, doc := range ragResult.Documents {
		contextBuilder += doc.Content + "\n\n"
	}

	augmentedPrompt := fmt.Sprintf("Context:\n%s\n\nQuestion: %s", contextBuilder, query)
	resp, err := client.Complete(ctx, &types.LLMRequest{
		UserPrompt: augmentedPrompt,
	})
	if err != nil {
		return "", err
	}

	return resp.Content, nil
}

// getLLMClient returns the appropriate LLM client (cached or uncached)
func (m *Manager) getLLMClient() llm.Client {
	if m.cfg.Features.EnableCaching && m.cachedClient != nil {
		return m.cachedClient
	}
	return m.llmClient
}

// SendAlert sends an alert if alerting is enabled
func (m *Manager) SendAlert(ctx context.Context, finding *types.Finding) error {
	if !m.cfg.Features.EnableAlerting || m.alertManager == nil {
		return nil
	}

	// Convert finding to alert
	alert := &alerting.Alert{
		Title:       finding.Title,
		Description: finding.Description,
		Severity:    finding.Severity,
		Findings:    []types.Finding{*finding},
	}

	return m.alertManager.Notify(ctx, alert)
}

// GetConfig returns the current configuration
func (m *Manager) GetConfig() *config.Config {
	return m.cfg
}

// GetLLMClient returns the LLM client
func (m *Manager) GetLLMClient() llm.Client {
	return m.getLLMClient()
}

// GetVectorStore returns the vector store if RAG is enabled
func (m *Manager) GetVectorStore() rag.Store {
	return m.ragStore
}

// GetKnowledgeBase returns the knowledge base if RAG is enabled
func (m *Manager) GetKnowledgeBase() *rag.KnowledgeBase {
	return m.knowledgeBase
}

// GetWorkflowClient returns the Temporal workflow client if enabled
func (m *Manager) GetWorkflowClient() *workflow.WorkflowClient {
	return m.workflowClient
}

// GetAlertManager returns the alert manager if alerting is enabled
func (m *Manager) GetAlertManager() *alerting.AlertManager {
	return m.alertManager
}

// GetTraceAgent returns the trace agent if enabled
func (m *Manager) GetTraceAgent() *agents.TraceAgent {
	return m.traceAgent
}

// GetMetricAgent returns the metric agent if enabled
func (m *Manager) GetMetricAgent() *agents.MetricAgent {
	return m.metricAgent
}

// GetLogAgent returns the log agent if enabled
func (m *Manager) GetLogAgent() *agents.LogAgent {
	return m.logAgent
}

// GetPrometheusCollector returns the Prometheus collector if enabled
func (m *Manager) GetPrometheusCollector() *collectors.PrometheusCollector {
	return m.prometheusCollector
}

// GetJaegerCollector returns the Jaeger collector if enabled
func (m *Manager) GetJaegerCollector() *collectors.JaegerCollector {
	return m.jaegerCollector
}

// GetLokiCollector returns the Loki collector if enabled
func (m *Manager) GetLokiCollector() *collectors.LokiCollector {
	return m.lokiCollector
}

// Health returns the health status of all components
func (m *Manager) Health(ctx context.Context) map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	health := map[string]interface{}{
		"initialized": m.initialized,
		"features":    m.cfg.GetEnabledFeatures(),
		"agents":      m.cfg.GetEnabledAgents(),
		"collectors":  m.cfg.GetEnabledCollectors(),
	}

	components := make(map[string]string)

	// Check LLM
	components["llm"] = "healthy"
	if m.llmClient == nil {
		components["llm"] = "not initialized"
	}

	// Check RAG
	if m.cfg.Features.EnableRAG {
		components["rag"] = "healthy"
		if m.ragStore == nil {
			components["rag"] = "not initialized"
		}
	} else {
		components["rag"] = "disabled"
	}

	// Check Temporal
	if m.cfg.Features.EnableTemporal {
		components["temporal"] = "healthy"
		if m.workflowClient == nil {
			components["temporal"] = "not initialized"
		}
	} else {
		components["temporal"] = "disabled"
	}

	// Check Caching
	if m.cfg.Features.EnableCaching {
		components["caching"] = "healthy"
		if m.cache == nil {
			components["caching"] = "not initialized"
		} else {
			stats := m.cache.Stats()
			components["cache_size"] = fmt.Sprintf("%d/%d", stats.Size, stats.MaxSize)
			components["cache_hit_rate"] = fmt.Sprintf("%.2f%%", stats.HitRate*100)
		}
	} else {
		components["caching"] = "disabled"
	}

	// Check Alerting
	if m.cfg.Features.EnableAlerting {
		components["alerting"] = "healthy"
		if m.alertManager == nil {
			components["alerting"] = "not initialized"
		}
	} else {
		components["alerting"] = "disabled"
	}

	health["components"] = components
	return health
}
