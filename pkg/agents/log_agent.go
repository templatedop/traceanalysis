// Package agents provides the log analysis agent
package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/traceanalysis/rag-temporal/internal/types"
	"github.com/traceanalysis/rag-temporal/pkg/llm"
	"github.com/traceanalysis/rag-temporal/pkg/rag"
)

// LogAgent analyzes log data
type LogAgent struct {
	*BaseAgent
	collector    LogCollector
	templateMap  map[string]*LogTemplatePattern
}

// LogCollector defines the interface for fetching logs
type LogCollector interface {
	Query(ctx context.Context, query string, timeRange types.TimeRange, limit int) ([]LogEntry, error)
	GetLabels(ctx context.Context) ([]string, error)
}

// LogEntry represents a single log entry
type LogEntry struct {
	Timestamp time.Time         `json:"timestamp"`
	Line      string            `json:"line"`
	Labels    map[string]string `json:"labels,omitempty"`
	Level     string            `json:"level,omitempty"`
	Service   string            `json:"service,omitempty"`
}

// LogTemplatePattern represents a parsed log template
type LogTemplatePattern struct {
	Template   string
	Pattern    *regexp.Regexp
	Count      int
	FirstSeen  time.Time
	LastSeen   time.Time
	Level      string
	Services   map[string]bool
	SampleLogs []string
}

// NewLogAgent creates a new log analysis agent
func NewLogAgent(llmClient llm.Client, ragStore rag.Store, collector LogCollector) *LogAgent {
	return &LogAgent{
		BaseAgent:   NewBaseAgent("log-analyzer", types.AnalysisTypeLog, llmClient, ragStore),
		collector:   collector,
		templateMap: make(map[string]*LogTemplatePattern),
	}
}

// Analyze performs log analysis
func (a *LogAgent) Analyze(ctx context.Context, request *types.AnalysisRequest) (*types.AgentResult, error) {
	startTime := time.Now()

	result := &types.AgentResult{
		AgentName: a.Name(),
		Type:      a.Type(),
	}

	// Build query based on services
	query := "{}"
	if len(request.Services) > 0 {
		query = fmt.Sprintf("{service=~\"%s\"}", strings.Join(request.Services, "|"))
	}

	// Fetch logs
	logs, err := a.collector.Query(ctx, query, request.TimeRange, 10000)
	if err != nil {
		result.Error = fmt.Sprintf("failed to fetch logs: %v", err)
		return result, nil
	}

	if len(logs) == 0 {
		result.Findings = append(result.Findings, CreateFinding(
			types.FindingTypeReliability,
			types.SeverityLow,
			"No logs found",
			"No logs found for the specified time range and services",
		))
		result.ProcessingTime = time.Since(startTime)
		return result, nil
	}

	// Parse logs into templates
	templates := a.parseLogTemplates(logs)

	// Group errors
	errorGroups := a.groupErrors(logs, templates)

	// Detect anomalies
	anomalies := a.detectLogAnomalies(logs, templates)

	// Calculate statistics
	stats := a.calculateLogStats(logs, templates, errorGroups)

	// Get RAG context
	ragContext, _ := a.GetRAGContext(ctx, request.Query, 5)

	// Prepare data for LLM
	logData := LogAnalysisData{
		Templates:    a.templatesToSummary(templates),
		ErrorGroups:  errorGroups,
		Anomalies:    anomalies,
		Statistics:   stats,
		SampleErrors: a.getSampleErrors(logs, 20),
	}

	// LLM analysis
	llmResult, err := a.llmAnalysis(ctx, logData, ragContext, request.Query)
	if err != nil {
		result.Findings = a.generateStatisticalFindings(errorGroups, anomalies)
		result.RawData = logData
		result.ProcessingTime = time.Since(startTime)
		result.Error = fmt.Sprintf("LLM analysis failed: %v", err)
		return result, nil
	}

	result.Findings = llmResult.Findings
	result.RawData = logData
	result.ProcessingTime = time.Since(startTime)

	return result, nil
}

