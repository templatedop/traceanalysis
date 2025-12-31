// Package workflow provides the Temporal worker configuration
package workflow

import (
	"context"
	"fmt"
	"time"

	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"

	"github.com/traceanalysis/rag-temporal/internal/config"
	"github.com/traceanalysis/rag-temporal/internal/types"
	"github.com/traceanalysis/rag-temporal/pkg/agents"
	"github.com/traceanalysis/rag-temporal/pkg/llm"
	"github.com/traceanalysis/rag-temporal/pkg/rag"
)

// Worker manages the Temporal worker
type Worker struct {
	client     client.Client
	worker     worker.Worker
	activities *Activities
	config     *config.TemporalConfig
}

// NewWorker creates a new Temporal worker
func NewWorker(
	cfg *config.TemporalConfig,
	traceAgent *agents.TraceAgent,
	metricAgent *agents.MetricAgent,
	logAgent *agents.LogAgent,
	llmClient llm.Client,
	ragStore rag.Store,
) (*Worker, error) {
	// Create Temporal client
	c, err := client.Dial(client.Options{
		HostPort:  cfg.HostPort,
		Namespace: cfg.Namespace,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create Temporal client: %w", err)
	}

	// Create activities
	activities := NewActivities(traceAgent, metricAgent, logAgent, llmClient, ragStore)

	// Create worker
	w := worker.New(c, cfg.TaskQueue, worker.Options{
		MaxConcurrentActivityExecutionSize: cfg.MaxConcurrent,
		MaxConcurrentWorkflowTaskExecutionSize: cfg.WorkerCount,
	})

	// Register workflows
	w.RegisterWorkflow(ObservabilityAnalysisWorkflow)
	w.RegisterWorkflow(ContinuousMonitoringWorkflow)
	w.RegisterWorkflow(IncidentAnalysisWorkflow)

	// Register activities
	w.RegisterActivity(activities.FetchTracesActivity)
	w.RegisterActivity(activities.FetchMetricsActivity)
	w.RegisterActivity(activities.FetchLogsActivity)
	w.RegisterActivity(activities.FetchRAGContextActivity)
	w.RegisterActivity(activities.LLMAnalysisActivity)
	w.RegisterActivity(activities.NotifyFindingsActivity)
	w.RegisterActivity(activities.CreateAlertActivity)

	return &Worker{
		client:     c,
		worker:     w,
		activities: activities,
		config:     cfg,
	}, nil
}

// Start starts the worker
func (w *Worker) Start() error {
	return w.worker.Start()
}

// Stop stops the worker
func (w *Worker) Stop() {
	w.worker.Stop()
	w.client.Close()
}

// Client returns the Temporal client
func (w *Worker) Client() client.Client {
	return w.client
}

// WorkflowClient provides methods to start workflows
type WorkflowClient struct {
	client    client.Client
	taskQueue string
}

// NewWorkflowClient creates a new workflow client
func NewWorkflowClient(cfg *config.TemporalConfig) (*WorkflowClient, error) {
	c, err := client.Dial(client.Options{
		HostPort:  cfg.HostPort,
		Namespace: cfg.Namespace,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create Temporal client: %w", err)
	}

	return &WorkflowClient{
		client:    c,
		taskQueue: cfg.TaskQueue,
	}, nil
}

// Close closes the client
func (c *WorkflowClient) Close() {
	c.client.Close()
}

// StartAnalysis starts an observability analysis workflow
func (c *WorkflowClient) StartAnalysis(ctx context.Context, input ObservabilityAnalysisInput) (string, error) {
	if input.ID == "" {
		input.ID = fmt.Sprintf("analysis_%d", time.Now().UnixNano())
	}

	options := client.StartWorkflowOptions{
		ID:        input.ID,
		TaskQueue: c.taskQueue,
	}

	we, err := c.client.ExecuteWorkflow(ctx, options, ObservabilityAnalysisWorkflow, input)
	if err != nil {
		return "", fmt.Errorf("failed to start workflow: %w", err)
	}

	return we.GetID(), nil
}

// StartContinuousMonitoring starts continuous monitoring workflow
func (c *WorkflowClient) StartContinuousMonitoring(ctx context.Context, input ContinuousMonitoringInput) (string, error) {
	if input.ID == "" {
		input.ID = fmt.Sprintf("monitoring_%d", time.Now().UnixNano())
	}

	options := client.StartWorkflowOptions{
		ID:        input.ID,
		TaskQueue: c.taskQueue,
	}

	we, err := c.client.ExecuteWorkflow(ctx, options, ContinuousMonitoringWorkflow, input)
	if err != nil {
		return "", fmt.Errorf("failed to start workflow: %w", err)
	}

	return we.GetID(), nil
}

// StartIncidentAnalysis starts an incident analysis workflow
func (c *WorkflowClient) StartIncidentAnalysis(ctx context.Context, input IncidentAnalysisInput) (string, error) {
	if input.ID == "" {
		input.ID = fmt.Sprintf("incident_%d", time.Now().UnixNano())
	}

	options := client.StartWorkflowOptions{
		ID:        input.ID,
		TaskQueue: c.taskQueue,
	}

	we, err := c.client.ExecuteWorkflow(ctx, options, IncidentAnalysisWorkflow, input)
	if err != nil {
		return "", fmt.Errorf("failed to start workflow: %w", err)
	}

	return we.GetID(), nil
}

// GetResult waits for and returns the analysis result
func (c *WorkflowClient) GetResult(ctx context.Context, workflowID string) (*types.AnalysisResult, error) {
	run := c.client.GetWorkflow(ctx, workflowID, "")

	var result types.AnalysisResult
	if err := run.Get(ctx, &result); err != nil {
		return nil, fmt.Errorf("failed to get workflow result: %w", err)
	}

	return &result, nil
}

// GetStatus returns the status of a workflow
func (c *WorkflowClient) GetStatus(ctx context.Context, workflowID string) (string, error) {
	desc, err := c.client.DescribeWorkflowExecution(ctx, workflowID, "")
	if err != nil {
		return "", fmt.Errorf("failed to describe workflow: %w", err)
	}

	return desc.WorkflowExecutionInfo.Status.String(), nil
}

// CancelWorkflow cancels a running workflow
func (c *WorkflowClient) CancelWorkflow(ctx context.Context, workflowID string) error {
	return c.client.CancelWorkflow(ctx, workflowID, "")
}
