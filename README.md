# Observability Analysis Framework

A comprehensive RAG (Retrieval-Augmented Generation) framework in Golang using Temporal workflows for orchestrating independent analysis agents. Enables LLM-enhanced observability analysis for traces, metrics, and logs with predictive capabilities and automated root cause identification.

## Features

- **Multi-Agent Architecture**: Independent agents for trace, metric, and log analysis
- **Temporal Workflows**: Reliable orchestration with state management and retry logic
- **RAG Integration**: Knowledge base with runbooks, past incidents, and architecture docs
- **Multi-LLM Support**: Ollama, OpenAI, Anthropic, DeepSeek
- **Data Source Integration**: Prometheus, Jaeger, Loki, OpenTelemetry
- **Natural Language Queries**: Ask questions in plain English
- **Predictive Analysis**: Capacity forecasting and anomaly detection
- **Continuous Monitoring**: Automated analysis at configurable intervals

## Quick Start

### Prerequisites

- Go 1.21+
- Temporal Server (or Temporal Cloud)
- Ollama with Qwen2.5 (or other LLM provider)
- Prometheus, Jaeger, Loki (or your observability stack)

### Installation

```bash
# Clone the repository
git clone https://github.com/traceanalysis/rag-temporal.git
cd rag-temporal

# Install dependencies
go mod tidy

# Build binaries
go build -o bin/server ./cmd/server
go build -o bin/worker ./cmd/worker
go build -o bin/rag-cli ./cmd/cli
```

### Configuration

```bash
# Create configuration from example
cp config.example.yaml config.yaml

# Edit configuration
vim config.yaml
```

Key configuration options:

```yaml
llm:
  provider: "ollama"
  default_model: "qwen2.5:32b"
  base_url: "http://localhost:11434"

temporal:
  host_port: "localhost:7233"
  task_queue: "observability-analysis"

collectors:
  prometheus:
    enabled: true
    url: "http://localhost:9090"
  jaeger:
    enabled: true
    query_url: "http://localhost:16686"
  loki:
    enabled: true
    url: "http://localhost:3100"
```

### Starting Services

```bash
# Start Temporal (if not already running)
temporal server start-dev

# Start the worker
./bin/worker --config config.yaml

# Start the API server
./bin/server --config config.yaml
```

### Using the CLI

```bash
# Run analysis
./bin/rag-cli analyze "Why is the checkout service slow?"

# Analyze with specific services and time range
./bin/rag-cli analyze -services=api,db -time=2h "Investigate high error rate"

# Incident analysis
./bin/rag-cli incident -title="API Outage" -desc="Users reporting 500 errors" -services=api

# Natural language query
./bin/rag-cli query "What are common causes of database connection issues?"

# Start continuous monitoring
./bin/rag-cli monitor start -services=api,db -interval=5m

# Add knowledge to RAG
./bin/rag-cli knowledge add -type=runbook -title="High CPU" -file=runbook.md

# Check health
./bin/rag-cli health
```

### Using the API

```bash
# Run analysis
curl -X POST http://localhost:8080/api/v1/analyze \
  -H "Content-Type: application/json" \
  -d '{
    "query": "Why is checkout slow?",
    "services": ["api", "payment"],
    "time_range_minutes": 60,
    "include_rag": true
  }'

# Natural language query
curl -X POST http://localhost:8080/api/v1/query \
  -H "Content-Type: application/json" \
  -d '{"query": "What could cause memory leaks?"}'

# Incident analysis
curl -X POST http://localhost:8080/api/v1/analyze/incident \
  -H "Content-Type: application/json" \
  -d '{
    "title": "Payment Service Down",
    "description": "Users unable to complete checkout",
    "services": ["payment", "db"],
    "symptoms": ["500 errors", "timeout"]
  }'
```

## Project Structure

```
.
├── cmd/
│   ├── server/          # API server
│   ├── worker/          # Temporal worker
│   └── cli/             # Command-line interface
├── pkg/
│   ├── agents/          # Analysis agents (trace, metric, log)
│   ├── collectors/      # Data source collectors
│   ├── llm/             # LLM client and prompts
│   ├── rag/             # RAG system (store, embedding)
│   └── workflow/        # Temporal workflows
├── internal/
│   ├── config/          # Configuration
│   └── types/           # Core types
├── examples/
│   ├── runbooks/        # Example runbooks
│   └── incidents/       # Example incidents
├── docs/
│   └── ARCHITECTURE.md  # Architecture documentation
├── config.example.yaml
└── README.md
```

## Knowledge Base

The RAG system uses a knowledge base to provide context for analysis. Populate it with:

