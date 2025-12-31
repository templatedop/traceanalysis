// Package workflow provides Temporal workflow definitions for observability analysis
package workflow

import (
	"fmt"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/traceanalysis/rag-temporal/internal/types"
)

// ObservabilityAnalysisInput is the input for the main analysis workflow
type ObservabilityAnalysisInput struct {
	ID          string            `json:"id"`
	Query       string            `json:"query"`
	Services    []string          `json:"services,omitempty"`
	TimeRange   types.TimeRange   `json:"time_range"`
	AnalysisType string           `json:"analysis_type"` // "incident", "performance", "capacity", "security", "all"
	IncludeRAG  bool              `json:"include_rag"`
	Notify      bool              `json:"notify"`
	Priority    int               `json:"priority"`
}

// ObservabilityAnalysisWorkflow is the main workflow for observability analysis
func ObservabilityAnalysisWorkflow(ctx workflow.Context, input ObservabilityAnalysisInput) (*types.AnalysisResult, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting observability analysis workflow", "id", input.ID, "type", input.AnalysisType)

	// Configure activity options
	activityOpts := workflow.ActivityOptions{
		StartToCloseTimeout: 5 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    time.Minute,
			MaximumAttempts:    3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, activityOpts)

	startTime := workflow.Now(ctx)

	// Step 1: Parallel data gathering from all agents
	var traceResult, metricResult, logResult *types.AgentResult

	// Create futures for parallel execution
	traceFuture := workflow.ExecuteActivity(ctx, "FetchTracesActivity", FetchTracesActivityInput{
		TimeRange: input.TimeRange,
		Services:  input.Services,
		Query:     input.Query,
	})

	metricFuture := workflow.ExecuteActivity(ctx, "FetchMetricsActivity", FetchMetricsActivityInput{
		TimeRange: input.TimeRange,
		Services:  input.Services,
		Query:     input.Query,
	})

	logFuture := workflow.ExecuteActivity(ctx, "FetchLogsActivity", FetchLogsActivityInput{
		TimeRange: input.TimeRange,
		Services:  input.Services,
		Query:     input.Query,
	})

	// Wait for all data gathering to complete
	if err := traceFuture.Get(ctx, &traceResult); err != nil {
		logger.Warn("Trace analysis failed", "error", err)
	}
	if err := metricFuture.Get(ctx, &metricResult); err != nil {
		logger.Warn("Metric analysis failed", "error", err)
	}
	if err := logFuture.Get(ctx, &logResult); err != nil {
		logger.Warn("Log analysis failed", "error", err)
	}

	// Step 2: Get RAG context if requested
	var ragContext *types.RAGResult
	if input.IncludeRAG {
		ragFuture := workflow.ExecuteActivity(ctx, "FetchRAGContextActivity", RAGQueryInput{
			Query: input.Query,
			TopK:  5,
		})
		if err := ragFuture.Get(ctx, &ragContext); err != nil {
			logger.Warn("RAG context fetch failed", "error", err)
		}
	}

	// Step 3: Comprehensive LLM analysis
	var llmResult *LLMAnalysisResult
	llmFuture := workflow.ExecuteActivity(ctx, "LLMAnalysisActivity", LLMAnalysisInput{
		TraceResult:  traceResult,
		MetricResult: metricResult,
		LogResult:    logResult,
		RAGContext:   ragContext,
		Query:        input.Query,
	})
	if err := llmFuture.Get(ctx, &llmResult); err != nil {
		logger.Warn("LLM analysis failed", "error", err)
		// Continue with raw findings
	}

	// Step 4: Compile final result
	result := compileResult(input, traceResult, metricResult, logResult, llmResult, ragContext, startTime)

	// Step 5: Notify if requested and has critical findings
	if input.Notify && hasCriticalFindings(result) {
		notifyFuture := workflow.ExecuteActivity(ctx, "NotifyFindingsActivity", NotifyFindingsInput{
			Result:   result,
			Channels: []string{"slack", "pagerduty"},
		})
		if err := notifyFuture.Get(ctx, nil); err != nil {
			logger.Warn("Notification failed", "error", err)
		}
	}

	logger.Info("Observability analysis workflow completed",
		"id", input.ID,
		"findings", len(result.Findings),
		"predictions", len(result.Predictions),
		"duration", time.Since(startTime))

	return result, nil
}

