// Package config provides configuration management for the observability analysis framework
package config

import (
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config represents the main configuration
type Config struct {
	Server      ServerConfig      `yaml:"server"`
	Temporal    TemporalConfig    `yaml:"temporal"`
	LLM         LLMConfig         `yaml:"llm"`
	RAG         RAGConfig         `yaml:"rag"`
	Collectors  CollectorsConfig  `yaml:"collectors"`
	Analysis    AnalysisConfig    `yaml:"analysis"`
	Alerting    AlertingConfig    `yaml:"alerting"`
	Logging     LoggingConfig     `yaml:"logging"`
}

// ServerConfig contains HTTP server settings
type ServerConfig struct {
	Host         string        `yaml:"host"`
	Port         int           `yaml:"port"`
	ReadTimeout  time.Duration `yaml:"read_timeout"`
	WriteTimeout time.Duration `yaml:"write_timeout"`
	EnableTLS    bool          `yaml:"enable_tls"`
	TLSCertFile  string        `yaml:"tls_cert_file"`
	TLSKeyFile   string        `yaml:"tls_key_file"`
}

// TemporalConfig contains Temporal workflow settings
type TemporalConfig struct {
	HostPort       string `yaml:"host_port"`
	Namespace      string `yaml:"namespace"`
	TaskQueue      string `yaml:"task_queue"`
	WorkerCount    int    `yaml:"worker_count"`
	MaxConcurrent  int    `yaml:"max_concurrent"`
	EnableTLS      bool   `yaml:"enable_tls"`
	TLSCertFile    string `yaml:"tls_cert_file"`
	TLSKeyFile     string `yaml:"tls_key_file"`
}

// LLMConfig contains LLM provider settings
type LLMConfig struct {
	Provider        string         `yaml:"provider"` // openai, ollama, anthropic, deepseek
	DefaultModel    string         `yaml:"default_model"`
	BaseURL         string         `yaml:"base_url"`
	APIKey          string         `yaml:"api_key"`
	Timeout         time.Duration  `yaml:"timeout"`
	MaxRetries      int            `yaml:"max_retries"`
	Temperature     float64        `yaml:"temperature"`
	MaxTokens       int            `yaml:"max_tokens"`
	Models          []ModelConfig  `yaml:"models"`
	RateLimitRPM    int            `yaml:"rate_limit_rpm"`
}

// ModelConfig contains configuration for a specific model
type ModelConfig struct {
	Name         string   `yaml:"name"`
	Provider     string   `yaml:"provider"`
	Model        string   `yaml:"model"`
	BaseURL      string   `yaml:"base_url"`
	APIKey       string   `yaml:"api_key"`
	MaxTokens    int      `yaml:"max_tokens"`
	Temperature  float64  `yaml:"temperature"`
	UseCases     []string `yaml:"use_cases"` // trace, metric, log, general
}

// RAGConfig contains RAG system settings
type RAGConfig struct {
	VectorStore     VectorStoreConfig `yaml:"vector_store"`
	EmbeddingModel  string            `yaml:"embedding_model"`
	EmbeddingDim    int               `yaml:"embedding_dim"`
	ChunkSize       int               `yaml:"chunk_size"`
	ChunkOverlap    int               `yaml:"chunk_overlap"`
	TopK            int               `yaml:"top_k"`
	MinScore        float64           `yaml:"min_score"`
	KnowledgeBase   string            `yaml:"knowledge_base_path"`
	AutoSync        bool              `yaml:"auto_sync"`
	SyncInterval    time.Duration     `yaml:"sync_interval"`
}

// VectorStoreConfig contains vector database settings
type VectorStoreConfig struct {
	Type       string `yaml:"type"` // memory, qdrant, weaviate, pinecone, pgvector
	Host       string `yaml:"host"`
	Port       int    `yaml:"port"`
	Collection string `yaml:"collection"`
	APIKey     string `yaml:"api_key"`
	Dimension  int    `yaml:"dimension"`
}

// CollectorsConfig contains data collector settings
type CollectorsConfig struct {
	Prometheus  PrometheusConfig  `yaml:"prometheus"`
	Jaeger      JaegerConfig      `yaml:"jaeger"`
	Tempo       TempoConfig       `yaml:"tempo"`
	Loki        LokiConfig        `yaml:"loki"`
	OTEL        OTELConfig        `yaml:"otel"`
}

// PrometheusConfig contains Prometheus settings
type PrometheusConfig struct {
	Enabled  bool          `yaml:"enabled"`
	URL      string        `yaml:"url"`
	Timeout  time.Duration `yaml:"timeout"`
	Username string        `yaml:"username"`
	Password string        `yaml:"password"`
}

// JaegerConfig contains Jaeger settings
type JaegerConfig struct {
	Enabled     bool          `yaml:"enabled"`
	QueryURL    string        `yaml:"query_url"`
	GRPCAddress string        `yaml:"grpc_address"`
	Timeout     time.Duration `yaml:"timeout"`
}

// TempoConfig contains Tempo settings
type TempoConfig struct {
	Enabled     bool          `yaml:"enabled"`
	URL         string        `yaml:"url"`
	GRPCAddress string        `yaml:"grpc_address"`
	Timeout     time.Duration `yaml:"timeout"`
}

// LokiConfig contains Loki settings
type LokiConfig struct {
	Enabled  bool          `yaml:"enabled"`
	URL      string        `yaml:"url"`
	Timeout  time.Duration `yaml:"timeout"`
	Username string        `yaml:"username"`
	Password string        `yaml:"password"`
}

// OTELConfig contains OpenTelemetry collector settings
type OTELConfig struct {
	Enabled      bool   `yaml:"enabled"`
	GRPCEndpoint string `yaml:"grpc_endpoint"`
	HTTPEndpoint string `yaml:"http_endpoint"`
}

// AnalysisConfig contains analysis settings
type AnalysisConfig struct {
	DefaultTimeRange time.Duration `yaml:"default_time_range"`
	MaxTraces        int           `yaml:"max_traces"`
	MaxLogs          int           `yaml:"max_logs"`
	AnomalyThreshold float64       `yaml:"anomaly_threshold"`
	Forecasting      ForecastConfig `yaml:"forecasting"`
}

// ForecastConfig contains forecasting settings
type ForecastConfig struct {
	Enabled        bool          `yaml:"enabled"`
	Horizon        time.Duration `yaml:"horizon"`
	MinDataPoints  int           `yaml:"min_data_points"`
	ConfidenceLevel float64      `yaml:"confidence_level"`
}

// AlertingConfig contains alerting settings
type AlertingConfig struct {
	Enabled      bool              `yaml:"enabled"`
	Slack        SlackConfig       `yaml:"slack"`
	PagerDuty    PagerDutyConfig   `yaml:"pagerduty"`
	Webhook      WebhookConfig     `yaml:"webhook"`
}

// SlackConfig contains Slack integration settings
type SlackConfig struct {
	Enabled    bool   `yaml:"enabled"`
	WebhookURL string `yaml:"webhook_url"`
	Channel    string `yaml:"channel"`
}

// PagerDutyConfig contains PagerDuty integration settings
type PagerDutyConfig struct {
	Enabled     bool   `yaml:"enabled"`
	RoutingKey  string `yaml:"routing_key"`
	ServiceKey  string `yaml:"service_key"`
}

// WebhookConfig contains generic webhook settings
type WebhookConfig struct {
	Enabled bool   `yaml:"enabled"`
	URL     string `yaml:"url"`
	Headers map[string]string `yaml:"headers"`
}

// LoggingConfig contains logging settings
type LoggingConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"` // json, text
	Output string `yaml:"output"` // stdout, file
	File   string `yaml:"file"`
}

