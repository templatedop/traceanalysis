.PHONY: all build test clean run-server run-worker docker-build docker-push deploy help

# Variables
BINARY_DIR := bin
SERVER_BINARY := $(BINARY_DIR)/server
WORKER_BINARY := $(BINARY_DIR)/worker
CLI_BINARY := $(BINARY_DIR)/rag-cli

GO := go
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS := -ldflags="-w -s -X main.Version=$(VERSION) -X main.Commit=$(COMMIT) -X main.BuildTime=$(BUILD_TIME)"

DOCKER_REGISTRY ?= ghcr.io
DOCKER_IMAGE := $(DOCKER_REGISTRY)/observability-analysis
DOCKER_TAG ?= $(VERSION)

HELM_RELEASE ?= observability-analysis
HELM_NAMESPACE ?= observability-analysis
HELM_CHART := ./helm/observability-analysis

# Default target
all: build

# Build all binaries
build: $(SERVER_BINARY) $(WORKER_BINARY) $(CLI_BINARY)

$(BINARY_DIR):
	mkdir -p $(BINARY_DIR)

$(SERVER_BINARY): $(BINARY_DIR)
	$(GO) build $(LDFLAGS) -o $@ ./cmd/server

$(WORKER_BINARY): $(BINARY_DIR)
	$(GO) build $(LDFLAGS) -o $@ ./cmd/worker

$(CLI_BINARY): $(BINARY_DIR)
	$(GO) build $(LDFLAGS) -o $@ ./cmd/cli

# Run tests
test:
	$(GO) test -v -race -coverprofile=coverage.out ./...

test-coverage: test
	$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

# Benchmarks
bench:
	$(GO) test -bench=. -benchmem ./...

# Lint
lint:
	golangci-lint run ./...

# Format code
fmt:
	$(GO) fmt ./...
	gofumpt -w .

# Tidy dependencies
tidy:
	$(GO) mod tidy

# Clean build artifacts
clean:
	rm -rf $(BINARY_DIR)
	rm -f coverage.out coverage.html

# Run server locally
run-server: $(SERVER_BINARY)
	$(SERVER_BINARY) --config config.yaml

# Run worker locally
run-worker: $(WORKER_BINARY)
	$(WORKER_BINARY) --config config.yaml

# ============================================================================
# Docker commands
# ============================================================================

docker-build:
	docker build -t $(DOCKER_IMAGE):$(DOCKER_TAG) \
		--build-arg VERSION=$(VERSION) \
		--build-arg COMMIT=$(COMMIT) \
		--build-arg BUILD_TIME=$(BUILD_TIME) .
	docker build -t $(DOCKER_IMAGE)-server:$(DOCKER_TAG) --target server \
		--build-arg VERSION=$(VERSION) \
		--build-arg COMMIT=$(COMMIT) \
		--build-arg BUILD_TIME=$(BUILD_TIME) .
	docker build -t $(DOCKER_IMAGE)-worker:$(DOCKER_TAG) --target worker \
		--build-arg VERSION=$(VERSION) \
		--build-arg COMMIT=$(COMMIT) \
		--build-arg BUILD_TIME=$(BUILD_TIME) .
	docker build -t $(DOCKER_IMAGE)-cli:$(DOCKER_TAG) --target cli .

docker-push:
	docker push $(DOCKER_IMAGE):$(DOCKER_TAG)
	docker push $(DOCKER_IMAGE)-server:$(DOCKER_TAG)
	docker push $(DOCKER_IMAGE)-worker:$(DOCKER_TAG)
	docker push $(DOCKER_IMAGE)-cli:$(DOCKER_TAG)

docker-tag-latest:
	docker tag $(DOCKER_IMAGE):$(DOCKER_TAG) $(DOCKER_IMAGE):latest
	docker tag $(DOCKER_IMAGE)-server:$(DOCKER_TAG) $(DOCKER_IMAGE)-server:latest
	docker tag $(DOCKER_IMAGE)-worker:$(DOCKER_TAG) $(DOCKER_IMAGE)-worker:latest
	docker tag $(DOCKER_IMAGE)-cli:$(DOCKER_TAG) $(DOCKER_IMAGE)-cli:latest

# ============================================================================
# Docker Compose commands
# ============================================================================

compose-up:
	docker-compose up -d

compose-up-build:
	docker-compose up -d --build

compose-down:
	docker-compose down

compose-down-volumes:
	docker-compose down -v

compose-logs:
	docker-compose logs -f

compose-logs-server:
	docker-compose logs -f server

compose-logs-worker:
	docker-compose logs -f worker

compose-build:
	docker-compose build

compose-ps:
	docker-compose ps

compose-restart:
	docker-compose restart server worker

compose-pull-models:
	docker-compose exec ollama ollama pull qwen2.5:32b
	docker-compose exec ollama ollama pull nomic-embed-text

# ============================================================================
# Helm commands
# ============================================================================

helm-lint:
	helm lint $(HELM_CHART)

helm-template:
	helm template $(HELM_RELEASE) $(HELM_CHART) --namespace $(HELM_NAMESPACE)

helm-install:
	helm install $(HELM_RELEASE) $(HELM_CHART) \
		--namespace $(HELM_NAMESPACE) \
		--create-namespace

