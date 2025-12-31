// Package agents provides the metric analysis agent
package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/traceanalysis/rag-temporal/internal/types"
	"github.com/traceanalysis/rag-temporal/pkg/llm"
	"github.com/traceanalysis/rag-temporal/pkg/rag"
)

// MetricAgent analyzes time series metrics
type MetricAgent struct {
	*BaseAgent
	collector MetricCollector
}

// MetricCollector defines the interface for fetching metrics
type MetricCollector interface {
	Query(ctx context.Context, query string, timeRange types.TimeRange) ([]types.TimeSeries, error)
	QueryRange(ctx context.Context, query string, timeRange types.TimeRange, step time.Duration) ([]types.TimeSeries, error)
	GetMetricNames(ctx context.Context) ([]string, error)
}

// NewMetricAgent creates a new metric analysis agent
func NewMetricAgent(llmClient llm.Client, ragStore rag.Store, collector MetricCollector) *MetricAgent {
	return &MetricAgent{
		BaseAgent: NewBaseAgent("metric-analyzer", types.AnalysisTypeMetric, llmClient, ragStore),
		collector: collector,
	}
}

// Analyze performs metric analysis
func (a *MetricAgent) Analyze(ctx context.Context, request *types.AnalysisRequest) (*types.AgentResult, error) {
	startTime := time.Now()

	result := &types.AgentResult{
		AgentName: a.Name(),
		Type:      a.Type(),
	}

	// Fetch key metrics
	metrics, err := a.fetchKeyMetrics(ctx, request.TimeRange, request.Services)
	if err != nil {
		result.Error = fmt.Sprintf("failed to fetch metrics: %v", err)
		return result, nil
	}

	if len(metrics) == 0 {
		result.Findings = append(result.Findings, CreateFinding(
			types.FindingTypeReliability,
			types.SeverityLow,
			"No metrics found",
			"No metrics found for the specified time range",
		))
		result.ProcessingTime = time.Since(startTime)
		return result, nil
	}

	// Detect anomalies
	anomalies := a.detectAnomalies(metrics)

	// Calculate correlations
	correlations := a.findCorrelations(metrics)

	// Generate predictions
	predictions := a.generatePredictions(metrics)

	// Get RAG context
	ragContext, _ := a.GetRAGContext(ctx, request.Query, 5)

	// Prepare data for LLM
	metricData := MetricAnalysisData{
		Series:       a.summarizeSeries(metrics),
		Anomalies:    anomalies,
		Correlations: correlations,
		Predictions:  predictions,
		Statistics:   a.calculateMetricStats(metrics),
	}

	// LLM analysis
	llmResult, err := a.llmAnalysis(ctx, metricData, ragContext, request.Query)
	if err != nil {
		result.Findings = a.generateStatisticalFindings(anomalies, predictions)
		result.Predictions = predictions
		result.RawData = metricData
		result.ProcessingTime = time.Since(startTime)
		result.Error = fmt.Sprintf("LLM analysis failed: %v", err)
		return result, nil
	}

	result.Findings = llmResult.Findings
	result.Predictions = llmResult.Predictions
	result.RawData = metricData
	result.ProcessingTime = time.Since(startTime)

	return result, nil
}

// MetricAnalysisData contains data for metric analysis
type MetricAnalysisData struct {
	Series       []SeriesSummary      `json:"series"`
	Anomalies    []types.Anomaly      `json:"anomalies"`
	Correlations []types.Correlation  `json:"correlations"`
	Predictions  []types.Prediction   `json:"predictions"`
	Statistics   MetricStatistics     `json:"statistics"`
}

// SeriesSummary summarizes a time series for LLM
type SeriesSummary struct {
	Name      string            `json:"name"`
	Labels    map[string]string `json:"labels,omitempty"`
	Min       float64           `json:"min"`
	Max       float64           `json:"max"`
	Avg       float64           `json:"avg"`
	Current   float64           `json:"current"`
	StdDev    float64           `json:"std_dev"`
	Trend     string            `json:"trend"` // increasing, decreasing, stable
	DataPoints int              `json:"data_points"`
}

// MetricStatistics contains aggregate statistics
type MetricStatistics struct {
	TotalSeries   int     `json:"total_series"`
	AnomalyCount  int     `json:"anomaly_count"`
	HighSeverity  int     `json:"high_severity_anomalies"`
	AvgCorrelation float64 `json:"avg_correlation"`
}

