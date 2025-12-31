// Package collectors provides the Jaeger trace collector
package collectors

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/traceanalysis/rag-temporal/internal/config"
	"github.com/traceanalysis/rag-temporal/internal/types"
)

// JaegerCollector collects traces from Jaeger
type JaegerCollector struct {
	baseURL string
	client  *http.Client
	config  *config.JaegerConfig
}

// NewJaegerCollector creates a new Jaeger collector
func NewJaegerCollector(cfg *config.JaegerConfig) (*JaegerCollector, error) {
	if !cfg.Enabled {
		return nil, fmt.Errorf("jaeger collector is not enabled")
	}

	return &JaegerCollector{
		baseURL: cfg.QueryURL,
		client:  &http.Client{Timeout: cfg.Timeout},
		config:  cfg,
	}, nil
}

// JaegerTrace represents a trace from Jaeger API
type JaegerTrace struct {
	TraceID   string       `json:"traceID"`
	Spans     []JaegerSpan `json:"spans"`
	Processes map[string]JaegerProcess `json:"processes"`
}

// JaegerSpan represents a span from Jaeger API
type JaegerSpan struct {
	TraceID       string           `json:"traceID"`
	SpanID        string           `json:"spanID"`
	OperationName string           `json:"operationName"`
	References    []JaegerReference `json:"references"`
	StartTime     int64            `json:"startTime"` // microseconds
	Duration      int64            `json:"duration"`  // microseconds
	Tags          []JaegerTag      `json:"tags"`
	Logs          []JaegerLog      `json:"logs"`
	ProcessID     string           `json:"processID"`
	Warnings      []string         `json:"warnings"`
}

// JaegerReference represents a span reference
type JaegerReference struct {
	RefType string `json:"refType"`
	TraceID string `json:"traceID"`
	SpanID  string `json:"spanID"`
}

// JaegerTag represents a span tag
type JaegerTag struct {
	Key   string      `json:"key"`
	Type  string      `json:"type"`
	Value interface{} `json:"value"`
}

// JaegerLog represents a span log
type JaegerLog struct {
	Timestamp int64       `json:"timestamp"`
	Fields    []JaegerTag `json:"fields"`
}

// JaegerProcess represents a process
type JaegerProcess struct {
	ServiceName string      `json:"serviceName"`
	Tags        []JaegerTag `json:"tags"`
}

// JaegerResponse is the API response wrapper
type JaegerResponse struct {
	Data   []JaegerTrace `json:"data"`
	Errors []interface{} `json:"errors"`
}

// FetchTraces fetches traces from Jaeger
func (c *JaegerCollector) FetchTraces(ctx context.Context, timeRange types.TimeRange, services []string, limit int) ([]types.Trace, error) {
	var allTraces []types.Trace

	// If no services specified, get all services
	if len(services) == 0 {
		var err error
		services, err = c.GetServices(ctx)
		if err != nil {
			return nil, err
		}
	}

	for _, service := range services {
		traces, err := c.fetchServiceTraces(ctx, service, timeRange, limit/len(services))
		if err != nil {
			continue // Log and continue with other services
		}
		allTraces = append(allTraces, traces...)
	}

	return allTraces, nil
}

func (c *JaegerCollector) fetchServiceTraces(ctx context.Context, service string, timeRange types.TimeRange, limit int) ([]types.Trace, error) {
	params := url.Values{}
	params.Set("service", service)
	params.Set("start", strconv.FormatInt(timeRange.Start.UnixMicro(), 10))
	params.Set("end", strconv.FormatInt(timeRange.End.UnixMicro(), 10))
	params.Set("limit", strconv.Itoa(limit))

	reqURL := fmt.Sprintf("%s/api/traces?%s", c.baseURL, params.Encode())

	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("jaeger returned status %d: %s", resp.StatusCode, string(body))
	}

	var jaegerResp JaegerResponse
	if err := json.NewDecoder(resp.Body).Decode(&jaegerResp); err != nil {
		return nil, err
	}

	return c.convertTraces(jaegerResp.Data), nil
}

