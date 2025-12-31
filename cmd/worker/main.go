// Package main provides the Temporal worker for the observability analysis framework
package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/traceanalysis/rag-temporal/internal/config"
	"github.com/traceanalysis/rag-temporal/pkg/agents"
	"github.com/traceanalysis/rag-temporal/pkg/collectors"
	"github.com/traceanalysis/rag-temporal/pkg/llm"
	"github.com/traceanalysis/rag-temporal/pkg/rag"
	"github.com/traceanalysis/rag-temporal/pkg/workflow"
)

var (
	configPath = flag.String("config", "config.yaml", "Path to configuration file")
)

func main() {
	flag.Parse()

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize components
	embedder := rag.NewOllamaEmbedder(cfg.LLM.BaseURL, cfg.RAG.EmbeddingModel, cfg.RAG.EmbeddingDim)
	ragStore := rag.NewMemoryStore(embedder)

	// Load knowledge base
	if cfg.RAG.KnowledgeBase != "" {
		kb := rag.NewKnowledgeBase(ragStore, embedder, cfg.RAG.KnowledgeBase)
		if err := kb.LoadFromDirectory(ctx, cfg.RAG.KnowledgeBase); err != nil {
			log.Printf("Warning: failed to load knowledge base: %v", err)
		} else {
			count, _ := ragStore.Count(ctx)
			log.Printf("Loaded %d documents into knowledge base", count)
		}
	}

	// Initialize LLM client
	llmClient, err := llm.NewClient(
		cfg.LLM.Provider,
		cfg.LLM.BaseURL,
		cfg.LLM.APIKey,
		cfg.LLM.DefaultModel,
		cfg.LLM.Timeout,
		cfg.LLM.Temperature,
		cfg.LLM.MaxTokens,
	)
	if err != nil {
		log.Fatalf("Failed to create LLM client: %v", err)
	}

	// Initialize collectors
	var traceCollector *collectors.JaegerCollector
	var metricCollector *collectors.PrometheusCollector
	var logCollector *collectors.LokiCollector

	if cfg.Collectors.Jaeger.Enabled {
		traceCollector, err = collectors.NewJaegerCollector(&cfg.Collectors.Jaeger)
		if err != nil {
			log.Printf("Warning: failed to create Jaeger collector: %v", err)
		}
	}

	if cfg.Collectors.Prometheus.Enabled {
		metricCollector, err = collectors.NewPrometheusCollector(&cfg.Collectors.Prometheus)
		if err != nil {
			log.Printf("Warning: failed to create Prometheus collector: %v", err)
		}
	}

	if cfg.Collectors.Loki.Enabled {
		logCollector, err = collectors.NewLokiCollector(&cfg.Collectors.Loki)
		if err != nil {
			log.Printf("Warning: failed to create Loki collector: %v", err)
		}
	}

	// Initialize agents
	var traceAgent *agents.TraceAgent
	var metricAgent *agents.MetricAgent
	var logAgent *agents.LogAgent

	if traceCollector != nil {
		traceAgent = agents.NewTraceAgent(llmClient, ragStore, traceCollector)
	}
	if metricCollector != nil {
		metricAgent = agents.NewMetricAgent(llmClient, ragStore, metricCollector)
	}
	if logCollector != nil {
		logAgent = agents.NewLogAgent(llmClient, ragStore, logCollector)
	}

	// Create and start worker
	worker, err := workflow.NewWorker(
		&cfg.Temporal,
		traceAgent,
		metricAgent,
		logAgent,
		llmClient,
		ragStore,
	)
	if err != nil {
		log.Fatalf("Failed to create worker: %v", err)
	}

	// Handle shutdown gracefully
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("Shutting down worker...")
		worker.Stop()
		cancel()
	}()

	log.Printf("Starting Temporal worker on task queue: %s", cfg.Temporal.TaskQueue)
	if err := worker.Start(); err != nil {
		log.Fatalf("Worker error: %v", err)
	}
}
