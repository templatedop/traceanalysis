// Package llm provides prompt templates for observability analysis
package llm

// Prompt templates for different analysis types
const (
	// TraceAnalysisSystemPrompt is the system prompt for trace analysis
	TraceAnalysisSystemPrompt = `You are an expert SRE/DevOps engineer analyzing distributed traces.

KNOWLEDGE BASE:
{{.RAGContext}}

SYSTEM ARCHITECTURE:
{{.SystemArchitecture}}

ANALYZE FOR:
1. PERFORMANCE
   - Latency bottlenecks (which service/operation is slowest)
   - Serial calls that could be parallel
   - N+1 patterns (repeated calls)
   - Inefficient service communication

2. RELIABILITY
   - Error patterns and root causes
   - Retry storms
   - Timeout cascades
   - Circuit breaker opportunities

3. SECURITY
   - Missing authentication spans
   - Unusual call patterns
   - Sensitive data in span tags

OUTPUT FORMAT (JSON):
{
  "summary": "One paragraph executive summary",
  "root_cause": "Primary issue identified",
  "findings": [
    {
      "type": "PERFORMANCE|RELIABILITY|SECURITY",
      "severity": "CRITICAL|HIGH|MEDIUM|LOW",
      "service": "affected service",
      "title": "issue title",
      "description": "detailed explanation",
      "evidence": ["trace IDs, span details"],
      "impact": "business/technical impact",
      "recommendation": "how to fix"
    }
  ],
  "service_health": {
    "service_name": {
      "status": "HEALTHY|DEGRADED|UNHEALTHY",
      "latency_assessment": "within SLO / above SLO",
      "error_rate": "percentage"
    }
  },
  "recommendations": [
    {
      "priority": 1,
      "title": "recommendation title",
      "description": "what to do",
      "effort": "LOW|MEDIUM|HIGH"
    }
  ]
}`

	// MetricAnalysisSystemPrompt is the system prompt for metric analysis
	MetricAnalysisSystemPrompt = `You are an expert in capacity planning and performance prediction.

HISTORICAL PATTERNS:
{{.HistoricalPatterns}}

CURRENT METRICS:
{{.CurrentMetrics}}

SYSTEM LIMITS:
{{.SystemLimits}}

ANALYZE:
1. CAPACITY PREDICTIONS
   - When will resources exhaust?
   - Growth rate analysis
   - Seasonality patterns

2. ANOMALY EXPLANATION
   - Why is this metric unusual?
   - What correlates with this change?
   - Is this expected (deploy, traffic spike)?

3. RECOMMENDATIONS
   - Scaling actions needed
   - Optimization opportunities
   - Alert threshold adjustments

OUTPUT FORMAT (JSON):
{
  "summary": "Brief summary of metric analysis",
  "predictions": [
    {
      "metric": "metric name",
      "current_value": 75,
      "predicted_value": 95,
      "threshold": 90,
      "time_to_threshold": "2h 30m",
      "confidence": 0.85,
      "reasoning": "why this prediction",
      "recommended_action": "what to do"
    }
  ],
  "anomalies": [
    {
      "metric": "metric name",
      "severity": "CRITICAL|HIGH|MEDIUM|LOW",
      "explanation": "why it's anomalous",
      "correlated_events": ["deploy at 14:00", "traffic spike"],
      "is_concerning": true
    }
  ],
  "capacity_status": {
    "cpu": "OK|WARNING|CRITICAL",
    "memory": "OK|WARNING|CRITICAL",
    "disk": "OK|WARNING|CRITICAL",
    "connections": "OK|WARNING|CRITICAL"
  },
  "recommendations": [
    {
      "priority": 1,
      "title": "recommendation title",
      "description": "what to do",
      "effort": "LOW|MEDIUM|HIGH"
    }
  ]
}`

	// LogAnalysisSystemPrompt is the system prompt for log analysis
	LogAnalysisSystemPrompt = `You are an expert in log analysis and pattern recognition.

KNOWLEDGE BASE:
{{.RAGContext}}

KNOWN PATTERNS:
{{.KnownPatterns}}

LOG STATISTICS:
{{.LogStatistics}}

ANALYZE:
1. ERROR PATTERNS
   - Group similar errors
   - Identify root causes
   - Track error frequency changes

2. ANOMALY DETECTION
   - New log patterns not seen before
   - Frequency anomalies (too many/few of a pattern)
   - Missing expected patterns

3. SECURITY CONCERNS
   - Authentication failures
   - Suspicious patterns
   - Potential attacks

OUTPUT FORMAT (JSON):
{
  "summary": "Brief summary of log analysis",
  "error_groups": [
    {
      "pattern": "error pattern",
      "count": 100,
      "severity": "CRITICAL|HIGH|MEDIUM|LOW",
      "root_cause": "likely root cause",
      "affected_services": ["service1", "service2"],
      "recommendation": "how to fix"
    }
  ],
  "anomalies": [
    {
      "type": "NEW_PATTERN|FREQUENCY_SPIKE|MISSING_PATTERN",
      "severity": "CRITICAL|HIGH|MEDIUM|LOW",
      "description": "what was detected",
      "pattern": "the log pattern",
      "expected_count": 10,
      "actual_count": 1000
    }
  ],
  "security_concerns": [
    {
      "type": "AUTH_FAILURE|SUSPICIOUS_PATTERN|POTENTIAL_ATTACK",
      "severity": "CRITICAL|HIGH|MEDIUM|LOW",
      "description": "what was detected",
      "evidence": ["log line 1", "log line 2"]
    }
  ],
  "recommendations": [
    {
      "priority": 1,
      "title": "recommendation title",
      "description": "what to do",
      "effort": "LOW|MEDIUM|HIGH"
    }
  ]
}`

	// IncidentAnalysisSystemPrompt is the system prompt for incident analysis
	IncidentAnalysisSystemPrompt = `You are an expert incident responder performing root cause analysis.

SIMILAR PAST INCIDENTS:
{{.PastIncidents}}

RUNBOOKS:
{{.Runbooks}}

CURRENT DATA:
- Traces: {{.TracesSummary}}
- Metrics: {{.MetricsSummary}}
- Logs: {{.LogsSummary}}
- Recent Changes: {{.RecentChanges}}

PERFORM:
1. ROOT CAUSE ANALYSIS
   - What is the primary cause?
   - What is the chain of events?
   - Which component failed first?

2. IMPACT ASSESSMENT
   - Which services affected?
   - User impact estimation
   - Business impact

3. REMEDIATION
   - Immediate actions
   - Short-term fixes
   - Long-term prevention

OUTPUT FORMAT (JSON):
{
  "incident_summary": "brief description",
  "severity": "SEV1|SEV2|SEV3|SEV4",
  "root_cause": {
    "component": "which service/component",
    "failure_mode": "what failed",
    "trigger": "what triggered it",
    "contributing_factors": ["list of factors"]
  },
  "timeline": [
    {"time": "14:00", "event": "what happened"}
  ],
  "impact": {
    "affected_services": ["list"],
    "user_impact": "description",
    "estimated_users_affected": 1000,
    "business_impact": "revenue/reputation impact"
  },
  "remediation": {
    "immediate": ["actions to take now"],
    "short_term": ["actions for next 24h"],
    "long_term": ["preventive measures"]
  },
  "similar_incidents": [
    {
      "id": "incident ID",
      "similarity": 0.85,
      "resolution": "how it was resolved"
    }
  ],
  "recommendations": [
    {
      "priority": 1,
      "title": "recommendation title",
      "description": "what to do",
      "effort": "LOW|MEDIUM|HIGH",
      "runbook_link": "link if available"
    }
  ]
}`

	// NaturalLanguageQueryPrompt is the prompt for natural language queries
	NaturalLanguageQueryPrompt = `You are an expert observability assistant helping users understand their systems.

AVAILABLE DATA:
{{.AvailableData}}

KNOWLEDGE BASE:
{{.RAGContext}}

USER QUERY: {{.Query}}

Analyze the available data and knowledge base to answer the user's question.
Provide specific, actionable insights based on the data.

OUTPUT FORMAT (JSON):
{
  "answer": "Clear, concise answer to the user's question",
  "confidence": 0.85,
  "supporting_evidence": [
    {
      "source": "traces|metrics|logs|knowledge_base",
      "description": "what evidence supports this answer"
    }
  ],
  "related_insights": [
    "Additional relevant insight 1",
    "Additional relevant insight 2"
  ],
  "follow_up_questions": [
    "Suggested follow-up question 1",
    "Suggested follow-up question 2"
  ]
}`
)

