// Package collectors provides data collectors for observability sources
package collectors

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"

	"github.com/traceanalysis/rag-temporal/internal/config"
	"github.com/traceanalysis/rag-temporal/internal/types"
)

// PrometheusCollector collects metrics from Prometheus
type PrometheusCollector struct {
	api     v1.API
	config  *config.PrometheusConfig
	client  *http.Client
}

// NewPrometheusCollector creates a new Prometheus collector
func NewPrometheusCollector(cfg *config.PrometheusConfig) (*PrometheusCollector, error) {
	if !cfg.Enabled {
		return nil, fmt.Errorf("prometheus collector is not enabled")
	}

	client, err := api.NewClient(api.Config{
		Address: cfg.URL,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create Prometheus client: %w", err)
	}

	return &PrometheusCollector{
		api:    v1.NewAPI(client),
		config: cfg,
		client: &http.Client{Timeout: cfg.Timeout},
	}, nil
}

// Query executes an instant query
func (c *PrometheusCollector) Query(ctx context.Context, query string, timeRange types.TimeRange) ([]types.TimeSeries, error) {
	result, warnings, err := c.api.Query(ctx, query, timeRange.End)
	if err != nil {
		return nil, fmt.Errorf("prometheus query failed: %w", err)
	}

	if len(warnings) > 0 {
		// Log warnings but continue
		for _, w := range warnings {
			fmt.Printf("Prometheus warning: %s\n", w)
		}
	}

	return c.convertResult(result), nil
}

// QueryRange executes a range query
func (c *PrometheusCollector) QueryRange(ctx context.Context, query string, timeRange types.TimeRange, step time.Duration) ([]types.TimeSeries, error) {
	r := v1.Range{
		Start: timeRange.Start,
		End:   timeRange.End,
		Step:  step,
	}

	result, warnings, err := c.api.QueryRange(ctx, query, r)
	if err != nil {
		return nil, fmt.Errorf("prometheus range query failed: %w", err)
	}

	if len(warnings) > 0 {
		for _, w := range warnings {
			fmt.Printf("Prometheus warning: %s\n", w)
		}
	}

	return c.convertResult(result), nil
}

// GetMetricNames returns all metric names
func (c *PrometheusCollector) GetMetricNames(ctx context.Context) ([]string, error) {
	names, warnings, err := c.api.LabelValues(ctx, "__name__", nil, time.Now().Add(-1*time.Hour), time.Now())
	if err != nil {
		return nil, fmt.Errorf("failed to get metric names: %w", err)
	}

	if len(warnings) > 0 {
		for _, w := range warnings {
			fmt.Printf("Prometheus warning: %s\n", w)
		}
	}

	result := make([]string, len(names))
	for i, n := range names {
		result[i] = string(n)
	}

	return result, nil
}

// GetLabels returns all label names
func (c *PrometheusCollector) GetLabels(ctx context.Context) ([]string, error) {
	labels, warnings, err := c.api.LabelNames(ctx, nil, time.Now().Add(-1*time.Hour), time.Now())
	if err != nil {
		return nil, fmt.Errorf("failed to get labels: %w", err)
	}

	if len(warnings) > 0 {
		for _, w := range warnings {
			fmt.Printf("Prometheus warning: %s\n", w)
		}
	}

	return labels, nil
}

// Health checks if Prometheus is healthy
func (c *PrometheusCollector) Health(ctx context.Context) error {
	_, err := c.api.Config(ctx)
	return err
}

func (c *PrometheusCollector) convertResult(value model.Value) []types.TimeSeries {
	var series []types.TimeSeries

	switch v := value.(type) {
	case model.Vector:
		for _, sample := range v {
			labels := make(map[string]string)
			for k, v := range sample.Metric {
				labels[string(k)] = string(v)
			}
			name := string(sample.Metric["__name__"])
			if name == "" {
				name = "unnamed"
			}

			series = append(series, types.TimeSeries{
				Name:   name,
				Labels: labels,
				Values: []types.DataPoint{
					{
						Timestamp: sample.Timestamp.Time(),
						Value:     float64(sample.Value),
					},
				},
			})
		}

	case model.Matrix:
		for _, stream := range v {
			labels := make(map[string]string)
			for k, v := range stream.Metric {
				labels[string(k)] = string(v)
			}
			name := string(stream.Metric["__name__"])
			if name == "" {
				name = "unnamed"
			}

			var values []types.DataPoint
			for _, sp := range stream.Values {
				values = append(values, types.DataPoint{
					Timestamp: sp.Timestamp.Time(),
					Value:     float64(sp.Value),
				})
			}

			series = append(series, types.TimeSeries{
				Name:   name,
				Labels: labels,
				Values: values,
			})
		}

	case *model.Scalar:
		series = append(series, types.TimeSeries{
			Name: "scalar",
			Values: []types.DataPoint{
				{
					Timestamp: v.Timestamp.Time(),
					Value:     float64(v.Value),
				},
			},
		})
	}

	return series
}

// KeyMetricQueries returns common observability metric queries
func KeyMetricQueries() []string {
	return []string{
		// Request rate
		`sum(rate(http_requests_total[5m])) by (service)`,
		// Error rate
		`sum(rate(http_requests_total{status=~"5.."}[5m])) by (service) / sum(rate(http_requests_total[5m])) by (service)`,
		// Latency P95
		`histogram_quantile(0.95, sum(rate(http_request_duration_seconds_bucket[5m])) by (le, service))`,
		// Latency P99
		`histogram_quantile(0.99, sum(rate(http_request_duration_seconds_bucket[5m])) by (le, service))`,
		// CPU usage
		`sum(rate(container_cpu_usage_seconds_total[5m])) by (container)`,
		// Memory usage
		`sum(container_memory_usage_bytes) by (container)`,
		// Goroutines (Go services)
		`go_goroutines`,
		// GC duration (Go services)
		`rate(go_gc_duration_seconds_sum[5m])`,
	}
}
