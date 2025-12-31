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
	Features    FeaturesConfig    `yaml:"features"`
	Temporal    TemporalConfig    `yaml:"temporal"`
	LLM         LLMConfig         `yaml:"llm"`
	RAG         RAGConfig         `yaml:"rag"`
	Caching     CachingConfig     `yaml:"caching"`
	Collectors  CollectorsConfig  `yaml:"collectors"`
	Agents      AgentsConfig      `yaml:"agents"`
	Analysis    AnalysisConfig    `yaml:"analysis"`
	Alerting    AlertingConfig    `yaml:"alerting"`
	Metrics     MetricsConfig     `yaml:"metrics"`
	Logging     LoggingConfig     `yaml:"logging"`
}

// FeaturesConfig contains feature flags to enable/disable major components
type FeaturesConfig struct {
	// EnableTemporal enables Temporal workflow orchestration
	// When disabled, analysis runs synchronously without workflow durability
	EnableTemporal bool `yaml:"enable_temporal"`

	// EnableRAG enables the RAG system for knowledge-augmented analysis
	// When disabled, LLM analysis runs without context from knowledge base
	EnableRAG bool `yaml:"enable_rag"`

	// EnableCaching enables LLM response caching
	// Helps reduce costs and latency for repeated queries
	EnableCaching bool `yaml:"enable_caching"`

	// EnableMetrics enables Prometheus metrics for self-observability
	EnableMetrics bool `yaml:"enable_metrics"`

	// EnableAlerting enables alert notifications (Slack, PagerDuty, etc.)
	EnableAlerting bool `yaml:"enable_alerting"`

	// EnablePredictions enables predictive analysis and forecasting
	EnablePredictions bool `yaml:"enable_predictions"`
}

// ServerConfig contains HTTP server settings
type ServerConfig struct {
	Host            string        `yaml:"host"`
	Port            int           `yaml:"port"`
	ReadTimeout     time.Duration `yaml:"read_timeout"`
	WriteTimeout    time.Duration `yaml:"write_timeout"`
	IdleTimeout     time.Duration `yaml:"idle_timeout"`
	ShutdownTimeout time.Duration `yaml:"shutdown_timeout"`
	DrainTimeout    time.Duration `yaml:"drain_timeout"`
	EnableTLS       bool          `yaml:"enable_tls"`
	TLSCertFile     string        `yaml:"tls_cert_file"`
	TLSKeyFile      string        `yaml:"tls_key_file"`
}

// TemporalConfig contains Temporal workflow settings
type TemporalConfig struct {
	HostPort              string        `yaml:"host_port"`
	Namespace             string        `yaml:"namespace"`
	TaskQueue             string        `yaml:"task_queue"`
	WorkerCount           int           `yaml:"worker_count"`
	MaxConcurrent         int           `yaml:"max_concurrent"`
	EnableTLS             bool          `yaml:"enable_tls"`
	TLSCertFile           string        `yaml:"tls_cert_file"`
	TLSKeyFile            string        `yaml:"tls_key_file"`
	WorkflowTimeout       time.Duration `yaml:"workflow_timeout"`
	ActivityTimeout       time.Duration `yaml:"activity_timeout"`
	RetryMaxAttempts      int           `yaml:"retry_max_attempts"`
	RetryInitialInterval  time.Duration `yaml:"retry_initial_interval"`
}

// CachingConfig contains LLM response caching settings
type CachingConfig struct {
	MaxEntries int           `yaml:"max_entries"`
	TTL        time.Duration `yaml:"ttl"`
	// CleanupInterval is how often to run cache cleanup
	CleanupInterval time.Duration `yaml:"cleanup_interval"`
}

// AgentsConfig contains settings for individual analysis agents
type AgentsConfig struct {
	Trace  AgentConfig `yaml:"trace"`
	Metric AgentConfig `yaml:"metric"`
	Log    AgentConfig `yaml:"log"`
}

// AgentConfig contains settings for a single agent
type AgentConfig struct {
	Enabled     bool          `yaml:"enabled"`
	Timeout     time.Duration `yaml:"timeout"`
	MaxRetries  int           `yaml:"max_retries"`
	Model       string        `yaml:"model"`        // Override default LLM model
	Concurrency int           `yaml:"concurrency"`  // Max concurrent operations
}