helm-install-dry-run:
	helm install $(HELM_RELEASE) $(HELM_CHART) \
		--namespace $(HELM_NAMESPACE) \
		--create-namespace \
		--dry-run

helm-upgrade:
	helm upgrade $(HELM_RELEASE) $(HELM_CHART) \
		--namespace $(HELM_NAMESPACE) \
		--install

helm-uninstall:
	helm uninstall $(HELM_RELEASE) --namespace $(HELM_NAMESPACE)

helm-status:
	helm status $(HELM_RELEASE) --namespace $(HELM_NAMESPACE)

helm-values:
	helm get values $(HELM_RELEASE) --namespace $(HELM_NAMESPACE)

helm-package:
	helm package $(HELM_CHART) -d ./dist

# ============================================================================
# Kubernetes deployment (raw manifests)
# ============================================================================

deploy:
	kubectl apply -f deploy/kubernetes/deployment.yaml

undeploy:
	kubectl delete -f deploy/kubernetes/deployment.yaml

k8s-logs-server:
	kubectl logs -l app.kubernetes.io/component=server -f --namespace $(HELM_NAMESPACE)

k8s-logs-worker:
	kubectl logs -l app.kubernetes.io/component=worker -f --namespace $(HELM_NAMESPACE)

k8s-port-forward:
	kubectl port-forward svc/$(HELM_RELEASE) 8080:80 --namespace $(HELM_NAMESPACE)

# ============================================================================
# Development setup
# ============================================================================

dev-deps:
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install mvdan.cc/gofumpt@latest

# Initialize config
init-config:
	cp config.example.yaml config.yaml
	@echo "Config file created. Edit config.yaml with your settings."

# Load example knowledge base
load-examples:
	mkdir -p knowledge/runbooks knowledge/incidents
	cp -r examples/runbooks/* knowledge/runbooks/ 2>/dev/null || true
	cp -r examples/incidents/* knowledge/incidents/ 2>/dev/null || true
	@echo "Example knowledge base loaded."

# ============================================================================
# API interaction
# ============================================================================

# Health check
health:
	@curl -s http://localhost:8080/health | jq .

ready:
	@curl -s http://localhost:8080/ready | jq .

features:
	@curl -s http://localhost:8080/api/v1/features | jq .

# Run analysis example
example-analyze:
	curl -X POST http://localhost:8080/api/v1/analyze \
		-H "Content-Type: application/json" \
		-d '{"query": "Why is the API slow?", "time_range_minutes": 60, "include_rag": true}' | jq .

example-query:
	curl -X POST http://localhost:8080/api/v1/query \
		-H "Content-Type: application/json" \
		-d '{"query": "What could cause database connection issues?"}' | jq .

# ============================================================================
# Version info
# ============================================================================

version:
	@echo "Version: $(VERSION)"
	@echo "Commit:  $(COMMIT)"
	@echo "Built:   $(BUILD_TIME)"

# ============================================================================
# Help
# ============================================================================

help:
	@echo "Observability Analysis Framework - Make targets"
	@echo ""
	@echo "Build:"
	@echo "  make build            - Build all binaries"
	@echo "  make clean            - Clean build artifacts"
	@echo "  make version          - Show version info"
	@echo ""
	@echo "Test:"
	@echo "  make test             - Run tests with coverage"
	@echo "  make test-coverage    - Generate HTML coverage report"
	@echo "  make bench            - Run benchmarks"
	@echo "  make lint             - Run linter"
	@echo ""
	@echo "Run:"
	@echo "  make run-server       - Run API server"
	@echo "  make run-worker       - Run Temporal worker"
	@echo ""
	@echo "Docker:"
	@echo "  make docker-build     - Build Docker images"
	@echo "  make docker-push      - Push Docker images"
	@echo ""
	@echo "Docker Compose:"
	@echo "  make compose-up       - Start all services"
	@echo "  make compose-up-build - Build and start services"
	@echo "  make compose-down     - Stop all services"
	@echo "  make compose-logs     - Follow logs"
	@echo "  make compose-ps       - Show running services"
	@echo "  make compose-pull-models - Pull required LLM models"
	@echo ""
	@echo "Helm:"
	@echo "  make helm-lint        - Lint Helm chart"
	@echo "  make helm-template    - Render templates locally"
	@echo "  make helm-install     - Install Helm release"
	@echo "  make helm-upgrade     - Upgrade Helm release"
	@echo "  make helm-uninstall   - Uninstall Helm release"
	@echo "  make helm-package     - Package Helm chart"
	@echo ""
	@echo "Kubernetes:"
	@echo "  make deploy           - Deploy to Kubernetes (raw manifests)"
	@echo "  make undeploy         - Remove from Kubernetes"
	@echo "  make k8s-port-forward - Port forward to service"
	@echo ""
	@echo "Development:"
	@echo "  make dev-deps         - Install dev dependencies"
	@echo "  make init-config      - Create config file"
	@echo "  make load-examples    - Load example knowledge base"
	@echo "  make fmt              - Format code"
	@echo "  make tidy             - Tidy dependencies"
	@echo ""
	@echo "API:"
	@echo "  make health           - Check health endpoint"
	@echo "  make ready            - Check ready endpoint"
	@echo "  make features         - List enabled features"
	@echo "  make example-analyze  - Run example analysis"
