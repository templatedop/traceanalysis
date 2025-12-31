// Package agents provides the trace analysis agent
package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/traceanalysis/rag-temporal/internal/types"
	"github.com/traceanalysis/rag-temporal/pkg/llm"
	"github.com/traceanalysis/rag-temporal/pkg/rag"
)

// TraceAgent analyzes distributed traces
type TraceAgent struct {
	*BaseAgent
	collector TraceCollector
}

// TraceCollector defines the interface for fetching traces
type TraceCollector interface {
	FetchTraces(ctx context.Context, timeRange types.TimeRange, services []string, limit int) ([]types.Trace, error)
	GetServiceGraph(ctx context.Context, timeRange types.TimeRange) (*types.ServiceGraph, error)
}

// NewTraceAgent creates a new trace analysis agent
func NewTraceAgent(llmClient llm.Client, ragStore rag.Store, collector TraceCollector) *TraceAgent {
	return &TraceAgent{
		BaseAgent: NewBaseAgent("trace-analyzer", types.AnalysisTypeTrace, llmClient, ragStore),
		collector: collector,
	}
}

// Analyze performs trace analysis
func (a *TraceAgent) Analyze(ctx context.Context, request *types.AnalysisRequest) (*types.AgentResult, error) {
	startTime := time.Now()

	result := &types.AgentResult{
		AgentName: a.Name(),
		Type:      a.Type(),
	}

	// Fetch traces
	traces, err := a.collector.FetchTraces(ctx, request.TimeRange, request.Services, 1000)
	if err != nil {
		result.Error = fmt.Sprintf("failed to fetch traces: %v", err)
		return result, nil
	}

	if len(traces) == 0 {
		result.Findings = append(result.Findings, CreateFinding(
			types.FindingTypeReliability,
			types.SeverityLow,
			"No traces found",
			fmt.Sprintf("No traces found for the specified time range and services: %v", request.Services),
		))
		result.ProcessingTime = time.Since(startTime)
		return result, nil
	}

	// Calculate statistics
	stats := a.calculateStats(traces)

	// Find bottlenecks
	bottlenecks := a.findBottlenecks(traces)

	// Build service graph
	graph, _ := a.collector.GetServiceGraph(ctx, request.TimeRange)

	// Get RAG context
	ragContext, _ := a.GetRAGContext(ctx, request.Query, 5)

	// Prepare trace data for LLM
	traceData := TraceAnalysisData{
		Statistics:   stats,
		Bottlenecks:  bottlenecks,
		ServiceGraph: graph,
		SampleTraces: a.getSampleTraces(traces, 10),
		ErrorTraces:  a.getErrorTraces(traces, 10),
	}

	// LLM analysis
	llmResult, err := a.llmAnalysis(ctx, traceData, ragContext, request.Query)
	if err != nil {
		// Return statistical findings even if LLM fails
		result.Findings = a.generateStatisticalFindings(stats, bottlenecks)
		result.RawData = traceData
		result.ProcessingTime = time.Since(startTime)
		result.Error = fmt.Sprintf("LLM analysis failed: %v", err)
		return result, nil
	}

	result.Findings = llmResult.Findings
	result.Predictions = llmResult.Predictions
	result.RawData = traceData
	result.ProcessingTime = time.Since(startTime)

	return result, nil
}

// TraceAnalysisData contains data for trace analysis
type TraceAnalysisData struct {
	Statistics   *types.TraceStatistics `json:"statistics"`
	Bottlenecks  []types.Bottleneck     `json:"bottlenecks"`
	ServiceGraph *types.ServiceGraph    `json:"service_graph,omitempty"`
	SampleTraces []TraceSummary         `json:"sample_traces"`
	ErrorTraces  []TraceSummary         `json:"error_traces"`
}