func (a *MetricAgent) fetchKeyMetrics(ctx context.Context, timeRange types.TimeRange, services []string) ([]types.TimeSeries, error) {
	var allSeries []types.TimeSeries

	// Key metric queries
	queries := []string{
		"rate(http_requests_total[5m])",
		"histogram_quantile(0.95, rate(http_request_duration_seconds_bucket[5m]))",
		"rate(http_requests_total{status=~\"5..\"}[5m])",
		"process_cpu_seconds_total",
		"process_resident_memory_bytes",
		"go_goroutines",
	}

	step := timeRange.Duration() / 100
	if step < time.Minute {
		step = time.Minute
	}

	for _, query := range queries {
		series, err := a.collector.QueryRange(ctx, query, timeRange, step)
		if err != nil {
			continue
		}
		allSeries = append(allSeries, series...)
	}

	return allSeries, nil
}

func (a *MetricAgent) detectAnomalies(series []types.TimeSeries) []types.Anomaly {
	var anomalies []types.Anomaly

	for _, s := range series {
		if len(s.Values) < 10 {
			continue
		}

		mean, stdDev := calculateMeanStdDev(s.Values)
		if stdDev == 0 {
			continue
		}

		for _, dp := range s.Values {
			zScore := math.Abs(dp.Value-mean) / stdDev

			if zScore > 3 {
				severity := types.SeverityLow
				if zScore > 4 {
					severity = types.SeverityMedium
				}
				if zScore > 5 {
					severity = types.SeverityHigh
				}

				anomalies = append(anomalies, types.Anomaly{
					ID:        fmt.Sprintf("anomaly_%d", len(anomalies)),
					Metric:    s.Name,
					Labels:    s.Labels,
					Timestamp: dp.Timestamp,
					Value:     dp.Value,
					Expected:  mean,
					Deviation: zScore,
					Severity:  severity,
				})
			}
		}
	}

	// Limit to top anomalies
	sort.Slice(anomalies, func(i, j int) bool {
		return anomalies[i].Deviation > anomalies[j].Deviation
	})

	if len(anomalies) > 50 {
		anomalies = anomalies[:50]
	}

	return anomalies
}

func (a *MetricAgent) findCorrelations(series []types.TimeSeries) []types.Correlation {
	var correlations []types.Correlation

	for i := 0; i < len(series); i++ {
		for j := i + 1; j < len(series); j++ {
			if len(series[i].Values) < 10 || len(series[j].Values) < 10 {
				continue
			}

			coef := pearsonCorrelation(series[i].Values, series[j].Values)

			if math.Abs(coef) > 0.7 {
				correlations = append(correlations, types.Correlation{
					MetricA:      series[i].Name,
					MetricB:      series[j].Name,
					Coefficient:  coef,
					Significance: math.Abs(coef),
				})
			}
		}
	}

	sort.Slice(correlations, func(i, j int) bool {
		return math.Abs(correlations[i].Coefficient) > math.Abs(correlations[j].Coefficient)
	})

	if len(correlations) > 20 {
		correlations = correlations[:20]
	}

	return correlations
}

func (a *MetricAgent) generatePredictions(series []types.TimeSeries) []types.Prediction {
	var predictions []types.Prediction

	for _, s := range series {
		if len(s.Values) < 20 {
			continue
		}

		// Simple linear regression for trend
		slope, intercept := linearRegression(s.Values)
		currentValue := s.Values[len(s.Values)-1].Value
		lastTimestamp := s.Values[len(s.Values)-1].Timestamp

		// Predict 24 hours ahead
		futureTime := lastTimestamp.Add(24 * time.Hour)
		timeDiff := futureTime.Sub(lastTimestamp).Hours()
		predictedValue := intercept + slope*timeDiff

		// Check if prediction crosses thresholds
		threshold := a.getThresholdForMetric(s.Name, currentValue)
		if threshold > 0 {
			if currentValue < threshold && predictedValue >= threshold {
				// Calculate time to threshold
				timeToThreshold := (threshold - intercept) / slope
				if timeToThreshold > 0 {
					predictions = append(predictions, types.Prediction{
						ID:              fmt.Sprintf("pred_%d", len(predictions)),
						Metric:          s.Name,
						CurrentValue:    currentValue,
						PredictedValue:  predictedValue,
						Threshold:       threshold,
						TimeToThreshold: time.Duration(timeToThreshold * float64(time.Hour)),
						Confidence:      a.calculatePredictionConfidence(s.Values, slope),
						Description:     fmt.Sprintf("Metric %s predicted to reach threshold %.2f", s.Name, threshold),
						Timestamp:       time.Now(),
					})
				}
			}
		}
	}

	return predictions
}

