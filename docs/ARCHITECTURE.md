# Architecture Documentation

## Overview

This framework provides an LLM-enhanced observability analysis platform using RAG (Retrieval-Augmented Generation) and Temporal workflows for orchestrating independent analysis agents. It enables natural language queries over traces, metrics, and logs while providing predictive analysis and automated root cause identification.

## Architecture Diagram

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              CLIENT LAYER                                    │
│                                                                             │
│    ┌──────────┐     ┌──────────┐     ┌──────────┐     ┌──────────┐        │
│    │   CLI    │     │   API    │     │  Webhook │     │ Grafana  │        │
│    │  Client  │     │  Client  │     │  Client  │     │  Plugin  │        │
│    └────┬─────┘     └────┬─────┘     └────┬─────┘     └────┬─────┘        │
└─────────┼────────────────┼────────────────┼────────────────┼───────────────┘
          │                │                │                │
          └────────────────┴────────────────┴────────────────┘
                                   │
                                   ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                              API SERVER                                      │
│                                                                             │
│    ┌────────────────────────────────────────────────────────────────────┐  │
│    │                         HTTP Handlers                               │  │
│    │                                                                     │  │
│    │  /api/v1/analyze    /api/v1/query    /api/v1/knowledge             │  │
│    │  /api/v1/incident   /api/v1/monitor  /api/v1/health                │  │
│    └────────────────────────────────┬───────────────────────────────────┘  │
│                                     │                                       │
└─────────────────────────────────────┼───────────────────────────────────────┘
                                      │
                                      ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                         TEMPORAL WORKFLOW ENGINE                            │
│                                                                             │
│    ┌────────────────────────────────────────────────────────────────────┐  │
│    │                     Workflow Orchestration                          │  │
│    │                                                                     │  │
│    │  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐    │  │
│    │  │  Analysis       │  │  Continuous     │  │  Incident       │    │  │
│    │  │  Workflow       │  │  Monitoring     │  │  Analysis       │    │  │
│    │  │                 │  │  Workflow       │  │  Workflow       │    │  │
│    │  └────────┬────────┘  └────────┬────────┘  └────────┬────────┘    │  │
│    │           │                    │                    │              │  │
│    └───────────┼────────────────────┼────────────────────┼──────────────┘  │
│                │                    │                    │                  │
│    ┌───────────┴────────────────────┴────────────────────┴──────────────┐  │
│    │                          Activities                                 │  │
│    │                                                                     │  │
│    │  FetchTraces  FetchMetrics  FetchLogs  FetchRAG  LLMAnalysis      │  │
│    └────────────────────────────────┬───────────────────────────────────┘  │
│                                     │                                       │
└─────────────────────────────────────┼───────────────────────────────────────┘
                                      │
          ┌───────────────────────────┼───────────────────────────┐
          │                           │                           │
          ▼                           ▼                           ▼
┌─────────────────────┐  ┌─────────────────────┐  ┌─────────────────────┐
│   TRACE AGENT       │  │   METRIC AGENT      │  │   LOG AGENT         │
│                     │  │                     │  │                     │
│ ┌─────────────────┐ │  │ ┌─────────────────┐ │  │ ┌─────────────────┐ │
│ │ Jaeger/Tempo    │ │  │ │ Prometheus      │ │  │ │ Loki            │ │
│ │ Collector       │ │  │ │ Collector       │ │  │ │ Collector       │ │
│ └────────┬────────┘ │  │ └────────┬────────┘ │  │ └────────┬────────┘ │
│          │          │  │          │          │  │          │          │
│ ┌────────▼────────┐ │  │ ┌────────▼────────┐ │  │ ┌────────▼────────┐ │
│ │ Statistical     │ │  │ │ Anomaly         │ │  │ │ Log Parser      │ │
│ │ Analysis        │ │  │ │ Detection       │ │  │ │ (Template)      │ │
│ │ - Latency       │ │  │ │ - Z-score       │ │  │ │                 │ │
│ │ - Bottlenecks   │ │  │ │ - Forecasting   │ │  │ │ Error Grouping  │ │
│ │ - Critical Path │ │  │ │ - Correlation   │ │  │ │ Pattern Detect  │ │
│ └────────┬────────┘ │  │ └────────┬────────┘ │  │ └────────┬────────┘ │
│          │          │  │          │          │  │          │          │
│ ┌────────▼────────┐ │  │ ┌────────▼────────┐ │  │ ┌────────▼────────┐ │
│ │ LLM Analysis    │ │  │ │ LLM Analysis    │ │  │ │ LLM Analysis    │ │
│ │ (Trace Prompt)  │ │  │ │ (Metric Prompt) │ │  │ │ (Log Prompt)    │ │
│ └─────────────────┘ │  │ └─────────────────┘ │  │ └─────────────────┘ │
└─────────────────────┘  └─────────────────────┘  └─────────────────────┘
          │                           │                           │
          └───────────────────────────┼───────────────────────────┘
                                      │
                                      ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                            LLM & RAG LAYER                                  │