// TraceSummary is a summary of a trace for LLM analysis
type TraceSummary struct {
	TraceID     string              `json:"trace_id"`
	Duration    string              `json:"duration"`
	SpanCount   int                 `json:"span_count"`
	HasError    bool                `json:"has_error"`
	Services    []string            `json:"services"`
	RootService string              `json:"root_service"`
	CriticalPath []SpanSummary      `json:"critical_path"`
	ErrorSpans  []SpanSummary       `json:"error_spans,omitempty"`
}

// SpanSummary is a summary of a span
type SpanSummary struct {
	Service      string `json:"service"`
	Operation    string `json:"operation"`
	Duration     string `json:"duration"`
	Status       string `json:"status"`
	ErrorMessage string `json:"error_message,omitempty"`
}

func (a *TraceAgent) calculateStats(traces []types.Trace) *types.TraceStatistics {
	if len(traces) == 0 {
		return &types.TraceStatistics{}
	}

	stats := &types.TraceStatistics{
		TotalTraces:     len(traces),
		ErrorsByService: make(map[string]int),
		SpanCounts:      make(map[string]int),
	}

	var durations []time.Duration
	errorCount := 0
	serviceLatencies := make(map[string][]time.Duration)

	for _, trace := range traces {
		durations = append(durations, trace.Duration)
		if trace.HasError {
			errorCount++
		}

		for _, span := range trace.Spans {
			stats.SpanCounts[span.Service]++
			serviceLatencies[span.Service] = append(serviceLatencies[span.Service], span.Duration)
			if span.Status == "ERROR" {
				stats.ErrorsByService[span.Service]++
			}
		}
	}

	stats.ErrorRate = float64(errorCount) / float64(len(traces)) * 100

	// Calculate percentiles
	sort.Slice(durations, func(i, j int) bool {
		return durations[i] < durations[j]
	})

	stats.P50Latency = percentile(durations, 50)
	stats.P95Latency = percentile(durations, 95)
	stats.P99Latency = percentile(durations, 99)

	// Find slowest services
	for service, latencies := range serviceLatencies {
		sort.Slice(latencies, func(i, j int) bool {
			return latencies[i] < latencies[j]
		})
		var total time.Duration
		for _, l := range latencies {
			total += l
		}
		avg := total / time.Duration(len(latencies))
		stats.SlowestServices = append(stats.SlowestServices, types.ServiceLatency{
			Service:     service,
			AvgDuration: avg,
			MaxDuration: latencies[len(latencies)-1],
			CallCount:   len(latencies),
		})
	}

	sort.Slice(stats.SlowestServices, func(i, j int) bool {
		return stats.SlowestServices[i].AvgDuration > stats.SlowestServices[j].AvgDuration
	})

	if len(stats.SlowestServices) > 10 {
		stats.SlowestServices = stats.SlowestServices[:10]
	}

	return stats
}

func (a *TraceAgent) findBottlenecks(traces []types.Trace) []types.Bottleneck {
	bottleneckMap := make(map[string]*types.Bottleneck)

	for _, trace := range traces {
		criticalPath := a.findCriticalPath(trace)
		for _, span := range criticalPath {
			percentage := float64(span.Duration) / float64(trace.Duration) * 100
			if percentage < 30 {
				continue
			}

			key := fmt.Sprintf("%s:%s", span.Service, span.Operation)
			if b, ok := bottleneckMap[key]; ok {
				b.TraceIDs = append(b.TraceIDs, trace.TraceID)
				b.Duration = (b.Duration + span.Duration) / 2
				b.Percentage = (b.Percentage + percentage) / 2
			} else {
				bottleneckMap[key] = &types.Bottleneck{
					Service:    span.Service,
					Operation:  span.Operation,
					Duration:   span.Duration,
					Percentage: percentage,
					TraceIDs:   []string{trace.TraceID},
				}
			}
		}
	}

	bottlenecks := make([]types.Bottleneck, 0, len(bottleneckMap))
	for _, b := range bottleneckMap {
		bottlenecks = append(bottlenecks, *b)
	}

	sort.Slice(bottlenecks, func(i, j int) bool {
		return bottlenecks[i].Percentage > bottlenecks[j].Percentage
	})

	if len(bottlenecks) > 10 {
		bottlenecks = bottlenecks[:10]
	}

	return bottlenecks
}