// DefaultConfig returns a configuration with sensible defaults
func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Host:         "0.0.0.0",
			Port:         8080,
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 30 * time.Second,
		},
		Temporal: TemporalConfig{
			HostPort:      "localhost:7233",
			Namespace:     "default",
			TaskQueue:     "observability-analysis",
			WorkerCount:   4,
			MaxConcurrent: 10,
		},
		LLM: LLMConfig{
			Provider:     "ollama",
			DefaultModel: "qwen2.5:32b",
			BaseURL:      "http://localhost:11434",
			Timeout:      5 * time.Minute,
			MaxRetries:   3,
			Temperature:  0.1,
			MaxTokens:    4096,
			RateLimitRPM: 60,
		},
		RAG: RAGConfig{
			VectorStore: VectorStoreConfig{
				Type:       "memory",
				Collection: "observability",
				Dimension:  1536,
			},
			EmbeddingModel: "nomic-embed-text",
			EmbeddingDim:   768,
			ChunkSize:      1000,
			ChunkOverlap:   200,
			TopK:           5,
			MinScore:       0.7,
			KnowledgeBase:  "./knowledge",
			AutoSync:       true,
			SyncInterval:   5 * time.Minute,
		},
		Collectors: CollectorsConfig{
			Prometheus: PrometheusConfig{
				Enabled: true,
				URL:     "http://localhost:9090",
				Timeout: 30 * time.Second,
			},
			Jaeger: JaegerConfig{
				Enabled:  true,
				QueryURL: "http://localhost:16686",
				Timeout:  30 * time.Second,
			},
			Loki: LokiConfig{
				Enabled: true,
				URL:     "http://localhost:3100",
				Timeout: 30 * time.Second,
			},
		},
		Analysis: AnalysisConfig{
			DefaultTimeRange: 1 * time.Hour,
			MaxTraces:        1000,
			MaxLogs:          10000,
			AnomalyThreshold: 3.0,
			Forecasting: ForecastConfig{
				Enabled:         true,
				Horizon:         24 * time.Hour,
				MinDataPoints:   100,
				ConfidenceLevel: 0.95,
			},
		},
		Alerting: AlertingConfig{
			Enabled: false,
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "json",
			Output: "stdout",
		},
	}
}

// Load loads configuration from a YAML file
func Load(path string) (*Config, error) {
	cfg := DefaultConfig()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, err
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	// Override with environment variables
	cfg.applyEnvOverrides()

	return cfg, nil
}

// applyEnvOverrides applies environment variable overrides
func (c *Config) applyEnvOverrides() {
	if v := os.Getenv("LLM_API_KEY"); v != "" {
		c.LLM.APIKey = v
	}
	if v := os.Getenv("LLM_BASE_URL"); v != "" {
		c.LLM.BaseURL = v
	}
	if v := os.Getenv("LLM_MODEL"); v != "" {
		c.LLM.DefaultModel = v
	}
	if v := os.Getenv("TEMPORAL_HOST"); v != "" {
		c.Temporal.HostPort = v
	}
	if v := os.Getenv("PROMETHEUS_URL"); v != "" {
		c.Collectors.Prometheus.URL = v
	}
	if v := os.Getenv("JAEGER_URL"); v != "" {
		c.Collectors.Jaeger.QueryURL = v
	}
	if v := os.Getenv("LOKI_URL"); v != "" {
		c.Collectors.Loki.URL = v
	}
}

// Save saves the configuration to a YAML file
func (c *Config) Save(path string) error {
	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