// ContinuousMonitoringInput is the input for continuous monitoring workflow
type ContinuousMonitoringInput struct {
	ID            string          `json:"id"`
	Services      []string        `json:"services"`
	Interval      time.Duration   `json:"interval"`
	LookbackWindow time.Duration  `json:"lookback_window"`
	AlertThreshold types.Severity `json:"alert_threshold"`
	MaxIterations  int            `json:"max_iterations"` // 0 = unlimited
}

// ContinuousMonitoringWorkflow runs continuous observability monitoring
func ContinuousMonitoringWorkflow(ctx workflow.Context, input ContinuousMonitoringInput) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting continuous monitoring workflow", "id", input.ID)

	if input.Interval == 0 {
		input.Interval = 5 * time.Minute
	}
	if input.LookbackWindow == 0 {
		input.LookbackWindow = 15 * time.Minute
	}

	iterations := 0
	for {
		// Check if we should stop
		if input.MaxIterations > 0 && iterations >= input.MaxIterations {
			logger.Info("Reached maximum iterations, stopping")
			break
		}
		iterations++

		// Calculate time range
		now := workflow.Now(ctx)
		timeRange := types.TimeRange{
			Start: now.Add(-input.LookbackWindow),
			End:   now,
		}

		// Run analysis as child workflow
		childCtx := workflow.WithChildOptions(ctx, workflow.ChildWorkflowOptions{
			WorkflowID: fmt.Sprintf("%s-iteration-%d", input.ID, iterations),
		})

		analysisInput := ObservabilityAnalysisInput{
			ID:           fmt.Sprintf("%s-%d", input.ID, iterations),
			Query:        "Detect anomalies and predict potential issues",
			Services:     input.Services,
			TimeRange:    timeRange,
			AnalysisType: "all",
			IncludeRAG:   true,
			Notify:       true,
		}

		var result *types.AnalysisResult
		err := workflow.ExecuteChildWorkflow(childCtx, ObservabilityAnalysisWorkflow, analysisInput).Get(ctx, &result)
		if err != nil {
			logger.Warn("Analysis iteration failed", "iteration", iterations, "error", err)
		} else {
			// Log findings
			for _, finding := range result.Findings {
				if severityLevel(finding.Severity) >= severityLevel(input.AlertThreshold) {
					logger.Info("Finding detected",
						"severity", finding.Severity,
						"title", finding.Title,
						"service", finding.AffectedServices)
				}
			}
		}

		// Wait for next interval
		if err := workflow.Sleep(ctx, input.Interval); err != nil {
			return err
		}
	}

	return nil
}

// IncidentAnalysisInput is the input for incident analysis workflow
type IncidentAnalysisInput struct {
	ID          string          `json:"id"`
	Title       string          `json:"title"`
	Description string          `json:"description"`
	Services    []string        `json:"services"`
	StartTime   time.Time       `json:"start_time"`
	Symptoms    []string        `json:"symptoms,omitempty"`
}

