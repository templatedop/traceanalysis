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
- **Configurable Features**: Enable/disable Temporal, RAG, caching, metrics, alerting
- **Graceful Shutdown**: Clean shutdown with connection draining

## Table of Contents

- [Quick Start](#quick-start)
- [Deployment](#deployment)
  - [Docker Compose](#docker-compose-deployment)
  - [Kubernetes with Helm](#kubernetes-deployment-helm)
  - [Kubernetes (Raw Manifests)](#kubernetes-deployment-raw-manifests)
- [Configuration](#configuration)
  - [Feature Flags](#feature-flags)
  - [Graceful Shutdown](#graceful-shutdown)
- [API Reference](#api-reference)
- [Knowledge Base](#knowledge-base)
- [Agent Capabilities](#agent-capabilities)
- [Development](#development)

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
make build

# Or build manually
go build -o bin/server ./cmd/server
go build -o bin/worker ./cmd/worker
go build -o bin/rag-cli ./cmd/cli
```

### Configuration

```bash
# Create configuration from example
make init-config
# Or: cp config.example.yaml config.yaml

# Edit configuration
vim config.yaml
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

## Deployment

### Docker Compose Deployment

The fastest way to get started with all dependencies:

```bash
# Start all services (Temporal, Qdrant, Ollama, Prometheus, Jaeger, Loki, Grafana)
make compose-up

# Or build from source and start
make compose-up-build

# Pull required LLM models
make compose-pull-models

# View logs
make compose-logs

# Check status
make compose-ps

# Stop all services
make compose-down

# Stop and remove volumes
make compose-down-volumes
```

**Services and Ports:**

| Service | Port | Description |
|---------|------|-------------|
| API Server | 8080 | Main API endpoint |
| Temporal UI | 8088 | Workflow management |
| Temporal | 7233 | Temporal gRPC |
| Grafana | 3000 | Dashboards (admin/admin) |
| Prometheus | 9090 | Metrics |
| Jaeger | 16686 | Trace UI |
| Loki | 3100 | Log aggregation |
| Qdrant | 6333 | Vector database |
| Ollama | 11434 | LLM API |

**Environment Variables:**

```bash
# Set version for builds
export VERSION=1.0.0
export COMMIT=$(git rev-parse --short HEAD)
export BUILD_TIME=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

# Start with version info
docker-compose up -d --build
```

### Kubernetes Deployment (Helm)

The recommended way to deploy to Kubernetes:

```bash
# Lint the chart
make helm-lint

# Preview rendered templates
make helm-template

# Install with defaults (Ollama)
make helm-install

# Or with custom values
helm install observability-analysis ./helm/observability-analysis \
  --namespace observability-analysis \
  --create-namespace \
  --set secrets.llmApiKey=sk-xxx \
  --set config.llm.provider=openai \
  --set config.llm.defaultModel=gpt-4

# Upgrade existing release
make helm-upgrade

# Check status
make helm-status

# Get current values
make helm-values

# Uninstall
make helm-uninstall
```

**Common Helm Values:**

```yaml
# values-production.yaml
server:
  replicaCount: 3
  autoscaling:
    enabled: true
    minReplicas: 3
    maxReplicas: 10

worker:
  replicaCount: 5
  autoscaling:
    enabled: true
    minReplicas: 5
    maxReplicas: 20

config:
  features:
    enableTemporal: true
    enableRAG: true
    enableCaching: true
    enableMetrics: true
    enableAlerting: true

  llm:
    provider: openai
    defaultModel: gpt-4

  server:
    shutdownTimeout: 30s
    drainTimeout: 10s

secrets:
  llmApiKey: "sk-xxx"

server:
  ingress:
    enabled: true
    className: nginx
    hosts:
      - host: observability.example.com
        paths:
          - path: /
            pathType: Prefix
    tls:
      - secretName: observability-tls
        hosts:
          - observability.example.com
```

```bash
# Install with custom values file
helm install observability-analysis ./helm/observability-analysis \
  -f values-production.yaml \
  --namespace production
```

### Kubernetes Deployment (Raw Manifests)

For simple deployments without Helm:

```bash
# Deploy
make deploy
# Or: kubectl apply -f deploy/kubernetes/deployment.yaml

# Undeploy
make undeploy

# Port forward for testing
make k8s-port-forward

# View logs
make k8s-logs-server
make k8s-logs-worker
```

### Docker Build

Build individual images:

```bash
# Build all images
make docker-build

# Build with custom registry
DOCKER_REGISTRY=myregistry.io make docker-build

# Push images
make docker-push

# Tag as latest
make docker-tag-latest
```

**Multi-stage build targets:**

```bash
# Build specific targets
docker build -t myapp-server --target server .
docker build -t myapp-worker --target worker .
docker build -t myapp-cli --target cli .
```

## Configuration

### Feature Flags

The framework supports configurable feature flags to enable/disable components:

```yaml
# config.yaml
features:
  # Enable Temporal workflow orchestration
  # When disabled, analysis runs synchronously
  enable_temporal: true

  # Enable RAG (Retrieval-Augmented Generation)
  # When disabled, LLM analysis runs without knowledge base
  enable_rag: true

  # Enable LLM response caching
  # Reduces costs and latency for repeated queries
  enable_caching: true

  # Enable Prometheus metrics for self-observability
  enable_metrics: true

  # Enable alert notifications (Slack, PagerDuty)
  enable_alerting: false

  # Enable predictive analysis and forecasting
  enable_predictions: true
```

**Agent Configuration:**

```yaml
agents:
  trace:
    enabled: true
    timeout: 2m
    max_retries: 3
    concurrency: 5
  metric:
    enabled: true
    timeout: 2m
  log:
    enabled: false  # Disable log agent
```

**Collector Configuration:**

```yaml
collectors:
  prometheus:
    enabled: true
    url: "http://prometheus:9090"
  jaeger:
    enabled: true
    query_url: "http://jaeger:16686"
  loki:
    enabled: false  # Disable Loki
```

### Graceful Shutdown

The framework handles graceful shutdown with configurable timeouts:

```yaml
server:
  host: "0.0.0.0"
  port: 8080
  read_timeout: 30s
  write_timeout: 30s
  idle_timeout: 120s
  # Graceful shutdown settings
  shutdown_timeout: 30s  # Max time for entire shutdown
  drain_timeout: 10s     # Time to wait for in-flight requests
```

**Shutdown Behavior:**

1. Receives SIGTERM/SIGINT signal
2. Stops accepting new requests (returns 503)
3. Health/Ready endpoints return "shutting_down" status
4. Waits for drain_timeout to complete in-flight requests
5. Closes HTTP server
6. Closes framework components (Temporal, RAG, cache, etc.)
7. Exits cleanly

**Kubernetes Integration:**

```yaml
# Deployment should match shutdown timeout
spec:
  terminationGracePeriodSeconds: 45  # > shutdown_timeout + drain_timeout
```

## API Reference

### Health Endpoints

```bash
# Health check (shows component status)
curl http://localhost:8080/health

# Readiness check (for load balancers)
curl http://localhost:8080/ready

# List enabled features
curl http://localhost:8080/api/v1/features
```

### Analysis Endpoints

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

# Incident analysis (requires Temporal)
curl -X POST http://localhost:8080/api/v1/analyze/incident \
  -H "Content-Type: application/json" \
  -d '{
    "title": "Payment Service Down",
    "description": "Users unable to complete checkout",
    "services": ["payment", "db"],
    "symptoms": ["500 errors", "timeout"]
  }'
```

### RAG Endpoints (when enabled)

```bash
# Query knowledge base
curl -X POST http://localhost:8080/api/v1/knowledge/query \
  -H "Content-Type: application/json" \
  -d '{"query": "database connection issues", "top_k": 5}'

# Add document
curl -X POST http://localhost:8080/api/v1/knowledge/add \
  -H "Content-Type: application/json" \
  -d '{
    "type": "runbook",
    "title": "High CPU Runbook",
    "content": "Steps to diagnose high CPU..."
  }'

# Get stats
curl http://localhost:8080/api/v1/knowledge/stats
```

### Monitoring Endpoints (requires Temporal)

```bash
# Start continuous monitoring
curl -X POST http://localhost:8080/api/v1/monitor/start \
  -H "Content-Type: application/json" \
  -d '{"services": ["api"], "interval_minutes": 5}'

# Stop monitoring
curl -X POST "http://localhost:8080/api/v1/monitor/stop?workflow_id=xxx"

# Check workflow status
curl "http://localhost:8080/api/v1/analyze/status?workflow_id=xxx"
```

## CLI Usage

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

## Knowledge Base

The RAG system uses a knowledge base to provide context for analysis:

### Runbooks

```markdown
# knowledge/runbooks/high-cpu.md

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
# knowledge/incidents/api-outage-2024-01.md

## Summary
API outage due to database connection pool exhaustion

## Root Cause
Slow query from new deployment

## Resolution
Rollback and add index
```

Load knowledge base:

```bash
# Via CLI
./bin/rag-cli knowledge add -type=runbook -title="High CPU" -file=runbooks/high-cpu.md

# Via API
curl -X POST http://localhost:8080/api/v1/knowledge/add \
  -H "Content-Type: application/json" \
  -d '{"type": "runbook", "title": "High CPU", "content": "..."}'
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
│   ├── framework/       # Framework manager with feature flags
│   ├── llm/             # LLM client and prompts
│   ├── rag/             # RAG system (store, embedding)
│   └── workflow/        # Temporal workflows
├── internal/
│   ├── config/          # Configuration with feature flags
│   └── types/           # Core types
├── helm/
│   └── observability-analysis/  # Helm chart
├── deploy/
│   ├── kubernetes/      # Raw Kubernetes manifests
│   ├── prometheus/      # Prometheus configuration
│   ├── grafana/         # Grafana dashboards
│   └── temporal/        # Temporal configuration
├── Dockerfile           # Multi-stage Docker build
├── docker-compose.yaml  # Full stack deployment
├── Makefile             # Build and deployment commands
├── config.example.yaml  # Example configuration
└── README.md
```

## Development

```bash
# Install dev dependencies
make dev-deps

# Run tests
make test

# Run with coverage
make test-coverage

# Run benchmarks
make bench

# Lint code
make lint

# Format code
make fmt

# Tidy dependencies
make tidy
```

## Model Recommendations

| Use Case | Recommended | Alternative |
|----------|------------|-------------|
| General Analysis | Qwen2.5-32B | DeepSeek-V2.5 |
| Log Classification | Mistral-7B | Qwen2.5-7B |
| Complex RCA | Qwen2.5-72B | GPT-4 |

## Make Targets

Run `make help` to see all available targets:

```
Build:
  make build            - Build all binaries
  make clean            - Clean build artifacts
  make version          - Show version info

Docker Compose:
  make compose-up       - Start all services
  make compose-down     - Stop all services
  make compose-logs     - Follow logs
  make compose-pull-models - Pull required LLM models

Helm:
  make helm-install     - Install Helm release
  make helm-upgrade     - Upgrade Helm release
  make helm-uninstall   - Uninstall Helm release

Kubernetes:
  make deploy           - Deploy (raw manifests)
  make k8s-port-forward - Port forward to service

API:
  make health           - Check health endpoint
  make features         - List enabled features
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