// LogAnalysisData contains data for log analysis
type LogAnalysisData struct {
	Templates    []LogTemplateSummary `json:"templates"`
	ErrorGroups  []types.ErrorGroup   `json:"error_groups"`
	Anomalies    []types.LogAnomaly   `json:"anomalies"`
	Statistics   types.LogStatistics  `json:"statistics"`
	SampleErrors []LogErrorSummary    `json:"sample_errors"`
}

// LogTemplateSummary summarizes a log template
type LogTemplateSummary struct {
	Template  string   `json:"template"`
	Count     int      `json:"count"`
	Level     string   `json:"level"`
	Services  []string `json:"services"`
	FirstSeen string   `json:"first_seen"`
	LastSeen  string   `json:"last_seen"`
}

// LogErrorSummary summarizes an error log
type LogErrorSummary struct {
	Timestamp string `json:"timestamp"`
	Service   string `json:"service"`
	Level     string `json:"level"`
	Message   string `json:"message"`
}

func (a *LogAgent) parseLogTemplates(logs []LogEntry) map[string]*LogTemplatePattern {
	templates := make(map[string]*LogTemplatePattern)

	for _, log := range logs {
		template := a.extractTemplate(log.Line)

		if existing, ok := templates[template]; ok {
			existing.Count++
			if log.Timestamp.Before(existing.FirstSeen) {
				existing.FirstSeen = log.Timestamp
			}
			if log.Timestamp.After(existing.LastSeen) {
				existing.LastSeen = log.Timestamp
			}
			if log.Service != "" {
				existing.Services[log.Service] = true
			}
			if len(existing.SampleLogs) < 3 {
				existing.SampleLogs = append(existing.SampleLogs, log.Line)
			}
		} else {
			services := make(map[string]bool)
			if log.Service != "" {
				services[log.Service] = true
			}
			templates[template] = &LogTemplatePattern{
				Template:   template,
				Count:      1,
				FirstSeen:  log.Timestamp,
				LastSeen:   log.Timestamp,
				Level:      log.Level,
				Services:   services,
				SampleLogs: []string{log.Line},
			}
		}
	}

	return templates
}

func (a *LogAgent) extractTemplate(line string) string {
	// Common patterns to replace with placeholders
	patterns := []struct {
		regex       *regexp.Regexp
		replacement string
	}{
		{regexp.MustCompile(`\b\d{4}-\d{2}-\d{2}[T ]\d{2}:\d{2}:\d{2}[.\d]*Z?\b`), "<TIMESTAMP>"},
		{regexp.MustCompile(`\b[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}\b`), "<UUID>"},
		{regexp.MustCompile(`\b\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}(:\d+)?\b`), "<IP>"},
		{regexp.MustCompile(`\b0x[0-9a-fA-F]+\b`), "<HEX>"},
		{regexp.MustCompile(`\b\d+\b`), "<NUM>"},
		{regexp.MustCompile(`"[^"]*"`), `"<STR>"`},
	}

	result := line
	for _, p := range patterns {
		result = p.regex.ReplaceAllString(result, p.replacement)
	}

	return result
}

func (a *LogAgent) groupErrors(logs []LogEntry, templates map[string]*LogTemplatePattern) []types.ErrorGroup {
	errorTemplates := make(map[string]*types.ErrorGroup)

	for _, log := range logs {
		level := strings.ToUpper(log.Level)
		if level != "ERROR" && level != "FATAL" && level != "CRITICAL" {
			continue
		}

		template := a.extractTemplate(log.Line)

		if existing, ok := errorTemplates[template]; ok {
			existing.Count++
			if log.Timestamp.Before(existing.FirstSeen) {
				existing.FirstSeen = log.Timestamp
			}
			if log.Timestamp.After(existing.LastSeen) {
				existing.LastSeen = log.Timestamp
			}
			if log.Service != "" && !containsString(existing.Services, log.Service) {
				existing.Services = append(existing.Services, log.Service)
			}
		} else {
			services := []string{}
			if log.Service != "" {
				services = []string{log.Service}
			}
			errorTemplates[template] = &types.ErrorGroup{
				ID:         fmt.Sprintf("error_%d", len(errorTemplates)),
				Pattern:    template,
				Message:    log.Line,
				Count:      1,
				FirstSeen:  log.Timestamp,
				LastSeen:   log.Timestamp,
				Services:   services,
				SampleLogs: []string{log.Line},
			}
		}
	}

	groups := make([]types.ErrorGroup, 0, len(errorTemplates))
	for _, g := range errorTemplates {
		groups = append(groups, *g)
	}

	// Sort by count descending
	sort.Slice(groups, func(i, j int) bool {
		return groups[i].Count > groups[j].Count
	})

	if len(groups) > 20 {
		groups = groups[:20]
	}

	return groups
}

