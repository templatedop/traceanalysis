.PHONY: all build test clean run-server run-worker docker-build docker-push deploy help

# Variables
BINARY_DIR := bin
SERVER_BINARY := $(BINARY_DIR)/server
WORKER_BINARY := $(BINARY_DIR)/worker
CLI_BINARY := $(BINARY_DIR)/rag-cli

GO := go
GOFLAGS := -ldflags="-w -s"
DOCKER_REGISTRY ?= ghcr.io
DOCKER_IMAGE := $(DOCKER_REGISTRY)/observability-analysis
DOCKER_TAG ?= latest

# Default target
all: build

# Build all binaries
build: $(SERVER_BINARY) $(WORKER_BINARY) $(CLI_BINARY)

$(BINARY_DIR):
	mkdir -p $(BINARY_DIR)

$(SERVER_BINARY): $(BINARY_DIR)
	$(GO) build $(GOFLAGS) -o $@ ./cmd/server

$(WORKER_BINARY): $(BINARY_DIR)
	$(GO) build $(GOFLAGS) -o $@ ./cmd/worker

$(CLI_BINARY): $(BINARY_DIR)
	$(GO) build $(GOFLAGS) -o $@ ./cmd/cli

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

# Docker commands
docker-build:
	docker build -t $(DOCKER_IMAGE):$(DOCKER_TAG) .
	docker build -t $(DOCKER_IMAGE)-server:$(DOCKER_TAG) --target server .
	docker build -t $(DOCKER_IMAGE)-worker:$(DOCKER_TAG) --target worker .
	docker build -t $(DOCKER_IMAGE)-cli:$(DOCKER_TAG) --target cli .

docker-push:
	docker push $(DOCKER_IMAGE):$(DOCKER_TAG)
	docker push $(DOCKER_IMAGE)-server:$(DOCKER_TAG)
	docker push $(DOCKER_IMAGE)-worker:$(DOCKER_TAG)
	docker push $(DOCKER_IMAGE)-cli:$(DOCKER_TAG)

# Docker Compose commands
compose-up:
	docker-compose up -d

compose-down:
	docker-compose down

compose-logs:
	docker-compose logs -f

compose-build:
	docker-compose build

# Kubernetes deployment
deploy:
	kubectl apply -f deploy/kubernetes/deployment.yaml

undeploy:
	kubectl delete -f deploy/kubernetes/deployment.yaml

# Development setup
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

# Generate API documentation
docs:
	@echo "API documentation available at http://localhost:8080/docs"

# Health check
health:
	curl -s http://localhost:8080/health | jq .

# Run analysis example
example-analyze:
	curl -X POST http://localhost:8080/api/v1/analyze \
		-H "Content-Type: application/json" \
		-d '{"query": "Why is the API slow?", "time_range_minutes": 60, "include_rag": true}' | jq .

# Help
help:
	@echo "Observability Analysis Framework - Make targets"
	@echo ""
	@echo "Build:"
	@echo "  make build          - Build all binaries"
	@echo "  make clean          - Clean build artifacts"
	@echo ""
	@echo "Test:"
	@echo "  make test           - Run tests with coverage"
	@echo "  make test-coverage  - Generate HTML coverage report"
	@echo "  make bench          - Run benchmarks"
	@echo "  make lint           - Run linter"
	@echo ""
	@echo "Run:"
	@echo "  make run-server     - Run API server"
	@echo "  make run-worker     - Run Temporal worker"
	@echo ""
	@echo "Docker:"
	@echo "  make docker-build   - Build Docker images"
	@echo "  make docker-push    - Push Docker images"
	@echo "  make compose-up     - Start with Docker Compose"
	@echo "  make compose-down   - Stop Docker Compose"
	@echo ""
	@echo "Kubernetes:"
	@echo "  make deploy         - Deploy to Kubernetes"
	@echo "  make undeploy       - Remove from Kubernetes"
	@echo ""
	@echo "Development:"
	@echo "  make dev-deps       - Install dev dependencies"
	@echo "  make init-config    - Create config file"
	@echo "  make load-examples  - Load example knowledge base"
	@echo "  make fmt            - Format code"
	@echo "  make tidy           - Tidy dependencies"
