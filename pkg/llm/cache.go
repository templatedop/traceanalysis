// Package llm provides caching for LLM responses
package llm

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sync"
	"time"

	"github.com/traceanalysis/rag-temporal/internal/types"
)

// CachedClient wraps an LLM client with caching
type CachedClient struct {
	client Client
	cache  Cache
	ttl    time.Duration
}

// Cache is the interface for LLM response caching
type Cache interface {
	Get(key string) (*types.LLMResponse, bool)
	Set(key string, response *types.LLMResponse, ttl time.Duration)
	Delete(key string)
	Clear()
	Size() int
}

// NewCachedClient creates a new cached LLM client
func NewCachedClient(client Client, cache Cache, ttl time.Duration) *CachedClient {
	if ttl == 0 {
		ttl = 5 * time.Minute
	}
	return &CachedClient{
		client: client,
		cache:  cache,
		ttl:    ttl,
	}
}

// Complete sends a completion request, checking cache first
func (c *CachedClient) Complete(ctx context.Context, req *types.LLMRequest) (*types.LLMResponse, error) {
	// Generate cache key
	key := c.generateKey(req)

	// Check cache
	if cached, ok := c.cache.Get(key); ok {
		return cached, nil
	}

	// Call underlying client
	resp, err := c.client.Complete(ctx, req)
	if err != nil {
		return nil, err
	}

	// Cache response
	c.cache.Set(key, resp, c.ttl)

	return resp, nil
}

// CompleteJSON sends a completion request and parses JSON, checking cache first
func (c *CachedClient) CompleteJSON(ctx context.Context, req *types.LLMRequest, result interface{}) error {
	resp, err := c.Complete(ctx, req)
	if err != nil {
		return err
	}

	// Parse JSON from cached or fresh response
	if resp.Parsed != nil {
		// Re-marshal and unmarshal to get the correct type
		data, err := json.Marshal(resp.Parsed)
		if err != nil {
			return err
		}
		return json.Unmarshal(data, result)
	}

	return json.Unmarshal([]byte(resp.Content), result)
}

// Name returns the provider name
func (c *CachedClient) Name() string {
	return c.client.Name()
}

// Model returns the model name
func (c *CachedClient) Model() string {
	return c.client.Model()
}

// Health checks if the LLM is available
func (c *CachedClient) Health(ctx context.Context) error {
	return c.client.Health(ctx)
}

// CacheStats returns cache statistics
func (c *CachedClient) CacheStats() CacheStats {
	if mc, ok := c.cache.(*MemoryCache); ok {
		return mc.Stats()
	}
	return CacheStats{}
}

// ClearCache clears the cache
func (c *CachedClient) ClearCache() {
	c.cache.Clear()
}

func (c *CachedClient) generateKey(req *types.LLMRequest) string {
	// Create a hash of the request
	data, _ := json.Marshal(struct {
		System      string
		User        string
		Model       string
		Temperature float64
	}{
		System:      req.SystemPrompt,
		User:        req.UserPrompt,
		Model:       req.Model,
		Temperature: req.Temperature,
	})

	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

// MemoryCache is an in-memory LRU cache for LLM responses
type MemoryCache struct {
	entries    map[string]*cacheEntry
	order      []string
	maxSize    int
	mu         sync.RWMutex
	hits       int64
	misses     int64
}

type cacheEntry struct {
	response  *types.LLMResponse
	expiresAt time.Time
}

// CacheStats contains cache statistics
type CacheStats struct {
	Size    int     `json:"size"`
	MaxSize int     `json:"max_size"`
	Hits    int64   `json:"hits"`
	Misses  int64   `json:"misses"`
	HitRate float64 `json:"hit_rate"`
}

// NewMemoryCache creates a new in-memory cache
func NewMemoryCache(maxSize int) *MemoryCache {
	if maxSize <= 0 {
		maxSize = 1000
	}
	return &MemoryCache{
		entries: make(map[string]*cacheEntry),
		order:   make([]string, 0, maxSize),
		maxSize: maxSize,
	}
}

// Get retrieves a response from the cache
func (c *MemoryCache) Get(key string) (*types.LLMResponse, bool) {
	c.mu.RLock()
	entry, ok := c.entries[key]
	c.mu.RUnlock()

	if !ok {
		c.mu.Lock()
		c.misses++
		c.mu.Unlock()
		return nil, false
	}

	// Check expiration
	if time.Now().After(entry.expiresAt) {
		c.Delete(key)
		c.mu.Lock()
		c.misses++
		c.mu.Unlock()
		return nil, false
	}

	c.mu.Lock()
	c.hits++
	// Move to front (most recently used)
	c.moveToFront(key)
	c.mu.Unlock()

	return entry.response, true
}

// Set stores a response in the cache
func (c *MemoryCache) Set(key string, response *types.LLMResponse, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check if already exists
	if _, ok := c.entries[key]; ok {
		c.entries[key] = &cacheEntry{
			response:  response,
			expiresAt: time.Now().Add(ttl),
		}
		c.moveToFront(key)
		return
	}

	// Evict if at capacity
	for len(c.entries) >= c.maxSize && len(c.order) > 0 {
		oldest := c.order[len(c.order)-1]
		c.order = c.order[:len(c.order)-1]
		delete(c.entries, oldest)
	}

	// Add new entry
	c.entries[key] = &cacheEntry{
		response:  response,
		expiresAt: time.Now().Add(ttl),
	}
	c.order = append([]string{key}, c.order...)
}

// Delete removes a response from the cache
func (c *MemoryCache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.entries, key)
	for i, k := range c.order {
		if k == key {
			c.order = append(c.order[:i], c.order[i+1:]...)
			break
		}
	}
}

// Clear removes all entries from the cache
func (c *MemoryCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries = make(map[string]*cacheEntry)
	c.order = make([]string, 0, c.maxSize)
	c.hits = 0
	c.misses = 0
}

// Size returns the number of entries in the cache
func (c *MemoryCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.entries)
}

// Stats returns cache statistics
func (c *MemoryCache) Stats() CacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	total := c.hits + c.misses
	hitRate := 0.0
	if total > 0 {
		hitRate = float64(c.hits) / float64(total)
	}

	return CacheStats{
		Size:    len(c.entries),
		MaxSize: c.maxSize,
		Hits:    c.hits,
		Misses:  c.misses,
		HitRate: hitRate,
	}
}

func (c *MemoryCache) moveToFront(key string) {
	for i, k := range c.order {
		if k == key {
			c.order = append(c.order[:i], c.order[i+1:]...)
			c.order = append([]string{key}, c.order...)
			return
		}
	}
}

// CleanExpired removes expired entries from the cache
func (c *MemoryCache) CleanExpired() int {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	removed := 0

	for key, entry := range c.entries {
		if now.After(entry.expiresAt) {
			delete(c.entries, key)
			removed++
		}
	}

	// Rebuild order slice
	newOrder := make([]string, 0, len(c.entries))
	for _, key := range c.order {
		if _, ok := c.entries[key]; ok {
			newOrder = append(newOrder, key)
		}
	}
	c.order = newOrder

	return removed
}

// StartCleanupRoutine starts a background routine to clean expired entries
func (c *MemoryCache) StartCleanupRoutine(ctx context.Context, interval time.Duration) {
	if interval == 0 {
		interval = time.Minute
	}

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				c.CleanExpired()
			}
		}
	}()
}