func (a *LogAgent) detectLogAnomalies(logs []LogEntry, templates map[string]*LogTemplatePattern) []types.LogAnomaly {
	var anomalies []types.LogAnomaly

	// Count logs per hour
	hourCounts := make(map[string]int)
	for _, log := range logs {
		hour := log.Timestamp.Truncate(time.Hour).Format(time.RFC3339)
		hourCounts[hour]++
	}

	// Detect frequency anomalies
	if len(hourCounts) > 3 {
		var counts []int
		for _, c := range hourCounts {
			counts = append(counts, c)
		}
		mean, stdDev := calculateIntMeanStdDev(counts)

		for hour, count := range hourCounts {
			if stdDev > 0 {
				zScore := float64(count-int(mean)) / stdDev
				if zScore > 3 {
					anomalies = append(anomalies, types.LogAnomaly{
						ID:            fmt.Sprintf("anomaly_%d", len(anomalies)),
						Type:          "frequency_spike",
						Description:   fmt.Sprintf("Log frequency spike at %s", hour),
						Timestamp:     parseTime(hour),
						Severity:      types.SeverityMedium,
						ExpectedCount: int(mean),
						ActualCount:   count,
					})
				}
			}
		}
	}

	// Detect new patterns (patterns seen only recently)
	recentThreshold := time.Now().Add(-15 * time.Minute)
	for template, pattern := range templates {
		if pattern.FirstSeen.After(recentThreshold) && pattern.Count > 10 {
			anomalies = append(anomalies, types.LogAnomaly{
				ID:          fmt.Sprintf("anomaly_%d", len(anomalies)),
				Type:        "new_pattern",
				Description: "New log pattern detected",
				Timestamp:   pattern.FirstSeen,
				Severity:    types.SeverityLow,
				Pattern:     template,
				ActualCount: pattern.Count,
			})
		}
	}

	return anomalies
}

func (a *LogAgent) calculateLogStats(logs []LogEntry, templates map[string]*LogTemplatePattern, errors []types.ErrorGroup) types.LogStatistics {
	stats := types.LogStatistics{
		TotalLogs:      len(logs),
		LogsByLevel:    make(map[string]int),
		LogsByService:  make(map[string]int),
		UniquePatterns: len(templates),
	}

	for _, log := range logs {
		level := log.Level
		if level == "" {
			level = "UNKNOWN"
		}
		stats.LogsByLevel[strings.ToUpper(level)]++

		if log.Service != "" {
			stats.LogsByService[log.Service]++
		}
	}

	errorCount := stats.LogsByLevel["ERROR"] + stats.LogsByLevel["FATAL"] + stats.LogsByLevel["CRITICAL"]
	if len(logs) > 0 {
		stats.ErrorRate = float64(errorCount) / float64(len(logs)) * 100
	}

	stats.AnomalyCount = len(a.detectLogAnomalies(logs, templates))

	return stats
}

func (a *LogAgent) templatesToSummary(templates map[string]*LogTemplatePattern) []LogTemplateSummary {
	summaries := make([]LogTemplateSummary, 0, len(templates))

	for template, pattern := range templates {
		services := make([]string, 0, len(pattern.Services))
		for svc := range pattern.Services {
			services = append(services, svc)
		}

		summaries = append(summaries, LogTemplateSummary{
			Template:  template,
			Count:     pattern.Count,
			Level:     pattern.Level,
			Services:  services,
			FirstSeen: pattern.FirstSeen.Format(time.RFC3339),
			LastSeen:  pattern.LastSeen.Format(time.RFC3339),
		})
	}

	// Sort by count descending
	sort.Slice(summaries, func(i, j int) bool {
		return summaries[i].Count > summaries[j].Count
	})

	if len(summaries) > 50 {
		summaries = summaries[:50]
	}

	return summaries
}

