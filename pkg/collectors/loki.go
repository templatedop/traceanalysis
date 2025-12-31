// Package collectors provides the Loki log collector
package collectors

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/traceanalysis/rag-temporal/internal/config"
	"github.com/traceanalysis/rag-temporal/internal/types"
	"github.com/traceanalysis/rag-temporal/pkg/agents"
)

// LokiCollector collects logs from Loki
type LokiCollector struct {
	baseURL string
	client  *http.Client
	config  *config.LokiConfig
}

// NewLokiCollector creates a new Loki collector
func NewLokiCollector(cfg *config.LokiConfig) (*LokiCollector, error) {
	if !cfg.Enabled {
		return nil, fmt.Errorf("loki collector is not enabled")
	}

	return &LokiCollector{
		baseURL: cfg.URL,
		client:  &http.Client{Timeout: cfg.Timeout},
		config:  cfg,
	}, nil
}

// LokiQueryResponse represents the Loki query response
type LokiQueryResponse struct {
	Status string `json:"status"`
	Data   struct {
		ResultType string       `json:"resultType"`
		Result     []LokiStream `json:"result"`
	} `json:"data"`
}

// LokiStream represents a log stream
type LokiStream struct {
	Stream map[string]string `json:"stream"`
	Values [][]string        `json:"values"` // [timestamp, log line]
}

// Query executes a LogQL query
func (c *LokiCollector) Query(ctx context.Context, query string, timeRange types.TimeRange, limit int) ([]agents.LogEntry, error) {
	params := url.Values{}
	params.Set("query", query)
	params.Set("start", strconv.FormatInt(timeRange.Start.UnixNano(), 10))
	params.Set("end", strconv.FormatInt(timeRange.End.UnixNano(), 10))
	params.Set("limit", strconv.Itoa(limit))
	params.Set("direction", "backward")

	reqURL := fmt.Sprintf("%s/loki/api/v1/query_range?%s", c.baseURL, params.Encode())

	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, err
	}

	if c.config.Username != "" {
		req.SetBasicAuth(c.config.Username, c.config.Password)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("loki returned status %d: %s", resp.StatusCode, string(body))
	}

	var lokiResp LokiQueryResponse
	if err := json.NewDecoder(resp.Body).Decode(&lokiResp); err != nil {
		return nil, err
	}

	return c.convertToLogEntries(lokiResp.Data.Result), nil
}

// GetLabels returns all label names
func (c *LokiCollector) GetLabels(ctx context.Context) ([]string, error) {
	reqURL := fmt.Sprintf("%s/loki/api/v1/labels", c.baseURL)

	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, err
	}

	if c.config.Username != "" {
		req.SetBasicAuth(c.config.Username, c.config.Password)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Status string   `json:"status"`
		Data   []string `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Data, nil
}

// GetLabelValues returns values for a specific label
func (c *LokiCollector) GetLabelValues(ctx context.Context, label string) ([]string, error) {
	reqURL := fmt.Sprintf("%s/loki/api/v1/label/%s/values", c.baseURL, label)

	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, err
	}

	if c.config.Username != "" {
		req.SetBasicAuth(c.config.Username, c.config.Password)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Status string   `json:"status"`
		Data   []string `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Data, nil
}

// Health checks if Loki is healthy
func (c *LokiCollector) Health(ctx context.Context) error {
	reqURL := fmt.Sprintf("%s/ready", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("loki returned status %d", resp.StatusCode)
	}

	return nil
}

func (c *LokiCollector) convertToLogEntries(streams []LokiStream) []agents.LogEntry {
	var entries []agents.LogEntry

	for _, stream := range streams {
		for _, value := range stream.Values {
			if len(value) < 2 {
				continue
			}

			// Parse timestamp (nanoseconds)
			ts, err := strconv.ParseInt(value[0], 10, 64)
			if err != nil {
				continue
			}

			line := value[1]
			level := extractLogLevel(line)
			service := stream.Stream["service"]
			if service == "" {
				service = stream.Stream["app"]
			}
			if service == "" {
				service = stream.Stream["job"]
			}

			entries = append(entries, agents.LogEntry{
				Timestamp: time.Unix(0, ts),
				Line:      line,
				Labels:    stream.Stream,
				Level:     level,
				Service:   service,
			})
		}
	}

	return entries
}

// extractLogLevel attempts to extract log level from log line
func extractLogLevel(line string) string {
	lineLower := strings.ToLower(line)

	patterns := []struct {
		pattern string
		level   string
	}{
		{`"level"\s*:\s*"(debug|info|warn|warning|error|fatal|critical)"`, ""},
		{`\[(debug|info|warn|warning|error|fatal|critical)\]`, ""},
		{`\b(DEBUG|INFO|WARN|WARNING|ERROR|FATAL|CRITICAL)\b`, ""},
	}

	for _, p := range patterns {
		re := regexp.MustCompile(p.pattern)
		matches := re.FindStringSubmatch(line)
		if len(matches) > 1 {
			return strings.ToUpper(matches[1])
		}
		matches = re.FindStringSubmatch(lineLower)
		if len(matches) > 1 {
			return strings.ToUpper(matches[1])
		}
	}

	// Simple keyword matching
	if strings.Contains(lineLower, "error") || strings.Contains(lineLower, "err") {
		return "ERROR"
	}
	if strings.Contains(lineLower, "warn") {
		return "WARN"
	}
	if strings.Contains(lineLower, "debug") {
		return "DEBUG"
	}
	if strings.Contains(lineLower, "fatal") || strings.Contains(lineLower, "critical") {
		return "FATAL"
	}

	return "INFO"
}

// CommonLogQueries returns common log queries for observability
func CommonLogQueries() map[string]string {
	return map[string]string{
		"all_errors":      `{level=~"error|Error|ERROR"}`,
		"all_warnings":    `{level=~"warn|Warn|WARN|warning|Warning|WARNING"}`,
		"http_errors":     `{job=~".+"} |~ "HTTP.*[45][0-9][0-9]"`,
		"exceptions":      `{job=~".+"} |~ "(?i)(exception|panic|fatal)"`,
		"slow_requests":   `{job=~".+"} |~ "(?i)(slow|timeout|deadline)"`,
		"connection_errors": `{job=~".+"} |~ "(?i)(connection refused|connection reset|ECONNREFUSED)"`,
		"oom":             `{job=~".+"} |~ "(?i)(out of memory|OOM|oom-killer)"`,
	}
}
