// Package llm provides LLM client implementations for various providers
package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/traceanalysis/rag-temporal/internal/config"
	"github.com/traceanalysis/rag-temporal/internal/types"
)

// Client is the interface for LLM providers
type Client interface {
	// Complete sends a completion request to the LLM
	Complete(ctx context.Context, req *types.LLMRequest) (*types.LLMResponse, error)
	// CompleteJSON sends a completion request and parses JSON response
	CompleteJSON(ctx context.Context, req *types.LLMRequest, result interface{}) error
	// Name returns the provider name
	Name() string
	// Model returns the default model name
	Model() string
	// Health checks if the LLM is available
	Health(ctx context.Context) error
}

// MultiClient manages multiple LLM clients and routes requests
type MultiClient struct {
	clients       map[string]Client
	defaultClient Client
	useCaseMap    map[string]string // maps use case to client name
	mu            sync.RWMutex
}

// NewMultiClient creates a new multi-client LLM manager
func NewMultiClient(cfg *config.LLMConfig) (*MultiClient, error) {
	mc := &MultiClient{
		clients:    make(map[string]Client),
		useCaseMap: make(map[string]string),
	}

	// Create default client
	defaultClient, err := NewClient(cfg.Provider, cfg.BaseURL, cfg.APIKey, cfg.DefaultModel, cfg.Timeout, cfg.Temperature, cfg.MaxTokens)
	if err != nil {
		return nil, fmt.Errorf("failed to create default client: %w", err)
	}
	mc.defaultClient = defaultClient
	mc.clients["default"] = defaultClient

	// Create additional clients for specific models
	for _, modelCfg := range cfg.Models {
		client, err := NewClient(modelCfg.Provider, modelCfg.BaseURL, modelCfg.APIKey, modelCfg.Model, cfg.Timeout, modelCfg.Temperature, modelCfg.MaxTokens)
		if err != nil {
			return nil, fmt.Errorf("failed to create client for %s: %w", modelCfg.Name, err)
		}
		mc.clients[modelCfg.Name] = client

		// Map use cases to this client
		for _, useCase := range modelCfg.UseCases {
			mc.useCaseMap[useCase] = modelCfg.Name
		}
	}

	return mc, nil
}

// GetClient returns the appropriate client for a use case
func (mc *MultiClient) GetClient(useCase string) Client {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	if clientName, ok := mc.useCaseMap[useCase]; ok {
		if client, ok := mc.clients[clientName]; ok {
			return client
		}
	}
	return mc.defaultClient
}

// Complete sends a completion request using the default client
func (mc *MultiClient) Complete(ctx context.Context, req *types.LLMRequest) (*types.LLMResponse, error) {
	return mc.defaultClient.Complete(ctx, req)
}

// CompleteJSON sends a completion request and parses JSON response
func (mc *MultiClient) CompleteJSON(ctx context.Context, req *types.LLMRequest, result interface{}) error {
	return mc.defaultClient.CompleteJSON(ctx, req, result)
}

// Name returns the default provider name
func (mc *MultiClient) Name() string {
	return mc.defaultClient.Name()
}

// Model returns the default model name
func (mc *MultiClient) Model() string {
	return mc.defaultClient.Model()
}

// Health checks all clients
func (mc *MultiClient) Health(ctx context.Context) error {
	for name, client := range mc.clients {
		if err := client.Health(ctx); err != nil {
			return fmt.Errorf("client %s unhealthy: %w", name, err)
		}
	}
	return nil
}

// NewClient creates a new LLM client based on provider
func NewClient(provider, baseURL, apiKey, model string, timeout time.Duration, temperature float64, maxTokens int) (Client, error) {
	switch strings.ToLower(provider) {
	case "ollama":
		return NewOllamaClient(baseURL, model, timeout, temperature, maxTokens), nil
	case "openai":
		return NewOpenAIClient(baseURL, apiKey, model, timeout, temperature, maxTokens), nil
	case "anthropic":
		return NewAnthropicClient(baseURL, apiKey, model, timeout, temperature, maxTokens), nil
	case "deepseek":
		return NewDeepSeekClient(baseURL, apiKey, model, timeout, temperature, maxTokens), nil
	default:
		return nil, fmt.Errorf("unsupported provider: %s", provider)
	}
}

// OllamaClient is an LLM client for Ollama
type OllamaClient struct {
	baseURL     string
	model       string
	client      *http.Client
	temperature float64
	maxTokens   int
}

// NewOllamaClient creates a new Ollama client
func NewOllamaClient(baseURL, model string, timeout time.Duration, temperature float64, maxTokens int) *OllamaClient {
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}
	if model == "" {
		model = "qwen2.5:32b"
	}
	return &OllamaClient{
		baseURL: baseURL,
		model:   model,
		client: &http.Client{
			Timeout: timeout,
		},
		temperature: temperature,
		maxTokens:   maxTokens,
	}
}

