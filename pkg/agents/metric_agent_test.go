package agents

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/traceanalysis/rag-temporal/internal/types"
)

// MockMetricCollector is a mock metric collector for testing
type MockMetricCollector struct {
	series []types.TimeSeries
}

func (m *MockMetricCollector) Query(ctx context.Context, query string, timeRange types.TimeRange) ([]types.TimeSeries, error) {
	return m.series, nil
}

func (m *MockMetricCollector) QueryRange(ctx context.Context, query string, timeRange types.TimeRange, step time.Duration) ([]types.TimeSeries, error) {
	return m.series, nil
}

func (m *MockMetricCollector) GetMetricNames(ctx context.Context) ([]string, error) {
	return []string{"http_requests_total", "cpu_usage", "memory_usage"}, nil
}

func TestMetricAgent_DetectAnomalies(t *testing.T) {
	llmClient := &MockLLMClient{}
	collector := &MockMetricCollector{}
	agent := NewMetricAgent(llmClient, nil, collector)

	// Create time series with normal values and one anomaly
	now := time.Now()
	values := make([]types.DataPoint, 100)
	for i := 0; i < 100; i++ {
		value := 50.0 + float64(i%5) // Normal values between 50-54
		if i == 50 {
			value = 200.0 // Anomaly
		}
		values[i] = types.DataPoint{
			Timestamp: now.Add(time.Duration(i) * time.Minute),
			Value:     value,
		}
	}

	series := []types.TimeSeries{
		{
			Name:   "test_metric",
			Values: values,
		},
	}

	anomalies := agent.detectAnomalies(series)

	// Should detect the anomaly at index 50
	found := false
	for _, a := range anomalies {
		if a.Value == 200.0 {
			found = true
			if a.Deviation < 3 {
				t.Errorf("Expected deviation > 3, got %f", a.Deviation)
			}
		}
	}

	if !found {
		t.Error("Expected to detect anomaly at value 200")
	}
}

func TestMetricAgent_FindCorrelations(t *testing.T) {
	llmClient := &MockLLMClient{}
	collector := &MockMetricCollector{}
	agent := NewMetricAgent(llmClient, nil, collector)

	now := time.Now()

	// Create two perfectly correlated series
	values1 := make([]types.DataPoint, 50)
	values2 := make([]types.DataPoint, 50)
	for i := 0; i < 50; i++ {
		values1[i] = types.DataPoint{
			Timestamp: now.Add(time.Duration(i) * time.Minute),
			Value:     float64(i),
		}
		values2[i] = types.DataPoint{
			Timestamp: now.Add(time.Duration(i) * time.Minute),
			Value:     float64(i * 2), // Perfectly correlated
		}
	}

	series := []types.TimeSeries{
		{Name: "metric_a", Values: values1},
		{Name: "metric_b", Values: values2},
	}

	correlations := agent.findCorrelations(series)

	// Should find strong correlation
	found := false
	for _, c := range correlations {
		if c.MetricA == "metric_a" && c.MetricB == "metric_b" {
			found = true
			if c.Coefficient < 0.99 {
				t.Errorf("Expected correlation > 0.99, got %f", c.Coefficient)
			}
		}
	}

	if !found {
		t.Error("Expected to find correlation between metric_a and metric_b")
	}
}

func TestCalculateMeanStdDev(t *testing.T) {
	values := []types.DataPoint{
		{Value: 10},
		{Value: 20},
		{Value: 30},
		{Value: 40},
		{Value: 50},
	}

	mean, stdDev := calculateMeanStdDev(values)

	expectedMean := 30.0
	if math.Abs(mean-expectedMean) > 0.001 {
		t.Errorf("Expected mean %f, got %f", expectedMean, mean)
	}

	// Standard deviation of [10,20,30,40,50] is approximately 14.14
	if stdDev < 14 || stdDev > 15 {
		t.Errorf("Expected stdDev ~14.14, got %f", stdDev)
	}
}

func TestLinearRegression(t *testing.T) {
	// Create a perfect linear series: y = 2x + 10
	values := make([]types.DataPoint, 10)
	for i := 0; i < 10; i++ {
		values[i] = types.DataPoint{
			Value: float64(2*i + 10),
		}
	}

	slope, intercept := linearRegression(values)

	if math.Abs(slope-2.0) > 0.001 {
		t.Errorf("Expected slope 2.0, got %f", slope)
	}
	if math.Abs(intercept-10.0) > 0.001 {
		t.Errorf("Expected intercept 10.0, got %f", intercept)
	}
}

func TestPearsonCorrelation(t *testing.T) {
	// Perfect positive correlation
	a := []types.DataPoint{{Value: 1}, {Value: 2}, {Value: 3}, {Value: 4}, {Value: 5}}
	b := []types.DataPoint{{Value: 2}, {Value: 4}, {Value: 6}, {Value: 8}, {Value: 10}}

	corr := pearsonCorrelation(a, b)
	if math.Abs(corr-1.0) > 0.001 {
		t.Errorf("Expected correlation 1.0, got %f", corr)
	}

	// Perfect negative correlation
	c := []types.DataPoint{{Value: 5}, {Value: 4}, {Value: 3}, {Value: 2}, {Value: 1}}
	corr = pearsonCorrelation(a, c)
	if math.Abs(corr-(-1.0)) > 0.001 {
		t.Errorf("Expected correlation -1.0, got %f", corr)
	}
}

func TestGetMinMax(t *testing.T) {
	values := []types.DataPoint{
		{Value: 50},
		{Value: 10},
		{Value: 90},
		{Value: 30},
		{Value: 70},
	}

	min, max := getMinMax(values)

	if min != 10 {
		t.Errorf("Expected min 10, got %f", min)
	}
	if max != 90 {
		t.Errorf("Expected max 90, got %f", max)
	}
}

func TestSeverityValue(t *testing.T) {
	tests := []struct {
		severity types.Severity
		expected int
	}{
		{types.SeverityCritical, 4},
		{types.SeverityHigh, 3},
		{types.SeverityMedium, 2},
		{types.SeverityLow, 1},
		{types.Severity("unknown"), 0},
	}

	for _, tt := range tests {
		result := severityValue(tt.severity)
		if result != tt.expected {
			t.Errorf("Expected %d for %s, got %d", tt.expected, tt.severity, result)
		}
	}
}

func BenchmarkDetectAnomalies(b *testing.B) {
	llmClient := &MockLLMClient{}
	collector := &MockMetricCollector{}
	agent := NewMetricAgent(llmClient, nil, collector)

	// Create large time series
	now := time.Now()
	values := make([]types.DataPoint, 10000)
	for i := 0; i < 10000; i++ {
		values[i] = types.DataPoint{
			Timestamp: now.Add(time.Duration(i) * time.Minute),
			Value:     50.0 + float64(i%10),
		}
	}

	series := []types.TimeSeries{
		{Name: "test_metric", Values: values},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		agent.detectAnomalies(series)
	}
}

func BenchmarkPearsonCorrelation(b *testing.B) {
	a := make([]types.DataPoint, 1000)
	bb := make([]types.DataPoint, 1000)
	for i := 0; i < 1000; i++ {
		a[i] = types.DataPoint{Value: float64(i)}
		bb[i] = types.DataPoint{Value: float64(i * 2)}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pearsonCorrelation(a, bb)
	}
}