func (a *MetricAgent) summarizeSeries(series []types.TimeSeries) []SeriesSummary {
	summaries := make([]SeriesSummary, 0, len(series))

	for _, s := range series {
		if len(s.Values) == 0 {
			continue
		}

		mean, stdDev := calculateMeanStdDev(s.Values)
		minVal, maxVal := getMinMax(s.Values)

		slope, _ := linearRegression(s.Values)
		trend := "stable"
		if slope > 0.01 {
			trend = "increasing"
		} else if slope < -0.01 {
			trend = "decreasing"
		}

		summaries = append(summaries, SeriesSummary{
			Name:       s.Name,
			Labels:     s.Labels,
			Min:        minVal,
			Max:        maxVal,
			Avg:        mean,
			Current:    s.Values[len(s.Values)-1].Value,
			StdDev:     stdDev,
			Trend:      trend,
			DataPoints: len(s.Values),
		})
	}

	return summaries
}

func (a *MetricAgent) calculateMetricStats(series []types.TimeSeries) MetricStatistics {
	stats := MetricStatistics{
		TotalSeries: len(series),
	}

	anomalies := a.detectAnomalies(series)
	stats.AnomalyCount = len(anomalies)

	for _, anomaly := range anomalies {
		if anomaly.Severity == types.SeverityHigh || anomaly.Severity == types.SeverityCritical {
			stats.HighSeverity++
		}
	}

	correlations := a.findCorrelations(series)
	if len(correlations) > 0 {
		var total float64
		for _, c := range correlations {
			total += math.Abs(c.Coefficient)
		}
		stats.AvgCorrelation = total / float64(len(correlations))
	}

	return stats
}

func (a *MetricAgent) getThresholdForMetric(name string, currentValue float64) float64 {
	// Default thresholds based on metric patterns
	switch {
	case contains(name, "cpu"):
		return 0.9 // 90% CPU
	case contains(name, "memory"):
		return 0.85 * currentValue * 1.5 // 85% of estimated max
	case contains(name, "error"):
		return currentValue * 2 // Double current error rate
	case contains(name, "latency", "duration"):
		return currentValue * 3 // Triple current latency
	default:
		return 0 // No threshold
	}
}

func (a *MetricAgent) calculatePredictionConfidence(values []types.DataPoint, slope float64) float64 {
	if len(values) < 10 {
		return 0.3
	}

	// Calculate R-squared
	mean, _ := calculateMeanStdDev(values)
	var ssRes, ssTot float64

	for i, v := range values {
		predicted := values[0].Value + slope*float64(i)
		ssRes += (v.Value - predicted) * (v.Value - predicted)
		ssTot += (v.Value - mean) * (v.Value - mean)
	}

	if ssTot == 0 {
		return 0.5
	}

	rSquared := 1 - (ssRes / ssTot)
	return math.Max(0.3, math.Min(0.95, rSquared))
}

// MetricAnalysisLLMResult represents the LLM analysis result
type MetricAnalysisLLMResult struct {
	Summary         string               `json:"summary"`
	Findings        []types.Finding      `json:"findings"`
	Predictions     []types.Prediction   `json:"predictions,omitempty"`
	CapacityStatus  map[string]string    `json:"capacity_status,omitempty"`
	Recommendations []RecommendationLLM  `json:"recommendations,omitempty"`
}

func (a *MetricAgent) llmAnalysis(ctx context.Context, data MetricAnalysisData, ragContext string, query string) (*MetricAnalysisLLMResult, error) {
	dataJSON, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal metric data: %w", err)
	}

	systemPrompt := llm.MetricAnalysisSystemPrompt

	userPrompt := fmt.Sprintf("Analyze the following metric data:\n\n%s", string(dataJSON))
	if query != "" {
		userPrompt += fmt.Sprintf("\n\nSpecific query: %s", query)
	}

	var result MetricAnalysisLLMResult
	err = a.llmClient.CompleteJSON(ctx, &types.LLMRequest{
		SystemPrompt: systemPrompt,
		UserPrompt:   userPrompt,
		Temperature:  0.1,
	}, &result)
	if err != nil {
		return nil, err
	}

	for i := range result.Findings {
		result.Findings[i].ID = fmt.Sprintf("finding_%d_%d", time.Now().UnixNano(), i)
		result.Findings[i].Timestamp = time.Now()
	}

	return &result, nil
}