│                                                                             │
│    ┌────────────────────────────────────────────────────────────────────┐  │
│    │                        LLM Client                                   │  │
│    │                                                                     │  │
│    │  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐           │  │
│    │  │ Ollama   │  │ OpenAI   │  │Anthropic │  │ DeepSeek │           │  │
│    │  │ (Qwen,   │  │ (GPT-4)  │  │ (Claude) │  │          │           │  │
│    │  │ Mistral) │  │          │  │          │  │          │           │  │
│    │  └──────────┘  └──────────┘  └──────────┘  └──────────┘           │  │
│    └────────────────────────────────────────────────────────────────────┘  │
│                                                                             │
│    ┌────────────────────────────────────────────────────────────────────┐  │
│    │                      RAG Knowledge Base                             │  │
│    │                                                                     │  │
│    │  ┌──────────────────────────────────────────────────────────────┐  │  │
│    │  │                    Vector Store                               │  │  │
│    │  │  (Memory / Qdrant / Weaviate / Pinecone / PgVector)          │  │  │
│    │  └──────────────────────────────────────────────────────────────┘  │  │
│    │                                                                     │  │
│    │  Documents:                                                         │  │
│    │  ┌───────────┐  ┌───────────┐  ┌───────────┐  ┌───────────┐       │  │
│    │  │ Runbooks  │  │ Incidents │  │Architecture│  │ Patterns  │       │  │
│    │  └───────────┘  └───────────┘  └───────────┘  └───────────┘       │  │
│    └────────────────────────────────────────────────────────────────────┘  │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
                                      │
                                      ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                           DATA SOURCES                                       │
│                                                                             │
│    ┌───────────┐   ┌───────────┐   ┌───────────┐   ┌───────────┐          │
│    │  Jaeger/  │   │Prometheus/│   │  Loki/    │   │   K8s     │          │
│    │  Tempo    │   │VictoriaM  │   │   ELK     │   │  Events   │          │
│    │  (Traces) │   │ (Metrics) │   │  (Logs)   │   │           │          │
│    └───────────┘   └───────────┘   └───────────┘   └───────────┘          │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

## Core Components

### 1. API Server (`cmd/server`)

The HTTP API server handles incoming requests and coordinates with the Temporal workflow engine.

**Key Responsibilities:**
- Request validation and routing
- Workflow initiation
- Result retrieval
- Health checking

**Endpoints:**
| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/v1/analyze` | POST | Start observability analysis |
| `/api/v1/analyze/incident` | POST | Analyze specific incident |
| `/api/v1/analyze/status` | GET | Get workflow status |
| `/api/v1/analyze/result` | GET | Get analysis result |
| `/api/v1/monitor/start` | POST | Start continuous monitoring |
| `/api/v1/monitor/stop` | POST | Stop monitoring |
| `/api/v1/query` | POST | Natural language query |
| `/api/v1/knowledge/query` | POST | Search knowledge base |
| `/api/v1/knowledge/add` | POST | Add document to knowledge base |

### 2. Temporal Worker (`cmd/worker`)

Executes workflow activities and maintains workflow state.

**Workflows:**
- **ObservabilityAnalysisWorkflow**: Main analysis workflow that orchestrates all agents
- **ContinuousMonitoringWorkflow**: Long-running workflow for continuous monitoring
- **IncidentAnalysisWorkflow**: Deep-dive incident analysis

**Activities:**
- `FetchTracesActivity`: Collects and analyzes traces
- `FetchMetricsActivity`: Collects and analyzes metrics
- `FetchLogsActivity`: Collects and analyzes logs
- `FetchRAGContextActivity`: Retrieves relevant knowledge
- `LLMAnalysisActivity`: Performs comprehensive LLM analysis
- `NotifyFindingsActivity`: Sends notifications

### 3. Analysis Agents (`pkg/agents`)

Independent agents that specialize in specific observability data types.

#### Trace Agent
- Collects traces from Jaeger/Tempo
- Calculates latency statistics (P50, P95, P99)
- Identifies bottlenecks and critical paths
- Builds service dependency graphs
- Uses LLM for root cause analysis

#### Metric Agent
- Queries Prometheus/VictoriaMetrics
- Detects anomalies using statistical methods (Z-score, IQR)
- Generates forecasts and predictions
- Finds correlations between metrics
- Uses LLM for capacity planning insights

#### Log Agent
- Collects logs from Loki/Elasticsearch
- Parses logs into templates (Drain algorithm)
- Groups similar errors
- Detects log anomalies (frequency spikes, new patterns)
- Uses LLM for error analysis and root cause

### 4. RAG System (`pkg/rag`)

Retrieval-Augmented Generation system for contextual knowledge.

**Components:**
- **Store**: Vector database interface (Memory, Qdrant, Weaviate, etc.)
- **Embedder**: Generates embeddings (Ollama, OpenAI)
- **KnowledgeBase**: Document management and chunking

**Document Types:**
- Runbooks: Troubleshooting procedures
- Incidents: Past incident post-mortems
- Architecture: System design documentation
- Patterns: Known issue patterns

### 5. LLM Client (`pkg/llm`)

Multi-provider LLM client with specialized prompts.

**Supported Providers:**
- Ollama (Qwen, Mistral, Llama)
- OpenAI (GPT-4, GPT-4-Turbo)
- Anthropic (Claude)
- DeepSeek

**Prompt Templates:**
- `TraceAnalysisSystemPrompt`: For trace analysis
- `MetricAnalysisSystemPrompt`: For metric analysis
- `LogAnalysisSystemPrompt`: For log analysis
- `IncidentAnalysisSystemPrompt`: For incident RCA
- `NaturalLanguageQueryPrompt`: For general queries

### 6. Data Collectors (`pkg/collectors`)

Integrations with observability backends.

- **PrometheusCollector**: Queries Prometheus/VictoriaMetrics
- **JaegerCollector**: Fetches traces from Jaeger
- **LokiCollector**: Queries logs from Loki

## Data Flow

### Analysis Request Flow

```
1. Client sends POST /api/v1/analyze
   │