### Runbooks
```markdown
# examples/runbooks/high-cpu.md

## Symptoms
- CPU usage above 80%
- Increased latency

## Investigation Steps
1. Check for recent deployments
2. Profile the application
3. Check for GC pressure
```

### Past Incidents
```markdown
# examples/incidents/api-outage-2024-01.md

## Summary
API outage due to database connection pool exhaustion

## Root Cause
Slow query from new deployment

## Resolution
Rollback and add index
```

Load knowledge base:
```bash
# The worker loads from the configured path
./bin/worker --config config.yaml

# Or add documents via CLI
./bin/rag-cli knowledge add -type=runbook -title="High CPU" -file=runbooks/high-cpu.md
```

## Agent Capabilities

### Trace Agent
- Fetches traces from Jaeger/Tempo
- Calculates P50/P95/P99 latencies
- Identifies bottlenecks and critical paths
- Builds service dependency graphs
- LLM analysis for root cause

### Metric Agent
- Queries Prometheus/VictoriaMetrics
- Anomaly detection (Z-score, IQR)
- Time series forecasting
- Correlation analysis
- Capacity predictions

### Log Agent
- Queries Loki/Elasticsearch
- Log template extraction (Drain algorithm)
- Error clustering and grouping
- Anomaly detection (frequency, new patterns)
- Security concern detection

## Workflows

### ObservabilityAnalysisWorkflow
Main workflow that orchestrates all agents in parallel:
1. Fetch data from all sources (parallel)
2. Run statistical analysis
3. Get RAG context
4. Perform LLM analysis
5. Compile and return results

### ContinuousMonitoringWorkflow
Long-running workflow for continuous analysis:
1. Execute analysis at intervals
2. Check for critical findings
3. Send notifications
4. Repeat

### IncidentAnalysisWorkflow
Deep-dive incident investigation:
1. Extended time range analysis
2. Correlation across all data sources
3. Similar incident retrieval from RAG
4. Detailed root cause analysis

## Configuration Reference

See [config.example.yaml](config.example.yaml) for full configuration options.

| Section | Key Options |
|---------|-------------|
| `server` | Host, port, TLS settings |
| `temporal` | Host, namespace, task queue |
| `llm` | Provider, model, temperature |
| `rag` | Vector store, embedding model |
| `collectors` | Prometheus, Jaeger, Loki URLs |
| `analysis` | Time ranges, thresholds |
| `alerting` | Slack, PagerDuty webhooks |

## Model Recommendations

| Use Case | Recommended | Alternative |
|----------|------------|-------------|
| General Analysis | Qwen2.5-32B | DeepSeek-V2.5 |
| Log Classification | Mistral-7B | Qwen2.5-7B |
| Complex RCA | Qwen2.5-72B | GPT-4 |

## Development

```bash
# Run tests
go test ./...

# Run with race detector
go test -race ./...

# Build for production
CGO_ENABLED=0 go build -o bin/server ./cmd/server
CGO_ENABLED=0 go build -o bin/worker ./cmd/worker
CGO_ENABLED=0 go build -o bin/rag-cli ./cmd/cli
```

## Docker Deployment

```dockerfile
# Dockerfile
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o server ./cmd/server
RUN go build -o worker ./cmd/worker

FROM alpine:latest
COPY --from=builder /app/server /app/worker /usr/local/bin/
CMD ["server"]
```

```yaml
# docker-compose.yaml
version: '3.8'
services:
  temporal:
    image: temporalio/auto-setup:latest
    ports:
      - "7233:7233"

  worker:
    build: .
    command: worker --config /config/config.yaml
    volumes:
      - ./config.yaml:/config/config.yaml
      - ./knowledge:/knowledge
    depends_on:
      - temporal

  server:
    build: .
    command: server --config /config/config.yaml
    ports:
      - "8080:8080"
    depends_on:
      - temporal
```

## Contributing

Contributions are welcome! Please read our contributing guidelines before submitting PRs.

## License

MIT License - see LICENSE file for details.

## References

This framework draws inspiration from:
- [K8sGPT](https://github.com/k8sgpt-ai/k8sgpt) - Kubernetes AI diagnostics
- [LogAI](https://github.com/salesforce/logai) - Log analysis
- [OpenLIT](https://github.com/openlit/openlit) - LLM observability
- [Robusta](https://github.com/robusta-dev/robusta) - K8s troubleshooting
- [Coroot](https://github.com/coroot/coroot) - Observability platform
- [SigNoz](https://github.com/SigNoz/signoz) - OpenTelemetry native observability
