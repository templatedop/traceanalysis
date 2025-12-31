package agents

import (
	"context"
	"testing"
	"time"

	"github.com/traceanalysis/rag-temporal/internal/types"
)

// MockLLMClient is a mock LLM client for testing
type MockLLMClient struct {
	CompleteFunc     func(ctx context.Context, req *types.LLMRequest) (*types.LLMResponse, error)
	CompleteJSONFunc func(ctx context.Context, req *types.LLMRequest, result interface{}) error
}

func (m *MockLLMClient) Complete(ctx context.Context, req *types.LLMRequest) (*types.LLMResponse, error) {
	if m.CompleteFunc != nil {
		return m.CompleteFunc(ctx, req)
	}
	return &types.LLMResponse{Content: "mock response"}, nil
}

func (m *MockLLMClient) CompleteJSON(ctx context.Context, req *types.LLMRequest, result interface{}) error {
	if m.CompleteJSONFunc != nil {
		return m.CompleteJSONFunc(ctx, req, result)
	}
	return nil
}

func (m *MockLLMClient) Name() string {
	return "mock"
}

func (m *MockLLMClient) Model() string {
	return "mock-model"
}

func (m *MockLLMClient) Health(ctx context.Context) error {
	return nil
}

// MockTraceCollector is a mock trace collector for testing
type MockTraceCollector struct {
	traces []types.Trace
	graph  *types.ServiceGraph
}

func (m *MockTraceCollector) FetchTraces(ctx context.Context, timeRange types.TimeRange, services []string, limit int) ([]types.Trace, error) {
	return m.traces, nil
}

func (m *MockTraceCollector) GetServiceGraph(ctx context.Context, timeRange types.TimeRange) (*types.ServiceGraph, error) {
	return m.graph, nil
}

func TestTraceAgent_CalculateStats(t *testing.T) {
	llmClient := &MockLLMClient{}
	collector := &MockTraceCollector{}
	agent := NewTraceAgent(llmClient, nil, collector)

	traces := []types.Trace{
		{
			TraceID:  "trace-1",
			Duration: 100 * time.Millisecond,
			HasError: false,
			Spans: []types.Span{
				{SpanID: "span-1", Service: "api", Duration: 50 * time.Millisecond, Status: "OK"},
				{SpanID: "span-2", Service: "db", Duration: 50 * time.Millisecond, Status: "OK"},
			},
		},
		{
			TraceID:  "trace-2",
			Duration: 200 * time.Millisecond,
			HasError: true,
			Spans: []types.Span{
				{SpanID: "span-3", Service: "api", Duration: 100 * time.Millisecond, Status: "OK"},
				{SpanID: "span-4", Service: "db", Duration: 100 * time.Millisecond, Status: "ERROR"},
			},
		},
	}

	stats := agent.calculateStats(traces)

	if stats.TotalTraces != 2 {
		t.Errorf("Expected 2 traces, got %d", stats.TotalTraces)
	}

	if stats.ErrorRate != 50.0 {
		t.Errorf("Expected 50%% error rate, got %f", stats.ErrorRate)
	}

	if stats.ErrorsByService["db"] != 1 {
		t.Errorf("Expected 1 error for db service, got %d", stats.ErrorsByService["db"])
	}
}

func TestTraceAgent_FindBottlenecks(t *testing.T) {
	llmClient := &MockLLMClient{}
	collector := &MockTraceCollector{}
	agent := NewTraceAgent(llmClient, nil, collector)

	traces := []types.Trace{
		{
			TraceID:  "trace-1",
			Duration: 100 * time.Millisecond,
			Spans: []types.Span{
				{SpanID: "span-1", Service: "api", Operation: "handle", Duration: 10 * time.Millisecond},
				{SpanID: "span-2", Service: "db", Operation: "query", Duration: 80 * time.Millisecond, ParentID: "span-1"},
			},
		},
	}

	bottlenecks := agent.findBottlenecks(traces)

	// DB should be identified as bottleneck (80% of trace duration)
	found := false
	for _, b := range bottlenecks {
		if b.Service == "db" && b.Operation == "query" {
			found = true
			if b.Percentage < 70 {
				t.Errorf("Expected DB bottleneck percentage > 70%%, got %f", b.Percentage)
			}
		}
	}

	if !found {
		t.Error("Expected DB to be identified as bottleneck")
	}
}

