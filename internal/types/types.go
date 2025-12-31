// Package types defines core types used throughout the observability analysis framework
package types

import (
	"context"
	"time"
)

// AnalysisType represents the type of observability analysis
type AnalysisType string

const (
	AnalysisTypeTrace    AnalysisType = "TRACE"
	AnalysisTypeMetric   AnalysisType = "METRIC"
	AnalysisTypeLog      AnalysisType = "LOG"
	AnalysisTypeIncident AnalysisType = "INCIDENT"
	AnalysisTypeAll      AnalysisType = "ALL"
)

// Severity levels for findings
type Severity string

const (
	SeverityCritical Severity = "CRITICAL"
	SeverityHigh     Severity = "HIGH"
	SeverityMedium   Severity = "MEDIUM"
	SeverityLow      Severity = "LOW"
)

// FindingType categorizes the type of finding
type FindingType string

const (
	FindingTypePerformance FindingType = "PERFORMANCE"
	FindingTypeSecurity    FindingType = "SECURITY"
	FindingTypeReliability FindingType = "RELIABILITY"
	FindingTypeCapacity    FindingType = "CAPACITY"
)

// TimeRange represents a time window for analysis
type TimeRange struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

// Duration returns the duration of the time range
func (tr TimeRange) Duration() time.Duration {
	return tr.End.Sub(tr.Start)
}

// AnalysisRequest represents a request for observability analysis
type AnalysisRequest struct {
	ID           string            `json:"id"`
	Type         AnalysisType      `json:"type"`
	TimeRange    TimeRange         `json:"time_range"`
	Services     []string          `json:"services,omitempty"`
	Query        string            `json:"query"`
	Filters      map[string]string `json:"filters,omitempty"`
	IncludeRAG   bool              `json:"include_rag"`
	MaxResults   int               `json:"max_results,omitempty"`
	Priority     int               `json:"priority,omitempty"`
	CallbackURL  string            `json:"callback_url,omitempty"`
}

// AnalysisResult contains the complete analysis output
type AnalysisResult struct {
	RequestID        string           `json:"request_id"`
	Type             AnalysisType     `json:"type"`
	Summary          string           `json:"summary"`
	Findings         []Finding        `json:"findings"`
	Predictions      []Prediction     `json:"predictions,omitempty"`
	Recommendations  []Recommendation `json:"recommendations"`
	RelatedIncidents []PastIncident   `json:"related_incidents,omitempty"`
	ServiceHealth    map[string]ServiceHealthStatus `json:"service_health,omitempty"`
	Metadata         AnalysisMetadata `json:"metadata"`
	CreatedAt        time.Time        `json:"created_at"`
}

// AnalysisMetadata contains metadata about the analysis
type AnalysisMetadata struct {
	ProcessingTime time.Duration          `json:"processing_time"`
	DataSources    []string               `json:"data_sources"`
	ModelsUsed     []string               `json:"models_used"`
	RAGDocuments   int                    `json:"rag_documents"`
	ConfidenceScore float64               `json:"confidence_score"`
}

// Finding represents a discovered issue or observation
type Finding struct {
	ID              string      `json:"id"`
	Type            FindingType `json:"type"`
	Severity        Severity    `json:"severity"`
	Title           string      `json:"title"`
	Description     string      `json:"description"`
	Evidence        []Evidence  `json:"evidence"`
	AffectedServices []string   `json:"affected_services"`
	Impact          string      `json:"impact"`
	RootCause       string      `json:"root_cause,omitempty"`
	Timestamp       time.Time   `json:"timestamp"`
}