// MetricsConfig contains self-observability metrics settings
type MetricsConfig struct {
	Path       string `yaml:"path"`        // Metrics endpoint path
	Namespace  string `yaml:"namespace"`   // Prometheus namespace
	Subsystem  string `yaml:"subsystem"`   // Prometheus subsystem
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
	VectorStore       VectorStoreConfig `yaml:"vector_store"`
	EmbeddingModel    string            `yaml:"embedding_model"`
	EmbeddingProvider string            `yaml:"embedding_provider"` // ollama, openai
	EmbeddingDim      int               `yaml:"embedding_dim"`
	ChunkSize         int               `yaml:"chunk_size"`
	ChunkOverlap      int               `yaml:"chunk_overlap"`
	TopK              int               `yaml:"top_k"`
	MinScore          float64           `yaml:"min_score"`
	KnowledgeBase     string            `yaml:"knowledge_base_path"`
	AutoSync          bool              `yaml:"auto_sync"`
	SyncInterval      time.Duration     `yaml:"sync_interval"`
	// IndexOnStartup controls whether to index the knowledge base when the application starts
	IndexOnStartup    bool              `yaml:"index_on_startup"`
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
			Host:            "0.0.0.0",
			Port:            8080,
			ReadTimeout:     30 * time.Second,
			WriteTimeout:    30 * time.Second,
			IdleTimeout:     120 * time.Second,
			ShutdownTimeout: 30 * time.Second,
			DrainTimeout:    10 * time.Second,
		},
		Features: FeaturesConfig{
			EnableTemporal:    true,
			EnableRAG:         true,
			EnableCaching:     true,
			EnableMetrics:     true,
			EnableAlerting:    false,
			EnablePredictions: true,
		},
		Temporal: TemporalConfig{
			HostPort:             "localhost:7233",
			Namespace:            "default",
			TaskQueue:            "observability-analysis",
			WorkerCount:          4,
			MaxConcurrent:        10,
			WorkflowTimeout:      10 * time.Minute,
			ActivityTimeout:      5 * time.Minute,
			RetryMaxAttempts:     3,
			RetryInitialInterval: 1 * time.Second,
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
				Dimension:  768,
			},
			EmbeddingModel:    "nomic-embed-text",
			EmbeddingProvider: "ollama",
			EmbeddingDim:      768,
			ChunkSize:         1000,
			ChunkOverlap:      200,
			TopK:              5,
			MinScore:          0.7,
			KnowledgeBase:     "./knowledge",
			AutoSync:          true,
			SyncInterval:      5 * time.Minute,
			IndexOnStartup:    true,
		},
		Caching: CachingConfig{
			MaxEntries:      1000,
			TTL:             1 * time.Hour,
			CleanupInterval: 5 * time.Minute,
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
		Agents: AgentsConfig{
			Trace: AgentConfig{
				Enabled:     true,
				Timeout:     2 * time.Minute,
				MaxRetries:  3,
				Concurrency: 5,
			},
			Metric: AgentConfig{
				Enabled:     true,
				Timeout:     2 * time.Minute,
				MaxRetries:  3,
				Concurrency: 5,
			},
			Log: AgentConfig{
				Enabled:     true,
				Timeout:     2 * time.Minute,
				MaxRetries:  3,
				Concurrency: 5,
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
		Metrics: MetricsConfig{
			Path:      "/metrics",
			Namespace: "observability_analysis",
			Subsystem: "framework",
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "json",
			Output: "stdout",
		},
	}
}

// IsFeatureEnabled checks if a feature is enabled by name
func (c *Config) IsFeatureEnabled(feature string) bool {
	switch feature {
	case "temporal":
		return c.Features.EnableTemporal
	case "rag":
		return c.Features.EnableRAG
	case "caching":
		return c.Features.EnableCaching
	case "metrics":
		return c.Features.EnableMetrics
	case "alerting":
		return c.Features.EnableAlerting
	case "predictions":
		return c.Features.EnablePredictions
	default:
		return false
	}
}

// IsAgentEnabled checks if a specific agent is enabled
func (c *Config) IsAgentEnabled(agent string) bool {
	switch agent {
	case "trace":
		return c.Agents.Trace.Enabled
	case "metric":
		return c.Agents.Metric.Enabled
	case "log":
		return c.Agents.Log.Enabled
	default:
		return false
	}
}

// IsCollectorEnabled checks if a specific collector is enabled
func (c *Config) IsCollectorEnabled(collector string) bool {
	switch collector {
	case "prometheus":
		return c.Collectors.Prometheus.Enabled
	case "jaeger":
		return c.Collectors.Jaeger.Enabled
	case "loki":
		return c.Collectors.Loki.Enabled
	case "tempo":
		return c.Collectors.Tempo.Enabled
	case "otel":
		return c.Collectors.OTEL.Enabled
	default:
		return false
	}
}

// GetEnabledFeatures returns a list of enabled features
func (c *Config) GetEnabledFeatures() []string {
	var features []string
	if c.Features.EnableTemporal {
		features = append(features, "temporal")
	}
	if c.Features.EnableRAG {
		features = append(features, "rag")
	}
	if c.Features.EnableCaching {
		features = append(features, "caching")
	}
	if c.Features.EnableMetrics {
		features = append(features, "metrics")
	}
	if c.Features.EnableAlerting {
		features = append(features, "alerting")
	}
	if c.Features.EnablePredictions {
		features = append(features, "predictions")
	}
	return features
}

// GetEnabledAgents returns a list of enabled agents
func (c *Config) GetEnabledAgents() []string {
	var agents []string
	if c.Agents.Trace.Enabled {
		agents = append(agents, "trace")
	}
	if c.Agents.Metric.Enabled {
		agents = append(agents, "metric")
	}
	if c.Agents.Log.Enabled {
		agents = append(agents, "log")
	}
	return agents
}

// GetEnabledCollectors returns a list of enabled collectors
func (c *Config) GetEnabledCollectors() []string {
	var collectors []string
	if c.Collectors.Prometheus.Enabled {
		collectors = append(collectors, "prometheus")
	}
	if c.Collectors.Jaeger.Enabled {
		collectors = append(collectors, "jaeger")
	}
	if c.Collectors.Loki.Enabled {
		collectors = append(collectors, "loki")
	}
	if c.Collectors.Tempo.Enabled {
		collectors = append(collectors, "tempo")
	}
	if c.Collectors.OTEL.Enabled {
		collectors = append(collectors, "otel")
	}
	return collectors
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