func TestCreateFinding(t *testing.T) {
	finding := CreateFinding(
		types.FindingTypePerformance,
		types.SeverityHigh,
		"High Latency",
		"Service is experiencing high latency",
	)

	if finding.Type != types.FindingTypePerformance {
		t.Errorf("Expected type PERFORMANCE, got %s", finding.Type)
	}
	if finding.Severity != types.SeverityHigh {
		t.Errorf("Expected severity HIGH, got %s", finding.Severity)
	}
	if finding.Title != "High Latency" {
		t.Errorf("Expected title 'High Latency', got '%s'", finding.Title)
	}
	if finding.ID == "" {
		t.Error("Expected finding ID to be set")
	}
	if finding.Timestamp.IsZero() {
		t.Error("Expected timestamp to be set")
	}
}

func TestCreatePrediction(t *testing.T) {
	prediction := CreatePrediction("cpu_usage", 75.0, 95.0, 0.85, "CPU will reach threshold")

	if prediction.Metric != "cpu_usage" {
		t.Errorf("Expected metric 'cpu_usage', got '%s'", prediction.Metric)
	}
	if prediction.CurrentValue != 75.0 {
		t.Errorf("Expected current value 75.0, got %f", prediction.CurrentValue)
	}
	if prediction.PredictedValue != 95.0 {
		t.Errorf("Expected predicted value 95.0, got %f", prediction.PredictedValue)
	}
	if prediction.Confidence != 0.85 {
		t.Errorf("Expected confidence 0.85, got %f", prediction.Confidence)
	}
}

func TestAgentRegistry(t *testing.T) {
	registry := NewAgentRegistry()

	// Create mock agents
	llmClient := &MockLLMClient{}
	traceAgent := NewTraceAgent(llmClient, nil, &MockTraceCollector{})

	// Register agent
	registry.Register(traceAgent)

	// Get by name
	agent, ok := registry.Get("trace-analyzer")
	if !ok {
		t.Error("Expected to find trace-analyzer agent")
	}
	if agent.Name() != "trace-analyzer" {
		t.Errorf("Expected name 'trace-analyzer', got '%s'", agent.Name())
	}

	// Get by type
	agents := registry.GetByType(types.AnalysisTypeTrace)
	if len(agents) != 1 {
		t.Errorf("Expected 1 agent for TRACE type, got %d", len(agents))
	}

	// Get all
	all := registry.All()
	if len(all) != 1 {
		t.Errorf("Expected 1 total agent, got %d", len(all))
	}
}

func TestBaseAgent_Health(t *testing.T) {
	llmClient := &MockLLMClient{}
	agent := NewBaseAgent("test-agent", types.AnalysisTypeTrace, llmClient, nil)

	ctx := context.Background()
	err := agent.Health(ctx)
	if err != nil {
		t.Errorf("Expected healthy agent, got error: %v", err)
	}
}

// Benchmark tests
func BenchmarkTraceAgent_CalculateStats(b *testing.B) {
	llmClient := &MockLLMClient{}
	collector := &MockTraceCollector{}
	agent := NewTraceAgent(llmClient, nil, collector)

	// Create 1000 traces
	traces := make([]types.Trace, 1000)
	for i := 0; i < 1000; i++ {
		traces[i] = types.Trace{
			TraceID:  string(rune(i)),
			Duration: time.Duration(i) * time.Millisecond,
			HasError: i%10 == 0,
			Spans: []types.Span{
				{SpanID: "span-1", Service: "api", Duration: time.Duration(i/2) * time.Millisecond},
				{SpanID: "span-2", Service: "db", Duration: time.Duration(i/2) * time.Millisecond},
			},
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		agent.calculateStats(traces)
	}
}