func (a *MetricAgent) generateStatisticalFindings(anomalies []types.Anomaly, predictions []types.Prediction) []types.Finding {
	var findings []types.Finding

	// Group anomalies by metric
	anomalyByMetric := make(map[string][]types.Anomaly)
	for _, anomaly := range anomalies {
		anomalyByMetric[anomaly.Metric] = append(anomalyByMetric[anomaly.Metric], anomaly)
	}

	for metric, metricAnomalies := range anomalyByMetric {
		maxSeverity := types.SeverityLow
		for _, a := range metricAnomalies {
			if severityValue(a.Severity) > severityValue(maxSeverity) {
				maxSeverity = a.Severity
			}
		}

		findings = append(findings, CreateFinding(
			types.FindingTypeCapacity,
			maxSeverity,
			fmt.Sprintf("Anomalies detected in %s", metric),
			fmt.Sprintf("%d anomalies detected with max deviation of %.2f standard deviations",
				len(metricAnomalies), metricAnomalies[0].Deviation),
		))
	}

	// Prediction findings
	for _, pred := range predictions {
		findings = append(findings, CreateFinding(
			types.FindingTypeCapacity,
			types.SeverityMedium,
			fmt.Sprintf("Capacity warning: %s", pred.Metric),
			fmt.Sprintf("Predicted to reach threshold in %s (confidence: %.0f%%)",
				pred.TimeToThreshold, pred.Confidence*100),
		))
	}

	return findings
}

// Helper functions

func calculateMeanStdDev(values []types.DataPoint) (float64, float64) {
	if len(values) == 0 {
		return 0, 0
	}

	var sum float64
	for _, v := range values {
		sum += v.Value
	}
	mean := sum / float64(len(values))

	var variance float64
	for _, v := range values {
		diff := v.Value - mean
		variance += diff * diff
	}
	variance /= float64(len(values))

	return mean, math.Sqrt(variance)
}

func getMinMax(values []types.DataPoint) (float64, float64) {
	if len(values) == 0 {
		return 0, 0
	}

	minVal := values[0].Value
	maxVal := values[0].Value

	for _, v := range values {
		if v.Value < minVal {
			minVal = v.Value
		}
		if v.Value > maxVal {
			maxVal = v.Value
		}
	}

	return minVal, maxVal
}

func pearsonCorrelation(a, b []types.DataPoint) float64 {
	n := min(len(a), len(b))
	if n < 3 {
		return 0
	}

	var sumA, sumB, sumAB, sumA2, sumB2 float64
	for i := 0; i < n; i++ {
		sumA += a[i].Value
		sumB += b[i].Value
		sumAB += a[i].Value * b[i].Value
		sumA2 += a[i].Value * a[i].Value
		sumB2 += b[i].Value * b[i].Value
	}

	nf := float64(n)
	numerator := nf*sumAB - sumA*sumB
	denominator := math.Sqrt((nf*sumA2 - sumA*sumA) * (nf*sumB2 - sumB*sumB))

	if denominator == 0 {
		return 0
	}

	return numerator / denominator
}

func linearRegression(values []types.DataPoint) (slope, intercept float64) {
	n := len(values)
	if n < 2 {
		return 0, 0
	}

	var sumX, sumY, sumXY, sumX2 float64
	for i, v := range values {
		x := float64(i)
		sumX += x
		sumY += v.Value
		sumXY += x * v.Value
		sumX2 += x * x
	}

	nf := float64(n)
	denominator := nf*sumX2 - sumX*sumX
	if denominator == 0 {
		return 0, sumY / nf
	}

	slope = (nf*sumXY - sumX*sumY) / denominator
	intercept = (sumY - slope*sumX) / nf

	return slope, intercept
}

func contains(s string, substrs ...string) bool {
	for _, substr := range substrs {
		if len(s) >= len(substr) {
			for i := 0; i <= len(s)-len(substr); i++ {
				if s[i:i+len(substr)] == substr {
					return true
				}
			}
		}
	}
	return false
}

func severityValue(s types.Severity) int {
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
