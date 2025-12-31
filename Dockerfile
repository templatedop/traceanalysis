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

# Build binaries
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /bin/server ./cmd/server
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /bin/worker ./cmd/worker
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /bin/rag-cli ./cmd/cli

# Runtime stage for server
FROM alpine:3.19 AS server

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

COPY --from=builder /bin/server /usr/local/bin/server
COPY config.example.yaml /app/config.yaml

# Create directory for knowledge base
RUN mkdir -p /app/knowledge

EXPOSE 8080

ENTRYPOINT ["server"]
CMD ["--config", "/app/config.yaml"]

# Runtime stage for worker
FROM alpine:3.19 AS worker

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

COPY --from=builder /bin/worker /usr/local/bin/worker
COPY config.example.yaml /app/config.yaml

# Create directory for knowledge base
RUN mkdir -p /app/knowledge

ENTRYPOINT ["worker"]
CMD ["--config", "/app/config.yaml"]

# Runtime stage for CLI
FROM alpine:3.19 AS cli

RUN apk add --no-cache ca-certificates tzdata

COPY --from=builder /bin/rag-cli /usr/local/bin/rag-cli

ENTRYPOINT ["rag-cli"]

# Default runtime stage (includes all binaries)
FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

COPY --from=builder /bin/server /usr/local/bin/server
COPY --from=builder /bin/worker /usr/local/bin/worker
COPY --from=builder /bin/rag-cli /usr/local/bin/rag-cli
COPY config.example.yaml /app/config.yaml

# Create directory for knowledge base
RUN mkdir -p /app/knowledge

EXPOSE 8080

# Default to running the server
ENTRYPOINT ["server"]
CMD ["--config", "/app/config.yaml"]