type ollamaRequest struct {
	Model    string             `json:"model"`
	Prompt   string             `json:"prompt"`
	System   string             `json:"system,omitempty"`
	Stream   bool               `json:"stream"`
	Options  *ollamaOptions     `json:"options,omitempty"`
}

type ollamaOptions struct {
	Temperature float64 `json:"temperature,omitempty"`
	NumPredict  int     `json:"num_predict,omitempty"`
}

type ollamaResponse struct {
	Response string `json:"response"`
	Done     bool   `json:"done"`
}

// Complete sends a completion request to Ollama
func (c *OllamaClient) Complete(ctx context.Context, req *types.LLMRequest) (*types.LLMResponse, error) {
	model := req.Model
	if model == "" {
		model = c.model
	}

	ollamaReq := ollamaRequest{
		Model:  model,
		Prompt: req.UserPrompt,
		System: req.SystemPrompt,
		Stream: false,
		Options: &ollamaOptions{
			Temperature: req.Temperature,
			NumPredict:  req.MaxTokens,
		},
	}

	if ollamaReq.Options.Temperature == 0 {
		ollamaReq.Options.Temperature = c.temperature
	}
	if ollamaReq.Options.NumPredict == 0 {
		ollamaReq.Options.NumPredict = c.maxTokens
	}

	body, err := json.Marshal(ollamaReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/api/generate", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ollama returned status %d: %s", resp.StatusCode, string(body))
	}

	var result ollamaResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &types.LLMResponse{
		Content:      result.Response,
		Model:        model,
		FinishReason: "stop",
	}, nil
}

// CompleteJSON sends a completion request and parses JSON response
func (c *OllamaClient) CompleteJSON(ctx context.Context, req *types.LLMRequest, result interface{}) error {
	// Add JSON instruction to prompt
	req.UserPrompt = req.UserPrompt + "\n\nRespond with valid JSON only. No markdown, no explanation."

	resp, err := c.Complete(ctx, req)
	if err != nil {
		return err
	}

	// Clean the response
	content := strings.TrimSpace(resp.Content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)

	if err := json.Unmarshal([]byte(content), result); err != nil {
		return fmt.Errorf("failed to parse JSON response: %w, content: %s", err, content)
	}

	return nil
}

// Name returns the provider name
func (c *OllamaClient) Name() string {
	return "ollama"
}

// Model returns the model name
func (c *OllamaClient) Model() string {
	return c.model
}

// Health checks if Ollama is available
func (c *OllamaClient) Health(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/api/tags", nil)
	if err != nil {
		return err
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("ollama returned status %d", resp.StatusCode)
	}
	return nil
}

// OpenAIClient is an LLM client for OpenAI-compatible APIs
type OpenAIClient struct {
	baseURL     string
	apiKey      string
	model       string
	client      *http.Client
	temperature float64
	maxTokens   int
}

// NewOpenAIClient creates a new OpenAI client
func NewOpenAIClient(baseURL, apiKey, model string, timeout time.Duration, temperature float64, maxTokens int) *OpenAIClient {
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	if model == "" {
		model = "gpt-4"
	}
	return &OpenAIClient{
		baseURL:     baseURL,
		apiKey:      apiKey,
		model:       model,
		client:      &http.Client{Timeout: timeout},
		temperature: temperature,
		maxTokens:   maxTokens,
	}
}

type openAIRequest struct {
	Model       string         `json:"model"`
	Messages    []openAIMessage `json:"messages"`
	Temperature float64        `json:"temperature,omitempty"`
	MaxTokens   int            `json:"max_tokens,omitempty"`
}

type openAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIResponse struct {
	Choices []struct {
		Message      openAIMessage `json:"message"`
		FinishReason string        `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		TotalTokens int `json:"total_tokens"`
	} `json:"usage"`
}

// Complete sends a completion request to OpenAI
func (c *OpenAIClient) Complete(ctx context.Context, req *types.LLMRequest) (*types.LLMResponse, error) {
	model := req.Model
	if model == "" {
		model = c.model
	}

	messages := []openAIMessage{}
	if req.SystemPrompt != "" {
		messages = append(messages, openAIMessage{Role: "system", Content: req.SystemPrompt})
	}
	messages = append(messages, openAIMessage{Role: "user", Content: req.UserPrompt})

	temperature := req.Temperature
	if temperature == 0 {
		temperature = c.temperature
	}
	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = c.maxTokens
	}

	openaiReq := openAIRequest{
		Model:       model,
		Messages:    messages,
		Temperature: temperature,
		MaxTokens:   maxTokens,
	}

	body, err := json.Marshal(openaiReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("openai returned status %d: %s", resp.StatusCode, string(body))
	}

	var result openAIResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(result.Choices) == 0 {
		return nil, fmt.Errorf("no completion choices returned")
	}

	return &types.LLMResponse{
		Content:      result.Choices[0].Message.Content,
		Model:        model,
		TokensUsed:   result.Usage.TotalTokens,
		FinishReason: result.Choices[0].FinishReason,
	}, nil
}

// CompleteJSON sends a completion request and parses JSON response
func (c *OpenAIClient) CompleteJSON(ctx context.Context, req *types.LLMRequest, result interface{}) error {
	req.UserPrompt = req.UserPrompt + "\n\nRespond with valid JSON only."

	resp, err := c.Complete(ctx, req)
	if err != nil {
		return err
	}

	content := strings.TrimSpace(resp.Content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)

	return json.Unmarshal([]byte(content), result)
}

// Name returns the provider name
func (c *OpenAIClient) Name() string {
	return "openai"
}

// Model returns the model name
func (c *OpenAIClient) Model() string {
	return c.model
}

// Health checks if the API is available
func (c *OpenAIClient) Health(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/models", nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

// AnthropicClient is an LLM client for Anthropic Claude
type AnthropicClient struct {
	baseURL     string
	apiKey      string
	model       string
	client      *http.Client
	temperature float64
	maxTokens   int
}

// NewAnthropicClient creates a new Anthropic client
func NewAnthropicClient(baseURL, apiKey, model string, timeout time.Duration, temperature float64, maxTokens int) *AnthropicClient {
	if baseURL == "" {
		baseURL = "https://api.anthropic.com/v1"
	}
	if model == "" {
		model = "claude-3-sonnet-20240229"
	}
	return &AnthropicClient{
		baseURL:     baseURL,
		apiKey:      apiKey,
		model:       model,
		client:      &http.Client{Timeout: timeout},
		temperature: temperature,
		maxTokens:   maxTokens,
	}
}

type anthropicRequest struct {
	Model       string            `json:"model"`
	MaxTokens   int               `json:"max_tokens"`
	System      string            `json:"system,omitempty"`
	Messages    []anthropicMessage `json:"messages"`
	Temperature float64           `json:"temperature,omitempty"`
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicResponse struct {
	Content []struct {
		Text string `json:"text"`
	} `json:"content"`
	StopReason string `json:"stop_reason"`
	Usage      struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

// Complete sends a completion request to Anthropic
func (c *AnthropicClient) Complete(ctx context.Context, req *types.LLMRequest) (*types.LLMResponse, error) {
	model := req.Model
	if model == "" {
		model = c.model
	}

	temperature := req.Temperature
	if temperature == 0 {
		temperature = c.temperature
	}
	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = c.maxTokens
	}

	anthropicReq := anthropicRequest{
		Model:       model,
		MaxTokens:   maxTokens,
		System:      req.SystemPrompt,
		Messages:    []anthropicMessage{{Role: "user", Content: req.UserPrompt}},
		Temperature: temperature,
	}

	body, err := json.Marshal(anthropicReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/messages", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", c.apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("anthropic returned status %d: %s", resp.StatusCode, string(body))
	}

	var result anthropicResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(result.Content) == 0 {
		return nil, fmt.Errorf("no content returned")
	}

	return &types.LLMResponse{
		Content:      result.Content[0].Text,
		Model:        model,
		TokensUsed:   result.Usage.InputTokens + result.Usage.OutputTokens,
		FinishReason: result.StopReason,
	}, nil
}

// CompleteJSON sends a completion request and parses JSON response
func (c *AnthropicClient) CompleteJSON(ctx context.Context, req *types.LLMRequest, result interface{}) error {
	req.UserPrompt = req.UserPrompt + "\n\nRespond with valid JSON only."

	resp, err := c.Complete(ctx, req)
	if err != nil {
		return err
	}

	content := strings.TrimSpace(resp.Content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)

	return json.Unmarshal([]byte(content), result)
}

// Name returns the provider name
func (c *AnthropicClient) Name() string {
	return "anthropic"
}

// Model returns the model name
func (c *AnthropicClient) Model() string {
	return c.model
}

// Health checks if the API is available
func (c *AnthropicClient) Health(ctx context.Context) error {
	return nil // Anthropic doesn't have a health endpoint
}

// DeepSeekClient is an LLM client for DeepSeek
type DeepSeekClient struct {
	*OpenAIClient
}

// NewDeepSeekClient creates a new DeepSeek client
func NewDeepSeekClient(baseURL, apiKey, model string, timeout time.Duration, temperature float64, maxTokens int) *DeepSeekClient {
	if baseURL == "" {
		baseURL = "https://api.deepseek.com/v1"
	}
	if model == "" {
		model = "deepseek-coder"
	}
	return &DeepSeekClient{
		OpenAIClient: NewOpenAIClient(baseURL, apiKey, model, timeout, temperature, maxTokens),
	}
}

// Name returns the provider name
func (c *DeepSeekClient) Name() string {
	return "deepseek"
}