// GetServices returns all available services
func (c *JaegerCollector) GetServices(ctx context.Context) ([]string, error) {
	reqURL := fmt.Sprintf("%s/api/services", c.baseURL)

	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Data []string `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Data, nil
}

// GetServiceGraph returns the service dependency graph
func (c *JaegerCollector) GetServiceGraph(ctx context.Context, timeRange types.TimeRange) (*types.ServiceGraph, error) {
	params := url.Values{}
	params.Set("start", strconv.FormatInt(timeRange.Start.UnixMicro(), 10))
	params.Set("end", strconv.FormatInt(timeRange.End.UnixMicro(), 10))

	reqURL := fmt.Sprintf("%s/api/dependencies?%s", c.baseURL, params.Encode())

	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Data []struct {
			Parent    string `json:"parent"`
			Child     string `json:"child"`
			CallCount int    `json:"callCount"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	// Build graph
	nodeMap := make(map[string]*types.ServiceNode)
	var edges []types.ServiceEdge

	for _, dep := range result.Data {
		// Add nodes
		if _, ok := nodeMap[dep.Parent]; !ok {
			nodeMap[dep.Parent] = &types.ServiceNode{
				ID:   dep.Parent,
				Name: dep.Parent,
				Type: "service",
			}
		}
		if _, ok := nodeMap[dep.Child]; !ok {
			nodeMap[dep.Child] = &types.ServiceNode{
				ID:   dep.Child,
				Name: dep.Child,
				Type: "service",
			}
		}

		// Add edge
		edges = append(edges, types.ServiceEdge{
			Source:      dep.Parent,
			Target:      dep.Child,
			RequestRate: float64(dep.CallCount),
		})
	}

	nodes := make([]types.ServiceNode, 0, len(nodeMap))
	for _, node := range nodeMap {
		nodes = append(nodes, *node)
	}

	return &types.ServiceGraph{
		Nodes: nodes,
		Edges: edges,
	}, nil
}

// GetTrace fetches a specific trace by ID
func (c *JaegerCollector) GetTrace(ctx context.Context, traceID string) (*types.Trace, error) {
	reqURL := fmt.Sprintf("%s/api/traces/%s", c.baseURL, traceID)

	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result JaegerResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if len(result.Data) == 0 {
		return nil, fmt.Errorf("trace not found: %s", traceID)
	}

	traces := c.convertTraces(result.Data)
	return &traces[0], nil
}

// Health checks if Jaeger is healthy
func (c *JaegerCollector) Health(ctx context.Context) error {
	reqURL := fmt.Sprintf("%s/api/services", c.baseURL)
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
		return fmt.Errorf("jaeger returned status %d", resp.StatusCode)
	}

	return nil
}

func (c *JaegerCollector) convertTraces(jaegerTraces []JaegerTrace) []types.Trace {
	traces := make([]types.Trace, 0, len(jaegerTraces))

	for _, jt := range jaegerTraces {
		trace := types.Trace{
			TraceID:  jt.TraceID,
			Spans:    make([]types.Span, 0, len(jt.Spans)),
			Services: make([]string, 0),
		}

		serviceSet := make(map[string]bool)
		var minStart, maxEnd int64

		for i, js := range jt.Spans {
			process := jt.Processes[js.ProcessID]
			serviceName := process.ServiceName

			if !serviceSet[serviceName] {
				serviceSet[serviceName] = true
				trace.Services = append(trace.Services, serviceName)
			}

			// Convert tags
			tags := make(map[string]string)
			status := "OK"
			statusMessage := ""
			for _, tag := range js.Tags {
				tags[tag.Key] = fmt.Sprintf("%v", tag.Value)
				if tag.Key == "error" && tag.Value == true {
					status = "ERROR"
					trace.HasError = true
				}
				if tag.Key == "otel.status_code" && tag.Value == "ERROR" {
					status = "ERROR"
					trace.HasError = true
				}
				if tag.Key == "otel.status_description" {
					statusMessage = fmt.Sprintf("%v", tag.Value)
				}
			}

			// Convert logs
			var logs []types.SpanLog
			for _, jl := range js.Logs {
				fields := make(map[string]string)
				for _, f := range jl.Fields {
					fields[f.Key] = fmt.Sprintf("%v", f.Value)
				}
				logs = append(logs, types.SpanLog{
					Timestamp: time.UnixMicro(jl.Timestamp),
					Fields:    fields,
				})
			}

			// Get parent ID
			parentID := ""
			for _, ref := range js.References {
				if ref.RefType == "CHILD_OF" {
					parentID = ref.SpanID
					break
				}
			}

			startTime := time.UnixMicro(js.StartTime)
			endTime := startTime.Add(time.Duration(js.Duration) * time.Microsecond)

			span := types.Span{
				SpanID:        js.SpanID,
				ParentID:      parentID,
				TraceID:       js.TraceID,
				Service:       serviceName,
				Operation:     js.OperationName,
				Duration:      time.Duration(js.Duration) * time.Microsecond,
				Status:        status,
				StatusMessage: statusMessage,
				Tags:          tags,
				Logs:          logs,
				StartTime:     startTime,
				EndTime:       endTime,
			}

			trace.Spans = append(trace.Spans, span)

			// Track timing
			if i == 0 || js.StartTime < minStart {
				minStart = js.StartTime
			}
			endMicros := js.StartTime + js.Duration
			if i == 0 || endMicros > maxEnd {
				maxEnd = endMicros
			}

			// Find root span
			if parentID == "" {
				trace.RootSpan = &trace.Spans[len(trace.Spans)-1]
			}
		}

		trace.Duration = time.Duration(maxEnd-minStart) * time.Microsecond
		trace.StartTime = time.UnixMicro(minStart)

		traces = append(traces, trace)
	}

	return traces
}