func (a *LogAgent) getSampleErrors(logs []LogEntry, limit int) []LogErrorSummary {
	var errors []LogErrorSummary

	for _, log := range logs {
		level := strings.ToUpper(log.Level)
		if level != "ERROR" && level != "FATAL" && level != "CRITICAL" {
			continue
		}

		errors = append(errors, LogErrorSummary{
			Timestamp: log.Timestamp.Format(time.RFC3339),
			Service:   log.Service,
			Level:     log.Level,
			Message:   truncateString(log.Line, 500),
		})

		if len(errors) >= limit {
			break
		}
	}

	return errors
}

// LogAnalysisLLMResult represents the LLM analysis result
type LogAnalysisLLMResult struct {
	Summary          string                  `json:"summary"`
	Findings         []types.Finding         `json:"findings"`
	ErrorGroups      []ErrorGroupAnalysis    `json:"error_groups,omitempty"`
	Anomalies        []AnomalyAnalysis       `json:"anomalies,omitempty"`
	SecurityConcerns []SecurityConcern       `json:"security_concerns,omitempty"`
	Recommendations  []RecommendationLLM     `json:"recommendations,omitempty"`
}

// ErrorGroupAnalysis is LLM analysis of an error group
type ErrorGroupAnalysis struct {
	Pattern        string `json:"pattern"`
	Count          int    `json:"count"`
	Severity       string `json:"severity"`
	RootCause      string `json:"root_cause"`
	Recommendation string `json:"recommendation"`
}

// AnomalyAnalysis is LLM analysis of an anomaly
type AnomalyAnalysis struct {
	Type        string `json:"type"`
	Severity    string `json:"severity"`
	Description string `json:"description"`
	Pattern     string `json:"pattern,omitempty"`
}

// SecurityConcern represents a security-related finding
type SecurityConcern struct {
	Type        string   `json:"type"`
	Severity    string   `json:"severity"`
	Description string   `json:"description"`
	Evidence    []string `json:"evidence"`
}

func (a *LogAgent) llmAnalysis(ctx context.Context, data LogAnalysisData, ragContext string, query string) (*LogAnalysisLLMResult, error) {
	dataJSON, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal log data: %w", err)
	}

	systemPrompt := llm.LogAnalysisSystemPrompt

	userPrompt := fmt.Sprintf("Analyze the following log data:\n\n%s", string(dataJSON))
	if query != "" {
		userPrompt += fmt.Sprintf("\n\nSpecific query: %s", query)
	}

	var result LogAnalysisLLMResult
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

func (a *LogAgent) generateStatisticalFindings(errors []types.ErrorGroup, anomalies []types.LogAnomaly) []types.Finding {
	var findings []types.Finding

	// High-count error groups
	for _, eg := range errors[:min(5, len(errors))] {
		severity := types.SeverityMedium
		if eg.Count > 100 {
			severity = types.SeverityHigh
		}
		if eg.Count > 1000 {
			severity = types.SeverityCritical
		}

		findings = append(findings, CreateFinding(
			types.FindingTypeReliability,
			severity,
			fmt.Sprintf("High error count: %d occurrences", eg.Count),
			fmt.Sprintf("Error pattern: %s", truncateString(eg.Pattern, 200)),
		))
	}

	// Anomaly findings
	for _, anomaly := range anomalies {
		findings = append(findings, CreateFinding(
			types.FindingTypeReliability,
			anomaly.Severity,
			anomaly.Description,
			fmt.Sprintf("Type: %s, Pattern: %s", anomaly.Type, truncateString(anomaly.Pattern, 200)),
		))
	}

	return findings
}

// Helper functions

func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func calculateIntMeanStdDev(values []int) (float64, float64) {
	if len(values) == 0 {
		return 0, 0
	}

	var sum float64
	for _, v := range values {
		sum += float64(v)
	}
	mean := sum / float64(len(values))

	var variance float64
	for _, v := range values {
		diff := float64(v) - mean
		variance += diff * diff
	}
	variance /= float64(len(values))

	return mean, variance
}

func parseTime(s string) time.Time {
	t, _ := time.Parse(time.RFC3339, s)
	return t
}