// IncidentAnalysisWorkflow performs deep incident analysis
func IncidentAnalysisWorkflow(ctx workflow.Context, input IncidentAnalysisInput) (*types.AnalysisResult, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting incident analysis workflow", "id", input.ID, "title", input.Title)

	activityOpts := workflow.ActivityOptions{
		StartToCloseTimeout: 10 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, activityOpts)

	// Expand time range for incident analysis
	timeRange := types.TimeRange{
		Start: input.StartTime.Add(-1 * time.Hour),
		End:   workflow.Now(ctx),
	}

	// Build query from symptoms
	query := input.Description
	if len(input.Symptoms) > 0 {
		query += " Symptoms: " + joinStrings(input.Symptoms, ", ")
	}

	// Run comprehensive analysis
	analysisInput := ObservabilityAnalysisInput{
		ID:           input.ID,
		Query:        query,
		Services:     input.Services,
		TimeRange:    timeRange,
		AnalysisType: "incident",
		IncludeRAG:   true,
		Notify:       true,
		Priority:     1,
	}

	var result *types.AnalysisResult
	err := workflow.ExecuteChildWorkflow(ctx, ObservabilityAnalysisWorkflow, analysisInput).Get(ctx, &result)
	if err != nil {
		return nil, err
	}

	// Fetch similar past incidents
	ragFuture := workflow.ExecuteActivity(ctx, "FetchRAGContextActivity", RAGQueryInput{
		Query: input.Title + " " + input.Description,
		TopK:  5,
		Types: []string{"incident"},
	})

	var ragResult *types.RAGResult
	if err := ragFuture.Get(ctx, &ragResult); err != nil {
		logger.Warn("Failed to fetch similar incidents", "error", err)
	} else if ragResult != nil {
		for _, doc := range ragResult.Documents {
			result.RelatedIncidents = append(result.RelatedIncidents, types.PastIncident{
				ID:    doc.ID,
				Title: doc.Title,
			})
		}
	}

	return result, nil
}

// Helper functions

func compileResult(
	input ObservabilityAnalysisInput,
	traceResult, metricResult, logResult *types.AgentResult,
	llmResult *LLMAnalysisResult,
	ragContext *types.RAGResult,
	startTime time.Time,
) *types.AnalysisResult {
	result := &types.AnalysisResult{
		RequestID:       input.ID,
		Type:            types.AnalysisType(input.AnalysisType),
		CreatedAt:       startTime,
		ServiceHealth:   make(map[string]types.ServiceHealthStatus),
	}

	// Compile findings from all sources
	if traceResult != nil {
		result.Findings = append(result.Findings, traceResult.Findings...)
	}
	if metricResult != nil {
		result.Findings = append(result.Findings, metricResult.Findings...)
		result.Predictions = append(result.Predictions, metricResult.Predictions...)
	}
	if logResult != nil {
		result.Findings = append(result.Findings, logResult.Findings...)
	}

	// Add LLM analysis if available
	if llmResult != nil {
		result.Summary = llmResult.Summary
		if len(llmResult.Findings) > 0 {
			result.Findings = llmResult.Findings
		}
		if len(llmResult.Predictions) > 0 {
			result.Predictions = llmResult.Predictions
		}
		result.Recommendations = llmResult.Recommendations
	}

	// Set metadata
	var dataSources []string
	var modelsUsed []string
	if traceResult != nil {
		dataSources = append(dataSources, "traces")
	}
	if metricResult != nil {
		dataSources = append(dataSources, "metrics")
	}
	if logResult != nil {
		dataSources = append(dataSources, "logs")
	}
	if llmResult != nil {
		modelsUsed = append(modelsUsed, "llm")
	}

	ragDocs := 0
	if ragContext != nil {
		ragDocs = len(ragContext.Documents)
	}

	result.Metadata = types.AnalysisMetadata{
		ProcessingTime: time.Since(startTime),
		DataSources:    dataSources,
		ModelsUsed:     modelsUsed,
		RAGDocuments:   ragDocs,
	}

	return result
}

func hasCriticalFindings(result *types.AnalysisResult) bool {
	for _, finding := range result.Findings {
		if finding.Severity == types.SeverityCritical || finding.Severity == types.SeverityHigh {
			return true
		}
	}
	return false
}

func severityLevel(s types.Severity) int {
	switch s {
	case types.SeverityCritical:
		return 4
	case types.SeverityHigh:
		return 3
	case types.SeverityMedium:
		return 2
	case types.SeverityLow:
		return 1
	default:
		return 0
	}
}

func joinStrings(strs []string, sep string) string {
	result := ""
	for i, s := range strs {
		if i > 0 {
			result += sep
		}
		result += s
	}
	return result
}
