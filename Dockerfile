# Build stage
FROM golang:1.21-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build binaries with version info
ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_TIME=unknown

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s -X main.Version=${VERSION} -X main.Commit=${COMMIT} -X main.BuildTime=${BUILD_TIME}" -o /bin/server ./cmd/server
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s -X main.Version=${VERSION} -X main.Commit=${COMMIT} -X main.BuildTime=${BUILD_TIME}" -o /bin/worker ./cmd/worker
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /bin/rag-cli ./cmd/cli

# Runtime stage for server
FROM alpine:3.19 AS server

RUN apk add --no-cache ca-certificates tzdata curl

WORKDIR /app

# Create non-root user
RUN addgroup -g 1000 appgroup && \
    adduser -u 1000 -G appgroup -D appuser

COPY --from=builder /bin/server /usr/local/bin/server
COPY config.example.yaml /app/config.yaml

# Create directory for knowledge base
RUN mkdir -p /app/knowledge && chown -R appuser:appgroup /app

USER appuser

EXPOSE 8080

# Graceful shutdown signal
STOPSIGNAL SIGTERM

# Health check
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD curl -f http://localhost:8080/health || exit 1

ENTRYPOINT ["server"]
CMD ["--config", "/app/config.yaml"]

# Runtime stage for worker
FROM alpine:3.19 AS worker

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

# Create non-root user
RUN addgroup -g 1000 appgroup && \
    adduser -u 1000 -G appgroup -D appuser

COPY --from=builder /bin/worker /usr/local/bin/worker
COPY config.example.yaml /app/config.yaml

# Create directory for knowledge base
RUN mkdir -p /app/knowledge && chown -R appuser:appgroup /app

USER appuser

# Graceful shutdown signal
STOPSIGNAL SIGTERM

ENTRYPOINT ["worker"]
CMD ["--config", "/app/config.yaml"]

# Runtime stage for CLI
FROM alpine:3.19 AS cli

RUN apk add --no-cache ca-certificates tzdata

# Create non-root user
RUN addgroup -g 1000 appgroup && \
    adduser -u 1000 -G appgroup -D appuser

COPY --from=builder /bin/rag-cli /usr/local/bin/rag-cli

USER appuser

ENTRYPOINT ["rag-cli"]

# Default runtime stage (includes all binaries)
FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata curl

WORKDIR /app

# Create non-root user
RUN addgroup -g 1000 appgroup && \
    adduser -u 1000 -G appgroup -D appuser

COPY --from=builder /bin/server /usr/local/bin/server
COPY --from=builder /bin/worker /usr/local/bin/worker
COPY --from=builder /bin/rag-cli /usr/local/bin/rag-cli
COPY config.example.yaml /app/config.yaml

# Create directory for knowledge base
RUN mkdir -p /app/knowledge && chown -R appuser:appgroup /app

USER appuser

EXPOSE 8080

# Graceful shutdown signal
STOPSIGNAL SIGTERM

# Health check
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD curl -f http://localhost:8080/health || exit 1

# Default to running the server
ENTRYPOINT ["server"]
CMD ["--config", "/app/config.yaml"]