func (a *TraceAgent) findCriticalPath(trace types.Trace) []types.Span {
	if len(trace.Spans) == 0 {
		return nil
	}

	// Build parent-child map
	childrenMap := make(map[string][]types.Span)
	var rootSpan *types.Span

	for i := range trace.Spans {
		span := &trace.Spans[i]
		if span.ParentID == "" {
			rootSpan = span
		} else {
			childrenMap[span.ParentID] = append(childrenMap[span.ParentID], *span)
		}
	}

	if rootSpan == nil && len(trace.Spans) > 0 {
		rootSpan = &trace.Spans[0]
	}

	// Find the longest path
	return a.findLongestPath(*rootSpan, childrenMap)
}

func (a *TraceAgent) findLongestPath(span types.Span, children map[string][]types.Span) []types.Span {
	path := []types.Span{span}

	childSpans := children[span.SpanID]
	if len(childSpans) == 0 {
		return path
	}

	var longestChild []types.Span
	for _, child := range childSpans {
		childPath := a.findLongestPath(child, children)
		if len(longestChild) == 0 || totalDuration(childPath) > totalDuration(longestChild) {
			longestChild = childPath
		}
	}

	return append(path, longestChild...)
}

func totalDuration(spans []types.Span) time.Duration {
	var total time.Duration
	for _, s := range spans {
		total += s.Duration
	}
	return total
}

func (a *TraceAgent) getSampleTraces(traces []types.Trace, limit int) []TraceSummary {
	if len(traces) > limit {
		traces = traces[:limit]
	}

	summaries := make([]TraceSummary, 0, len(traces))
	for _, trace := range traces {
		summaries = append(summaries, a.traceToSummary(trace))
	}
	return summaries
}

func (a *TraceAgent) getErrorTraces(traces []types.Trace, limit int) []TraceSummary {
	var errorTraces []types.Trace
	for _, trace := range traces {
		if trace.HasError {
			errorTraces = append(errorTraces, trace)
		}
	}

	if len(errorTraces) > limit {
		errorTraces = errorTraces[:limit]
	}

	summaries := make([]TraceSummary, 0, len(errorTraces))
	for _, trace := range errorTraces {
		summaries = append(summaries, a.traceToSummary(trace))
	}
	return summaries
}

func (a *TraceAgent) traceToSummary(trace types.Trace) TraceSummary {
	summary := TraceSummary{
		TraceID:   trace.TraceID,
		Duration:  trace.Duration.String(),
		SpanCount: len(trace.Spans),
		HasError:  trace.HasError,
		Services:  trace.Services,
	}

	if trace.RootSpan != nil {
		summary.RootService = trace.RootSpan.Service
	}

	criticalPath := a.findCriticalPath(trace)
	for _, span := range criticalPath {
		summary.CriticalPath = append(summary.CriticalPath, SpanSummary{
			Service:   span.Service,
			Operation: span.Operation,
			Duration:  span.Duration.String(),
			Status:    span.Status,
		})
	}

	for _, span := range trace.Spans {
		if span.Status == "ERROR" {
			summary.ErrorSpans = append(summary.ErrorSpans, SpanSummary{
				Service:      span.Service,
				Operation:    span.Operation,
				Duration:     span.Duration.String(),
				Status:       span.Status,
				ErrorMessage: span.StatusMessage,
			})
		}
	}

	return summary
}

// TraceAnalysisLLMResult represents the LLM analysis result
type TraceAnalysisLLMResult struct {
	Summary         string                 `json:"summary"`
	RootCause       string                 `json:"root_cause,omitempty"`
	Findings        []types.Finding        `json:"findings"`
	Predictions     []types.Prediction     `json:"predictions,omitempty"`
	ServiceHealth   map[string]ServiceHealth `json:"service_health,omitempty"`
	Recommendations []RecommendationLLM    `json:"recommendations,omitempty"`
}