// Evidence represents supporting data for a finding
type Evidence struct {
	Type        string            `json:"type"` // trace, metric, log, event
	Source      string            `json:"source"`
	Description string            `json:"description"`
	Data        interface{}       `json:"data,omitempty"`
	Links       []string          `json:"links,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// Prediction represents a predictive analysis result
type Prediction struct {
	ID               string        `json:"id"`
	Metric           string        `json:"metric"`
	CurrentValue     float64       `json:"current_value"`
	PredictedValue   float64       `json:"predicted_value"`
	Threshold        float64       `json:"threshold,omitempty"`
	TimeToThreshold  time.Duration `json:"time_to_threshold,omitempty"`
	Confidence       float64       `json:"confidence"`
	Description      string        `json:"description"`
	RecommendedAction string       `json:"recommended_action,omitempty"`
	Timestamp        time.Time     `json:"timestamp"`
}

// Recommendation represents an actionable suggestion
type Recommendation struct {
	ID                  string   `json:"id"`
	Priority            int      `json:"priority"`
	Title               string   `json:"title"`
	Description         string   `json:"description"`
	Impact              string   `json:"impact"`
	Effort              string   `json:"effort"` // LOW, MEDIUM, HIGH
	RunbookLink         string   `json:"runbook_link,omitempty"`
	AutomationAvailable bool     `json:"automation_available"`
	Actions             []Action `json:"actions,omitempty"`
}

// Action represents a specific action to take
type Action struct {
	Type        string            `json:"type"`
	Description string            `json:"description"`
	Command     string            `json:"command,omitempty"`
	Parameters  map[string]string `json:"parameters,omitempty"`
	Automated   bool              `json:"automated"`
}

// PastIncident represents a historical incident for RAG context
type PastIncident struct {
	ID               string    `json:"id"`
	Title            string    `json:"title"`
	Date             time.Time `json:"date"`
	Severity         Severity  `json:"severity"`
	Symptoms         []string  `json:"symptoms"`
	RootCause        string    `json:"root_cause"`
	Resolution       string    `json:"resolution"`
	PreventionMeasures []string `json:"prevention_measures"`
	AffectedServices []string  `json:"affected_services"`
	Duration         time.Duration `json:"duration"`
	SimilarityScore  float64   `json:"similarity_score,omitempty"`
}

// ServiceHealthStatus represents the health status of a service
type ServiceHealthStatus struct {
	Service           string  `json:"service"`
	Status            string  `json:"status"` // HEALTHY, DEGRADED, UNHEALTHY
	LatencyP50        float64 `json:"latency_p50_ms"`
	LatencyP95        float64 `json:"latency_p95_ms"`
	LatencyP99        float64 `json:"latency_p99_ms"`
	ErrorRate         float64 `json:"error_rate"`
	RequestRate       float64 `json:"request_rate"`
	SLOCompliance     float64 `json:"slo_compliance"`
	LastChecked       time.Time `json:"last_checked"`
}

// Trace represents a distributed trace
type Trace struct {
	TraceID   string            `json:"trace_id"`
	RootSpan  *Span             `json:"root_span"`
	Spans     []Span            `json:"spans"`
	Duration  time.Duration     `json:"duration"`
	HasError  bool              `json:"has_error"`
	Services  []string          `json:"services"`
	Metadata  map[string]string `json:"metadata,omitempty"`
	StartTime time.Time         `json:"start_time"`
}

// Span represents a single span in a trace
type Span struct {
	SpanID        string            `json:"span_id"`
	ParentID      string            `json:"parent_id,omitempty"`
	TraceID       string            `json:"trace_id"`
	Service       string            `json:"service"`
	Operation     string            `json:"operation"`
	Duration      time.Duration     `json:"duration"`
	Status        string            `json:"status"` // OK, ERROR, UNSET
	StatusMessage string            `json:"status_message,omitempty"`
	Tags          map[string]string `json:"tags,omitempty"`
	Logs          []SpanLog         `json:"logs,omitempty"`
	StartTime     time.Time         `json:"start_time"`
	EndTime       time.Time         `json:"end_time"`
}

// SpanLog represents a log entry within a span
type SpanLog struct {
	Timestamp time.Time         `json:"timestamp"`
	Fields    map[string]string `json:"fields"`
}

// TraceStatistics contains aggregated trace statistics
type TraceStatistics struct {
	TotalTraces     int                    `json:"total_traces"`
	ErrorRate       float64                `json:"error_rate"`
	P50Latency      time.Duration          `json:"p50_latency"`
	P95Latency      time.Duration          `json:"p95_latency"`
	P99Latency      time.Duration          `json:"p99_latency"`
	SlowestServices []ServiceLatency       `json:"slowest_services"`
	ErrorsByService map[string]int         `json:"errors_by_service"`
	SpanCounts      map[string]int         `json:"span_counts"`
}

// ServiceLatency represents latency information for a service
type ServiceLatency struct {
	Service      string        `json:"service"`
	Operation    string        `json:"operation"`
	AvgDuration  time.Duration `json:"avg_duration"`
	MaxDuration  time.Duration `json:"max_duration"`
	CallCount    int           `json:"call_count"`
}

// ServiceGraph represents the dependency graph of services
type ServiceGraph struct {
	Nodes []ServiceNode `json:"nodes"`
	Edges []ServiceEdge `json:"edges"`
}

// ServiceNode represents a service in the dependency graph
type ServiceNode struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Type        string  `json:"type"` // service, database, cache, queue, external
	RequestRate float64 `json:"request_rate"`
	ErrorRate   float64 `json:"error_rate"`
	AvgLatency  float64 `json:"avg_latency_ms"`
}

// ServiceEdge represents a connection between services
type ServiceEdge struct {
	Source      string  `json:"source"`
	Target      string  `json:"target"`
	RequestRate float64 `json:"request_rate"`
	ErrorRate   float64 `json:"error_rate"`
	AvgLatency  float64 `json:"avg_latency_ms"`
}

// MetricData contains time series metrics data
type MetricData struct {
	Series       []TimeSeries  `json:"series"`
	Anomalies    []Anomaly     `json:"anomalies,omitempty"`
	Predictions  []Prediction  `json:"predictions,omitempty"`
	Correlations []Correlation `json:"correlations,omitempty"`
}

// TimeSeries represents a time series of metric values
type TimeSeries struct {
	Name   string            `json:"name"`
	Labels map[string]string `json:"labels,omitempty"`
	Values []DataPoint       `json:"values"`
}

// DataPoint represents a single data point in a time series
type DataPoint struct {
	Timestamp time.Time `json:"timestamp"`
	Value     float64   `json:"value"`
}

// Anomaly represents a detected anomaly in metrics
type Anomaly struct {
	ID          string    `json:"id"`
	Metric      string    `json:"metric"`
	Labels      map[string]string `json:"labels,omitempty"`
	Timestamp   time.Time `json:"timestamp"`
	Value       float64   `json:"value"`
	Expected    float64   `json:"expected"`
	Deviation   float64   `json:"deviation"`
	Severity    Severity  `json:"severity"`
	Description string    `json:"description,omitempty"`
}

// Correlation represents a correlation between two metrics
type Correlation struct {
	MetricA     string  `json:"metric_a"`
	MetricB     string  `json:"metric_b"`
	Coefficient float64 `json:"coefficient"`
	Lag         time.Duration `json:"lag"`
	Significance float64 `json:"significance"`
}

// LogData contains log analysis data
type LogData struct {
	Templates    []LogTemplate    `json:"templates"`
	ErrorGroups  []ErrorGroup     `json:"error_groups"`
	Anomalies    []LogAnomaly     `json:"anomalies,omitempty"`
	Statistics   LogStatistics    `json:"statistics"`
}

// LogTemplate represents a parsed log template
type LogTemplate struct {
	ID          string   `json:"id"`
	Template    string   `json:"template"`
	Parameters  []string `json:"parameters"`
	Count       int      `json:"count"`
	FirstSeen   time.Time `json:"first_seen"`
	LastSeen    time.Time `json:"last_seen"`
	Severity    string   `json:"severity"` // DEBUG, INFO, WARN, ERROR, FATAL
	Services    []string `json:"services"`
}

// ErrorGroup represents a group of similar errors
type ErrorGroup struct {
	ID           string    `json:"id"`
	Pattern      string    `json:"pattern"`
	Message      string    `json:"message"`
	Count        int       `json:"count"`
	FirstSeen    time.Time `json:"first_seen"`
	LastSeen     time.Time `json:"last_seen"`
	Services     []string  `json:"services"`
	StackTrace   string    `json:"stack_trace,omitempty"`
	SampleLogs   []string  `json:"sample_logs"`
}

// LogAnomaly represents an anomaly detected in logs
type LogAnomaly struct {
	ID          string    `json:"id"`
	Type        string    `json:"type"` // frequency_spike, new_pattern, missing_pattern
	Description string    `json:"description"`
	Timestamp   time.Time `json:"timestamp"`
	Severity    Severity  `json:"severity"`
	Pattern     string    `json:"pattern,omitempty"`
	ExpectedCount int     `json:"expected_count,omitempty"`
	ActualCount   int     `json:"actual_count,omitempty"`
}

// LogStatistics contains aggregated log statistics
type LogStatistics struct {
	TotalLogs       int            `json:"total_logs"`
	LogsByLevel     map[string]int `json:"logs_by_level"`
	LogsByService   map[string]int `json:"logs_by_service"`
	ErrorRate       float64        `json:"error_rate"`
	UniquePatterns  int            `json:"unique_patterns"`
	AnomalyCount    int            `json:"anomaly_count"`
}

// Bottleneck represents a performance bottleneck
type Bottleneck struct {
	Service     string        `json:"service"`
	Operation   string        `json:"operation"`
	Duration    time.Duration `json:"duration"`
	Percentage  float64       `json:"percentage"`
	TraceIDs    []string      `json:"trace_ids,omitempty"`
	Description string        `json:"description,omitempty"`
}

// Agent represents an analysis agent
type Agent interface {
	Name() string
	Type() AnalysisType
	Analyze(ctx context.Context, request *AnalysisRequest) (*AgentResult, error)
	Health(ctx context.Context) error
}

// AgentResult represents the result from a single agent
type AgentResult struct {
	AgentName    string        `json:"agent_name"`
	Type         AnalysisType  `json:"type"`
	Findings     []Finding     `json:"findings"`
	Predictions  []Prediction  `json:"predictions,omitempty"`
	RawData      interface{}   `json:"raw_data,omitempty"`
	ProcessingTime time.Duration `json:"processing_time"`
	Error        string        `json:"error,omitempty"`
}

// RAGDocument represents a document in the RAG knowledge base
type RAGDocument struct {
	ID          string            `json:"id"`
	Type        string            `json:"type"` // runbook, incident, architecture, pattern
	Title       string            `json:"title"`
	Content     string            `json:"content"`
	Embedding   []float64         `json:"embedding,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	Tags        []string          `json:"tags,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// RAGQuery represents a query to the RAG system
type RAGQuery struct {
	Query       string   `json:"query"`
	TopK        int      `json:"top_k"`
	Types       []string `json:"types,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	MinScore    float64  `json:"min_score,omitempty"`
}

// RAGResult represents a result from RAG retrieval
type RAGResult struct {
	Documents   []RAGDocument `json:"documents"`
	Query       string        `json:"query"`
	TotalFound  int           `json:"total_found"`
}

// LLMRequest represents a request to the LLM
type LLMRequest struct {
	SystemPrompt string                 `json:"system_prompt"`
	UserPrompt   string                 `json:"user_prompt"`
	Context      map[string]interface{} `json:"context,omitempty"`
	Model        string                 `json:"model,omitempty"`
	Temperature  float64                `json:"temperature,omitempty"`
	MaxTokens    int                    `json:"max_tokens,omitempty"`
}

// LLMResponse represents a response from the LLM
type LLMResponse struct {
	Content      string                 `json:"content"`
	Model        string                 `json:"model"`
	TokensUsed   int                    `json:"tokens_used"`
	Parsed       map[string]interface{} `json:"parsed,omitempty"`
	FinishReason string                 `json:"finish_reason"`
}
