// Package agents provides independent analysis agents for observability data
package agents

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/traceanalysis/rag-temporal/internal/types"
	"github.com/traceanalysis/rag-temporal/pkg/llm"
	"github.com/traceanalysis/rag-temporal/pkg/rag"
)

// BaseAgent provides common functionality for all agents
type BaseAgent struct {
	name       string
	agentType  types.AnalysisType
	llmClient  llm.Client
	ragStore   rag.Store
	mu         sync.RWMutex
	lastHealth time.Time
	healthy    bool
}

// NewBaseAgent creates a new base agent
func NewBaseAgent(name string, agentType types.AnalysisType, llmClient llm.Client, ragStore rag.Store) *BaseAgent {
	return &BaseAgent{
		name:      name,
		agentType: agentType,
		llmClient: llmClient,
		ragStore:  ragStore,
		healthy:   true,
	}
}

// Name returns the agent name
func (a *BaseAgent) Name() string {
	return a.name
}

// Type returns the analysis type
func (a *BaseAgent) Type() types.AnalysisType {
	return a.agentType
}

// Health checks if the agent is healthy
func (a *BaseAgent) Health(ctx context.Context) error {
	a.mu.RLock()
	if time.Since(a.lastHealth) < 30*time.Second && a.healthy {
		a.mu.RUnlock()
		return nil
	}
	a.mu.RUnlock()

	// Check LLM health
	if err := a.llmClient.Health(ctx); err != nil {
		a.setHealth(false)
		return fmt.Errorf("llm unhealthy: %w", err)
	}

	a.setHealth(true)
	return nil
}

func (a *BaseAgent) setHealth(healthy bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.healthy = healthy
	a.lastHealth = time.Now()
}

// GetRAGContext retrieves relevant context from the RAG store
func (a *BaseAgent) GetRAGContext(ctx context.Context, query string, topK int) (string, error) {
	if a.ragStore == nil {
		return "", nil
	}

	result, err := a.ragStore.Query(ctx, &types.RAGQuery{
		Query: query,
		TopK:  topK,
	})
	if err != nil {
		return "", err
	}

	var context string
	for _, doc := range result.Documents {
		context += fmt.Sprintf("--- %s (%s) ---\n%s\n\n", doc.Title, doc.Type, doc.Content)
	}

	return context, nil
}

// CreateFinding creates a new finding with default values
func CreateFinding(findingType types.FindingType, severity types.Severity, title, description string) types.Finding {
	return types.Finding{
		ID:          fmt.Sprintf("finding_%d", time.Now().UnixNano()),
		Type:        findingType,
		Severity:    severity,
		Title:       title,
		Description: description,
		Timestamp:   time.Now(),
	}
}

// CreatePrediction creates a new prediction
func CreatePrediction(metric string, current, predicted float64, confidence float64, description string) types.Prediction {
	return types.Prediction{
		ID:             fmt.Sprintf("prediction_%d", time.Now().UnixNano()),
		Metric:         metric,
		CurrentValue:   current,
		PredictedValue: predicted,
		Confidence:     confidence,
		Description:    description,
		Timestamp:      time.Now(),
	}
}

// AgentRegistry manages registered agents
type AgentRegistry struct {
	agents map[string]types.Agent
	mu     sync.RWMutex
}

// NewAgentRegistry creates a new agent registry
func NewAgentRegistry() *AgentRegistry {
	return &AgentRegistry{
		agents: make(map[string]types.Agent),
	}
}

// Register registers an agent
func (r *AgentRegistry) Register(agent types.Agent) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.agents[agent.Name()] = agent
}

// Get returns an agent by name
func (r *AgentRegistry) Get(name string) (types.Agent, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	agent, ok := r.agents[name]
	return agent, ok
}

// GetByType returns agents matching the analysis type
func (r *AgentRegistry) GetByType(t types.AnalysisType) []types.Agent {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []types.Agent
	for _, agent := range r.agents {
		if agent.Type() == t || t == types.AnalysisTypeAll {
			result = append(result, agent)
		}
	}
	return result
}

// All returns all registered agents
func (r *AgentRegistry) All() []types.Agent {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]types.Agent, 0, len(r.agents))
	for _, agent := range r.agents {
		result = append(result, agent)
	}
	return result
}

// HealthCheck checks the health of all agents
func (r *AgentRegistry) HealthCheck(ctx context.Context) map[string]error {
	r.mu.RLock()
	agents := make([]types.Agent, 0, len(r.agents))
	for _, agent := range r.agents {
		agents = append(agents, agent)
	}
	r.mu.RUnlock()

	results := make(map[string]error)
	var wg sync.WaitGroup
	var mu sync.Mutex

	for _, agent := range agents {
		wg.Add(1)
		go func(a types.Agent) {
			defer wg.Done()
			err := a.Health(ctx)
			mu.Lock()
			results[a.Name()] = err
			mu.Unlock()
		}(agent)
	}

	wg.Wait()
	return results
}