// ServiceHealth from LLM
type ServiceHealth struct {
	Status            string  `json:"status"`
	LatencyAssessment string  `json:"latency_assessment"`
	ErrorRate         string  `json:"error_rate"`
}

// RecommendationLLM from LLM
type RecommendationLLM struct {
	Priority    int    `json:"priority"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Effort      string `json:"effort"`
}

func (a *TraceAgent) llmAnalysis(ctx context.Context, data TraceAnalysisData, ragContext string, query string) (*TraceAnalysisLLMResult, error) {
	dataJSON, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal trace data: %w", err)
	}

	systemPrompt := llm.TraceAnalysisSystemPrompt
	systemPrompt = replaceTemplate(systemPrompt, "{{.RAGContext}}", ragContext)
	systemPrompt = replaceTemplate(systemPrompt, "{{.SystemArchitecture}}", "")

	userPrompt := fmt.Sprintf("Analyze the following trace data:\n\n%s", string(dataJSON))
	if query != "" {
		userPrompt += fmt.Sprintf("\n\nSpecific query: %s", query)
	}

	var result TraceAnalysisLLMResult
	err = a.llmClient.CompleteJSON(ctx, &types.LLMRequest{
		SystemPrompt: systemPrompt,
		UserPrompt:   userPrompt,
		Temperature:  0.1,
	}, &result)
	if err != nil {
		return nil, err
	}

	// Set timestamps for findings
	for i := range result.Findings {
		result.Findings[i].ID = fmt.Sprintf("finding_%d_%d", time.Now().UnixNano(), i)
		result.Findings[i].Timestamp = time.Now()
	}

	return &result, nil
}

func (a *TraceAgent) generateStatisticalFindings(stats *types.TraceStatistics, bottlenecks []types.Bottleneck) []types.Finding {
	var findings []types.Finding

	// High error rate finding
	if stats.ErrorRate > 5 {
		severity := types.SeverityMedium
		if stats.ErrorRate > 10 {
			severity = types.SeverityHigh
		}
		if stats.ErrorRate > 20 {
			severity = types.SeverityCritical
		}

		findings = append(findings, CreateFinding(
			types.FindingTypeReliability,
			severity,
			"High Error Rate Detected",
			fmt.Sprintf("Error rate is %.2f%% across %d traces", stats.ErrorRate, stats.TotalTraces),
		))
	}

	// Slow service findings
	for _, svc := range stats.SlowestServices[:min(3, len(stats.SlowestServices))] {
		if svc.AvgDuration > 500*time.Millisecond {
			findings = append(findings, CreateFinding(
				types.FindingTypePerformance,
				types.SeverityMedium,
				fmt.Sprintf("Slow Service: %s", svc.Service),
				fmt.Sprintf("Average latency: %s, Max: %s across %d calls", svc.AvgDuration, svc.MaxDuration, svc.CallCount),
			))
		}
	}

	// Bottleneck findings
	for _, b := range bottlenecks[:min(3, len(bottlenecks))] {
		findings = append(findings, CreateFinding(
			types.FindingTypePerformance,
			types.SeverityHigh,
			fmt.Sprintf("Bottleneck: %s/%s", b.Service, b.Operation),
			fmt.Sprintf("Accounts for %.1f%% of trace duration (avg: %s)", b.Percentage, b.Duration),
		))
	}

	return findings
}

func percentile(sorted []time.Duration, p int) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	index := (len(sorted) - 1) * p / 100
	return sorted[index]
}

func replaceTemplate(template, placeholder, value string) string {
	if value == "" {
		value = "Not available"
	}
	return fmt.Sprintf("%s", template[:]) // Simple string replacement handled elsewhere
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