2. API Server validates request and starts workflow
   │
3. Temporal starts ObservabilityAnalysisWorkflow
   │
4. Workflow executes activities in parallel:
   │
   ├─► FetchTracesActivity ─► TraceAgent.Analyze()
   │                              │
   │                              ├─► Fetch traces from Jaeger
   │                              ├─► Calculate statistics
   │                              ├─► Find bottlenecks
   │                              └─► LLM analysis with prompts
   │
   ├─► FetchMetricsActivity ─► MetricAgent.Analyze()
   │                              │
   │                              ├─► Query Prometheus
   │                              ├─► Detect anomalies
   │                              ├─► Generate predictions
   │                              └─► LLM analysis
   │
   └─► FetchLogsActivity ─► LogAgent.Analyze()
                               │
                               ├─► Query Loki
                               ├─► Parse log templates
                               ├─► Group errors
                               └─► LLM analysis
   │
5. Workflow fetches RAG context
   │
6. Workflow performs comprehensive LLM analysis
   │
7. Workflow compiles results and returns to client
```

### Continuous Monitoring Flow

```
1. Client starts monitoring workflow
   │
2. ContinuousMonitoringWorkflow starts
   │
3. Loop:
   │
   ├─► Execute ObservabilityAnalysisWorkflow
   │
   ├─► Check for critical findings
   │
   ├─► Send notifications if needed
   │
   └─► Sleep for interval
   │
4. Continue until stopped or max iterations
```

## Model Recommendations

| Use Case | Recommended Model | Alternative |
|----------|------------------|-------------|
| Trace Analysis | Qwen2.5-32B | DeepSeek-V2.5 |
| Metric Analysis | Qwen2.5-32B | Llama-3.1-70B |
| Log Analysis | Mistral-7B | Qwen2.5-7B |
| Incident RCA | Qwen2.5-72B | GPT-4 |
| General Queries | Qwen2.5-32B | Claude-3-Sonnet |

## Scaling Considerations

### Horizontal Scaling
- API servers can be scaled behind a load balancer
- Temporal workers can be scaled independently
- Each worker handles multiple activities concurrently

### Performance Optimization
- Connection pooling for data collectors
- Caching for RAG embeddings
- Batched LLM requests where possible
- Async processing for non-blocking operations

### High Availability
- Temporal provides workflow state persistence
- Multi-node Temporal cluster for HA
- Vector store replication for RAG

## Security

### Authentication
- API key authentication for API endpoints
- mTLS for inter-service communication
- Temporal namespace isolation

### Authorization
- Role-based access control for knowledge base
- Audit logging for all operations

### Data Protection
- Sensitive data masking in traces/logs
- Encryption at rest and in transit
- Configurable data retention

## Monitoring

### Metrics to Track
- Workflow execution time
- Agent analysis duration
- LLM token usage
- RAG query latency
- Error rates by component

### Alerting
- Workflow failures
- High LLM latency
- Data source connectivity issues
- Knowledge base sync failures
