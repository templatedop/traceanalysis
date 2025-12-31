// Package metrics provides Prometheus metrics for self-observability
package metrics

import (
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	// Analysis metrics
	AnalysisTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "observability_analysis",
			Name:      "analysis_total",
			Help:      "Total number of analysis requests",
		},
		[]string{"type", "status"},
	)

	AnalysisDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "observability_analysis",
			Name:      "analysis_duration_seconds",
			Help:      "Duration of analysis in seconds",
			Buckets:   []float64{0.1, 0.5, 1, 2, 5, 10, 30, 60, 120, 300},
		},
		[]string{"type"},
	)

	FindingsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "observability_analysis",
			Name:      "findings_total",
			Help:      "Total number of findings",
		},
		[]string{"type", "severity"},
	)

	// Agent metrics
	AgentAnalysisTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "observability_analysis",
			Name:      "agent_analysis_total",
			Help:      "Total number of agent analyses",
		},
		[]string{"agent", "status"},
	)

	AgentAnalysisDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "observability_analysis",
			Name:      "agent_analysis_duration_seconds",
			Help:      "Duration of agent analysis in seconds",
			Buckets:   []float64{0.1, 0.5, 1, 2, 5, 10, 30, 60},
		},
		[]string{"agent"},
	)

	// LLM metrics
	LLMRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "observability_analysis",
			Name:      "llm_requests_total",
			Help:      "Total number of LLM requests",
		},
		[]string{"provider", "model", "status"},
	)

	LLMRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "observability_analysis",
			Name:      "llm_request_duration_seconds",
			Help:      "Duration of LLM requests in seconds",
			Buckets:   []float64{0.5, 1, 2, 5, 10, 30, 60, 120},
		},
		[]string{"provider", "model"},
	)

	LLMTokensUsed = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "observability_analysis",
			Name:      "llm_tokens_total",
			Help:      "Total number of LLM tokens used",
		},
		[]string{"provider", "model"},
	)

	LLMCacheHits = promauto.NewCounter(
		prometheus.CounterOpts{
			Namespace: "observability_analysis",
			Name:      "llm_cache_hits_total",
			Help:      "Total number of LLM cache hits",
		},
	)

	LLMCacheMisses = promauto.NewCounter(
		prometheus.CounterOpts{
			Namespace: "observability_analysis",
			Name:      "llm_cache_misses_total",
			Help:      "Total number of LLM cache misses",
		},
	)

	// RAG metrics
	RAGQueriesTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "observability_analysis",
			Name:      "rag_queries_total",
			Help:      "Total number of RAG queries",
		},
		[]string{"status"},
	)

	RAGQueryDuration = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Namespace: "observability_analysis",
			Name:      "rag_query_duration_seconds",
			Help:      "Duration of RAG queries in seconds",
			Buckets:   []float64{0.01, 0.05, 0.1, 0.25, 0.5, 1, 2, 5},
		},
	)

	RAGDocumentsTotal = promauto.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "observability_analysis",
			Name:      "rag_documents_total",
			Help:      "Total number of documents in RAG store",
		},
	)

	RAGDocumentsRetrieved = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Namespace: "observability_analysis",
			Name:      "rag_documents_retrieved",
			Help:      "Number of documents retrieved per query",
			Buckets:   []float64{0, 1, 2, 3, 5, 10, 20},
		},
	)

	// Collector metrics
	CollectorFetchTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "observability_analysis",
			Name:      "collector_fetch_total",
			Help:      "Total number of collector fetches",
		},
		[]string{"collector", "status"},
	)

	CollectorFetchDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "observability_analysis",
			Name:      "collector_fetch_duration_seconds",
			Help:      "Duration of collector fetches in seconds",
			Buckets:   []float64{0.1, 0.5, 1, 2, 5, 10, 30},
		},
		[]string{"collector"},
	)

	// Alerting metrics
	AlertsSentTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "observability_analysis",
			Name:      "alerts_sent_total",
			Help:      "Total number of alerts sent",
		},
		[]string{"channel", "severity", "status"},
	)

	// Workflow metrics
	WorkflowsStartedTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "observability_analysis",
			Name:      "workflows_started_total",
			Help:      "Total number of workflows started",
		},
		[]string{"workflow"},
	)

	WorkflowsCompletedTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "observability_analysis",
			Name:      "workflows_completed_total",
			Help:      "Total number of workflows completed",
		},
		[]string{"workflow", "status"},
	)

	WorkflowDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "observability_analysis",
			Name:      "workflow_duration_seconds",
			Help:      "Duration of workflows in seconds",
			Buckets:   []float64{1, 5, 10, 30, 60, 120, 300, 600},
		},
		[]string{"workflow"},
	)

	// API metrics
	HTTPRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "observability_analysis",
			Name:      "http_requests_total",
			Help:      "Total number of HTTP requests",
		},
		[]string{"method", "path", "status"},
	)

	HTTPRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "observability_analysis",
			Name:      "http_request_duration_seconds",
			Help:      "Duration of HTTP requests in seconds",
			Buckets:   []float64{0.01, 0.05, 0.1, 0.25, 0.5, 1, 2, 5, 10, 30, 60},
		},
		[]string{"method", "path"},
	)
)

