// Package workflow provides Temporal workflow definitions for observability analysis
package workflow

import (
	"context"
	"fmt"
	"time"

	"github.com/traceanalysis/rag-temporal/internal/types"
	"github.com/traceanalysis/rag-temporal/pkg/agents"
	"github.com/traceanalysis/rag-temporal/pkg/llm"
	"github.com/traceanalysis/rag-temporal/pkg/rag"
)

// Activities contains all workflow activities
type Activities struct {
	traceAgent  *agents.TraceAgent
	metricAgent *agents.MetricAgent
	logAgent    *agents.LogAgent
	llmClient   llm.Client
	ragStore    rag.Store
}

// NewActivities creates a new Activities instance
func NewActivities(
	traceAgent *agents.TraceAgent,
	metricAgent *agents.MetricAgent,
	logAgent *agents.LogAgent,
	llmClient llm.Client,
	ragStore rag.Store,
) *Activities {
	return &Activities{
		traceAgent:  traceAgent,
		metricAgent: metricAgent,
		logAgent:    logAgent,
		llmClient:   llmClient,
		ragStore:    ragStore,
	}
}

// FetchTracesActivityInput is the input for FetchTracesActivity
type FetchTracesActivityInput struct {
	TimeRange types.TimeRange
	Services  []string
	Query     string
}

// FetchTracesActivity fetches and analyzes traces
func (a *Activities) FetchTracesActivity(ctx context.Context, input FetchTracesActivityInput) (*types.AgentResult, error) {
	request := &types.AnalysisRequest{
		ID:        fmt.Sprintf("trace_%d", time.Now().UnixNano()),
		Type:      types.AnalysisTypeTrace,
		TimeRange: input.TimeRange,
		Services:  input.Services,
		Query:     input.Query,
	}

	return a.traceAgent.Analyze(ctx, request)
}

// FetchMetricsActivityInput is the input for FetchMetricsActivity
type FetchMetricsActivityInput struct {
	TimeRange types.TimeRange
	Services  []string
	Query     string
}

// FetchMetricsActivity fetches and analyzes metrics
func (a *Activities) FetchMetricsActivity(ctx context.Context, input FetchMetricsActivityInput) (*types.AgentResult, error) {
	request := &types.AnalysisRequest{
		ID:        fmt.Sprintf("metric_%d", time.Now().UnixNano()),
		Type:      types.AnalysisTypeMetric,
		TimeRange: input.TimeRange,
		Services:  input.Services,
		Query:     input.Query,
	}

	return a.metricAgent.Analyze(ctx, request)
}

// FetchLogsActivityInput is the input for FetchLogsActivity
type FetchLogsActivityInput struct {
	TimeRange types.TimeRange
	Services  []string
	Query     string
}

// FetchLogsActivity fetches and analyzes logs
func (a *Activities) FetchLogsActivity(ctx context.Context, input FetchLogsActivityInput) (*types.AgentResult, error) {
	request := &types.AnalysisRequest{
		ID:        fmt.Sprintf("log_%d", time.Now().UnixNano()),
		Type:      types.AnalysisTypeLog,
		TimeRange: input.TimeRange,
		Services:  input.Services,
		Query:     input.Query,
	}

	return a.logAgent.Analyze(ctx, request)
}

// RAGQueryInput is the input for RAGQueryActivity
type RAGQueryInput struct {
	Query string
	TopK  int
	Types []string
}

// FetchRAGContextActivity retrieves relevant context from RAG
func (a *Activities) FetchRAGContextActivity(ctx context.Context, input RAGQueryInput) (*types.RAGResult, error) {
	return a.ragStore.Query(ctx, &types.RAGQuery{
		Query: input.Query,
		TopK:  input.TopK,
		Types: input.Types,
	})
}

// LLMAnalysisInput is the input for LLMAnalysisActivity
type LLMAnalysisInput struct {
	TraceResult  *types.AgentResult
	MetricResult *types.AgentResult
	LogResult    *types.AgentResult
	RAGContext   *types.RAGResult
	Query        string
}

// LLMAnalysisResult is the output of LLMAnalysisActivity
type LLMAnalysisResult struct {
	Summary         string                 `json:"summary"`
	Findings        []types.Finding        `json:"findings"`
	Predictions     []types.Prediction     `json:"predictions,omitempty"`
	Recommendations []types.Recommendation `json:"recommendations,omitempty"`
	IncidentSeverity string               `json:"incident_severity,omitempty"`
	RootCause       *RootCause             `json:"root_cause,omitempty"`
}