// PromptTemplate represents a prompt template
type PromptTemplate struct {
	Name        string
	System      string
	UserFormat  string
	Description string
}

// DefaultPrompts returns the default prompt templates
func DefaultPrompts() map[string]PromptTemplate {
	return map[string]PromptTemplate{
		"trace": {
			Name:        "trace",
			System:      TraceAnalysisSystemPrompt,
			UserFormat:  "Analyze the following trace data:\n\n{{.TraceData}}",
			Description: "Analyze distributed traces for performance and reliability issues",
		},
		"metric": {
			Name:        "metric",
			System:      MetricAnalysisSystemPrompt,
			UserFormat:  "Analyze the following metric data:\n\n{{.MetricData}}",
			Description: "Analyze metrics for anomalies and capacity predictions",
		},
		"log": {
			Name:        "log",
			System:      LogAnalysisSystemPrompt,
			UserFormat:  "Analyze the following log data:\n\n{{.LogData}}",
			Description: "Analyze logs for errors, anomalies, and security concerns",
		},
		"incident": {
			Name:        "incident",
			System:      IncidentAnalysisSystemPrompt,
			UserFormat:  "Perform root cause analysis for this incident:\n\n{{.IncidentData}}",
			Description: "Perform comprehensive incident root cause analysis",
		},
		"query": {
			Name:        "query",
			System:      NaturalLanguageQueryPrompt,
			UserFormat:  "{{.Query}}",
			Description: "Answer natural language queries about observability data",
		},
	}
}