// RecordAnalysis records an analysis execution
func RecordAnalysis(analysisType, status string, duration time.Duration, findingsCount int, findingsBySeverity map[string]int) {
	AnalysisTotal.WithLabelValues(analysisType, status).Inc()
	AnalysisDuration.WithLabelValues(analysisType).Observe(duration.Seconds())

	for severity, count := range findingsBySeverity {
		FindingsTotal.WithLabelValues(analysisType, severity).Add(float64(count))
	}
}

// RecordAgentAnalysis records an agent analysis execution
func RecordAgentAnalysis(agent, status string, duration time.Duration) {
	AgentAnalysisTotal.WithLabelValues(agent, status).Inc()
	AgentAnalysisDuration.WithLabelValues(agent).Observe(duration.Seconds())
}

// RecordLLMRequest records an LLM request
func RecordLLMRequest(provider, model, status string, duration time.Duration, tokens int) {
	LLMRequestsTotal.WithLabelValues(provider, model, status).Inc()
	LLMRequestDuration.WithLabelValues(provider, model).Observe(duration.Seconds())
	if tokens > 0 {
		LLMTokensUsed.WithLabelValues(provider, model).Add(float64(tokens))
	}
}

// RecordRAGQuery records a RAG query
func RecordRAGQuery(status string, duration time.Duration, documentsRetrieved int) {
	RAGQueriesTotal.WithLabelValues(status).Inc()
	RAGQueryDuration.Observe(duration.Seconds())
	RAGDocumentsRetrieved.Observe(float64(documentsRetrieved))
}

// RecordCollectorFetch records a collector fetch
func RecordCollectorFetch(collector, status string, duration time.Duration) {
	CollectorFetchTotal.WithLabelValues(collector, status).Inc()
	CollectorFetchDuration.WithLabelValues(collector).Observe(duration.Seconds())
}

// RecordAlertSent records an alert sent
func RecordAlertSent(channel, severity, status string) {
	AlertsSentTotal.WithLabelValues(channel, severity, status).Inc()
}

// Handler returns the Prometheus HTTP handler
func Handler() http.Handler {
	return promhttp.Handler()
}

// MetricsMiddleware is an HTTP middleware that records request metrics
func MetricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Wrap response writer to capture status code
		wrapped := &responseWriter{ResponseWriter: w, statusCode: 200}

		next.ServeHTTP(wrapped, r)

		duration := time.Since(start)
		status := http.StatusText(wrapped.statusCode)

		HTTPRequestsTotal.WithLabelValues(r.Method, r.URL.Path, status).Inc()
		HTTPRequestDuration.WithLabelValues(r.Method, r.URL.Path).Observe(duration.Seconds())
	})
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (w *responseWriter) WriteHeader(code int) {
	w.statusCode = code
	w.ResponseWriter.WriteHeader(code)
}