// RootCause represents the root cause analysis
type RootCause struct {
	Component           string   `json:"component"`
	FailureMode         string   `json:"failure_mode"`
	Trigger             string   `json:"trigger"`
	ContributingFactors []string `json:"contributing_factors"`
}

// LLMAnalysisActivity performs comprehensive LLM analysis
func (a *Activities) LLMAnalysisActivity(ctx context.Context, input LLMAnalysisInput) (*LLMAnalysisResult, error) {
	// Build context from all sources
	var contextParts []string

	if input.TraceResult != nil && input.TraceResult.Error == "" {
		contextParts = append(contextParts, fmt.Sprintf("TRACE ANALYSIS:\n%d findings", len(input.TraceResult.Findings)))
		for _, f := range input.TraceResult.Findings {
			contextParts = append(contextParts, fmt.Sprintf("- [%s] %s: %s", f.Severity, f.Title, f.Description))
		}
	}

	if input.MetricResult != nil && input.MetricResult.Error == "" {
		contextParts = append(contextParts, fmt.Sprintf("\nMETRIC ANALYSIS:\n%d findings, %d predictions",
			len(input.MetricResult.Findings), len(input.MetricResult.Predictions)))
		for _, f := range input.MetricResult.Findings {
			contextParts = append(contextParts, fmt.Sprintf("- [%s] %s: %s", f.Severity, f.Title, f.Description))
		}
	}

	if input.LogResult != nil && input.LogResult.Error == "" {
		contextParts = append(contextParts, fmt.Sprintf("\nLOG ANALYSIS:\n%d findings", len(input.LogResult.Findings)))
		for _, f := range input.LogResult.Findings {
			contextParts = append(contextParts, fmt.Sprintf("- [%s] %s: %s", f.Severity, f.Title, f.Description))
		}
	}

	// Add RAG context
	if input.RAGContext != nil && len(input.RAGContext.Documents) > 0 {
		contextParts = append(contextParts, "\nRELEVANT KNOWLEDGE BASE DOCUMENTS:")
		for _, doc := range input.RAGContext.Documents {
			contextParts = append(contextParts, fmt.Sprintf("- %s (%s): %s",
				doc.Title, doc.Type, truncate(doc.Content, 500)))
		}
	}

	systemPrompt := llm.IncidentAnalysisSystemPrompt
	userPrompt := fmt.Sprintf(`Based on the following observability data, provide a comprehensive analysis:

%s

User Query: %s

Provide your analysis in JSON format with: summary, findings, predictions, recommendations, incident_severity, and root_cause.`,
		join(contextParts, "\n"), input.Query)

	var result LLMAnalysisResult
	err := a.llmClient.CompleteJSON(ctx, &types.LLMRequest{
		SystemPrompt: systemPrompt,
		UserPrompt:   userPrompt,
		Temperature:  0.1,
		MaxTokens:    4096,
	}, &result)
	if err != nil {
		return nil, fmt.Errorf("LLM analysis failed: %w", err)
	}

	// Set timestamps for findings
	for i := range result.Findings {
		result.Findings[i].ID = fmt.Sprintf("finding_%d_%d", time.Now().UnixNano(), i)
		result.Findings[i].Timestamp = time.Now()
	}

	return &result, nil
}

// NotifyFindingsInput is the input for NotifyFindingsActivity
type NotifyFindingsInput struct {
	Result   *types.AnalysisResult
	Channels []string
}

// NotifyFindingsActivity sends notifications about findings
func (a *Activities) NotifyFindingsActivity(ctx context.Context, input NotifyFindingsInput) error {
	// This would integrate with Slack, PagerDuty, etc.
	// For now, just log the findings
	for _, finding := range input.Result.Findings {
		if finding.Severity == types.SeverityCritical || finding.Severity == types.SeverityHigh {
			// Log critical findings
			fmt.Printf("[%s] %s: %s\n", finding.Severity, finding.Title, finding.Description)
		}
	}
	return nil
}

// CreateAlertInput is the input for CreateAlertActivity
type CreateAlertInput struct {
	Finding types.Finding
	Channel string
}

// CreateAlertActivity creates an alert from a finding
func (a *Activities) CreateAlertActivity(ctx context.Context, input CreateAlertInput) error {
	// This would create alerts in external systems
	fmt.Printf("ALERT [%s]: %s - %s\n", input.Finding.Severity, input.Finding.Title, input.Finding.Description)
	return nil
}

// Helper functions

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func join(parts []string, sep string) string {
	result := ""
	for i, p := range parts {
		if i > 0 {
			result += sep
		}
		result += p
	}
	return result
}
