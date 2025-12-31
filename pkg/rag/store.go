// Package rag provides the RAG (Retrieval-Augmented Generation) system
package rag

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/traceanalysis/rag-temporal/internal/types"
)

// Store is the interface for RAG document storage
type Store interface {
	// Add adds a document to the store
	Add(ctx context.Context, doc *types.RAGDocument) error
	// AddBatch adds multiple documents to the store
	AddBatch(ctx context.Context, docs []*types.RAGDocument) error
	// Query retrieves relevant documents for a query
	Query(ctx context.Context, query *types.RAGQuery) (*types.RAGResult, error)
	// Get retrieves a document by ID
	Get(ctx context.Context, id string) (*types.RAGDocument, error)
	// Delete removes a document by ID
	Delete(ctx context.Context, id string) error
	// Update updates an existing document
	Update(ctx context.Context, doc *types.RAGDocument) error
	// List lists all documents with optional filtering
	List(ctx context.Context, docType string, limit int) ([]*types.RAGDocument, error)
	// Count returns the total number of documents
	Count(ctx context.Context) (int, error)
	// Clear removes all documents
	Clear(ctx context.Context) error
}

// EmbeddingProvider generates embeddings for text
type EmbeddingProvider interface {
	// Embed generates an embedding for the given text
	Embed(ctx context.Context, text string) ([]float64, error)
	// EmbedBatch generates embeddings for multiple texts
	EmbedBatch(ctx context.Context, texts []string) ([][]float64, error)
	// Dimension returns the embedding dimension
	Dimension() int
}

// MemoryStore is an in-memory implementation of the RAG store
type MemoryStore struct {
	documents map[string]*types.RAGDocument
	embedder  EmbeddingProvider
	mu        sync.RWMutex
}

// NewMemoryStore creates a new in-memory RAG store
func NewMemoryStore(embedder EmbeddingProvider) *MemoryStore {
	return &MemoryStore{
		documents: make(map[string]*types.RAGDocument),
		embedder:  embedder,
	}
}

// Add adds a document to the store
func (s *MemoryStore) Add(ctx context.Context, doc *types.RAGDocument) error {
	if doc.ID == "" {
		doc.ID = generateID()
	}

	// Generate embedding if not present
	if len(doc.Embedding) == 0 && s.embedder != nil {
		embedding, err := s.embedder.Embed(ctx, doc.Content)
		if err != nil {
			return fmt.Errorf("failed to generate embedding: %w", err)
		}
		doc.Embedding = embedding
	}

	doc.CreatedAt = time.Now()
	doc.UpdatedAt = doc.CreatedAt

	s.mu.Lock()
	defer s.mu.Unlock()
	s.documents[doc.ID] = doc
	return nil
}

// AddBatch adds multiple documents to the store
func (s *MemoryStore) AddBatch(ctx context.Context, docs []*types.RAGDocument) error {
	for _, doc := range docs {
		if err := s.Add(ctx, doc); err != nil {
			return err
		}
	}
	return nil
}

// Query retrieves relevant documents for a query
func (s *MemoryStore) Query(ctx context.Context, query *types.RAGQuery) (*types.RAGResult, error) {
	if query.TopK <= 0 {
		query.TopK = 5
	}

	// Generate query embedding
	var queryEmbedding []float64
	if s.embedder != nil {
		var err error
		queryEmbedding, err = s.embedder.Embed(ctx, query.Query)
		if err != nil {
			return nil, fmt.Errorf("failed to generate query embedding: %w", err)
		}
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	type scoredDoc struct {
		doc   *types.RAGDocument
		score float64
	}

	var scored []scoredDoc

	for _, doc := range s.documents {
		// Filter by type if specified
		if len(query.Types) > 0 {
			matched := false
			for _, t := range query.Types {
				if doc.Type == t {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}

		// Filter by tags if specified
		if len(query.Tags) > 0 {
			matched := false
			for _, qt := range query.Tags {
				for _, dt := range doc.Tags {
					if qt == dt {
						matched = true
						break
					}
				}
				if matched {
					break
				}
			}
			if !matched {
				continue
			}
		}

		// Calculate similarity score
		var score float64
		if len(queryEmbedding) > 0 && len(doc.Embedding) > 0 {
			score = cosineSimilarity(queryEmbedding, doc.Embedding)
		} else {
			// Fallback to keyword matching
			score = keywordSimilarity(query.Query, doc.Content+doc.Title)
		}

		if query.MinScore > 0 && score < query.MinScore {
			continue
		}

		scored = append(scored, scoredDoc{doc: doc, score: score})
	}

	// Sort by score descending
	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	// Take top K
	if len(scored) > query.TopK {
		scored = scored[:query.TopK]
	}

	docs := make([]types.RAGDocument, len(scored))
	for i, sd := range scored {
		docCopy := *sd.doc
		docs[i] = docCopy
	}

	return &types.RAGResult{
		Documents:  docs,
		Query:      query.Query,
		TotalFound: len(scored),
	}, nil
}

// Get retrieves a document by ID
func (s *MemoryStore) Get(ctx context.Context, id string) (*types.RAGDocument, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	doc, ok := s.documents[id]
	if !ok {
		return nil, fmt.Errorf("document not found: %s", id)
	}
	return doc, nil
}

// Delete removes a document by ID
func (s *MemoryStore) Delete(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.documents[id]; !ok {
		return fmt.Errorf("document not found: %s", id)
	}
	delete(s.documents, id)
	return nil
}

// Update updates an existing document
func (s *MemoryStore) Update(ctx context.Context, doc *types.RAGDocument) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.documents[doc.ID]; !ok {
		return fmt.Errorf("document not found: %s", doc.ID)
	}

	// Regenerate embedding if content changed
	if s.embedder != nil {
		embedding, err := s.embedder.Embed(ctx, doc.Content)
		if err != nil {
			return fmt.Errorf("failed to generate embedding: %w", err)
		}
		doc.Embedding = embedding
	}

	doc.UpdatedAt = time.Now()
	s.documents[doc.ID] = doc
	return nil
}

// List lists all documents with optional filtering
func (s *MemoryStore) List(ctx context.Context, docType string, limit int) ([]*types.RAGDocument, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var docs []*types.RAGDocument
	for _, doc := range s.documents {
		if docType != "" && doc.Type != docType {
			continue
		}
		docs = append(docs, doc)
		if limit > 0 && len(docs) >= limit {
			break
		}
	}
	return docs, nil
}

// Count returns the total number of documents
func (s *MemoryStore) Count(ctx context.Context) (int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.documents), nil
}

// Clear removes all documents
func (s *MemoryStore) Clear(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.documents = make(map[string]*types.RAGDocument)
	return nil
}

// cosineSimilarity calculates the cosine similarity between two vectors
func cosineSimilarity(a, b []float64) float64 {
	if len(a) != len(b) {
		return 0
	}

	var dotProduct, normA, normB float64
	for i := range a {
		dotProduct += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return dotProduct / (math.Sqrt(normA) * math.Sqrt(normB))
}

// keywordSimilarity calculates a simple keyword-based similarity score
func keywordSimilarity(query, content string) float64 {
	queryWords := strings.Fields(strings.ToLower(query))
	contentLower := strings.ToLower(content)

	if len(queryWords) == 0 {
		return 0
	}

	matches := 0
	for _, word := range queryWords {
		if strings.Contains(contentLower, word) {
			matches++
		}
	}

	return float64(matches) / float64(len(queryWords))
}

// generateID generates a unique document ID
func generateID() string {
	return fmt.Sprintf("doc_%d", time.Now().UnixNano())
}

// KnowledgeBase manages the RAG knowledge base
type KnowledgeBase struct {
	store     Store
	embedder  EmbeddingProvider
	basePath  string
	chunkSize int
	overlap   int
}

// NewKnowledgeBase creates a new knowledge base
func NewKnowledgeBase(store Store, embedder EmbeddingProvider, basePath string) *KnowledgeBase {
	return &KnowledgeBase{
		store:     store,
		embedder:  embedder,
		basePath:  basePath,
		chunkSize: 1000,
		overlap:   200,
	}
}

// LoadFromDirectory loads documents from a directory
func (kb *KnowledgeBase) LoadFromDirectory(ctx context.Context, dir string) error {
	return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".md" && ext != ".txt" && ext != ".json" && ext != ".yaml" && ext != ".yml" {
			return nil
		}

		return kb.LoadFile(ctx, path)
	})
}

// LoadFile loads a single file into the knowledge base
func (kb *KnowledgeBase) LoadFile(ctx context.Context, path string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read file %s: %w", path, err)
	}

	// Determine document type from path or content
	docType := kb.inferDocumentType(path)

	// Extract metadata
	metadata := map[string]string{
		"source": path,
		"type":   docType,
	}

	// Chunk the content if it's large
	chunks := kb.chunkContent(string(content))

	for i, chunk := range chunks {
		doc := &types.RAGDocument{
			ID:      fmt.Sprintf("%s_chunk_%d", filepath.Base(path), i),
			Type:    docType,
			Title:   fmt.Sprintf("%s (part %d)", filepath.Base(path), i+1),
			Content: chunk,
			Metadata: metadata,
			Tags:    kb.extractTags(path, chunk),
		}

		if err := kb.store.Add(ctx, doc); err != nil {
			return fmt.Errorf("failed to add document: %w", err)
		}
	}

	return nil
}

// inferDocumentType infers the document type from the path
func (kb *KnowledgeBase) inferDocumentType(path string) string {
	pathLower := strings.ToLower(path)

	if strings.Contains(pathLower, "runbook") {
		return "runbook"
	}
	if strings.Contains(pathLower, "incident") || strings.Contains(pathLower, "postmortem") {
		return "incident"
	}
	if strings.Contains(pathLower, "architecture") || strings.Contains(pathLower, "design") {
		return "architecture"
	}
	if strings.Contains(pathLower, "pattern") {
		return "pattern"
	}
	return "general"
}

// extractTags extracts tags from path and content
func (kb *KnowledgeBase) extractTags(path, content string) []string {
	var tags []string

	// Extract from path
	pathParts := strings.Split(strings.ToLower(path), string(os.PathSeparator))
	for _, part := range pathParts {
		if part != "" && part != "." && part != ".." {
			tags = append(tags, part)
		}
	}

	// Extract common observability terms
	terms := []string{
		"cpu", "memory", "disk", "network", "latency", "error", "timeout",
		"database", "cache", "queue", "api", "http", "grpc",
		"kubernetes", "docker", "container", "pod", "node",
		"gc", "jvm", "heap", "oom", "deadlock", "leak",
	}

	contentLower := strings.ToLower(content)
	for _, term := range terms {
		if strings.Contains(contentLower, term) {
			tags = append(tags, term)
		}
	}

	return uniqueStrings(tags)
}

// chunkContent splits content into smaller chunks
func (kb *KnowledgeBase) chunkContent(content string) []string {
	if len(content) <= kb.chunkSize {
		return []string{content}
	}

	var chunks []string
	words := strings.Fields(content)
	currentChunk := strings.Builder{}
	wordCount := 0

	for _, word := range words {
		if currentChunk.Len()+len(word)+1 > kb.chunkSize && currentChunk.Len() > 0 {
			chunks = append(chunks, currentChunk.String())

			// Keep overlap
			overlapWords := words[max(0, wordCount-kb.overlap/10) : wordCount]
			currentChunk.Reset()
			for _, ow := range overlapWords {
				currentChunk.WriteString(ow)
				currentChunk.WriteString(" ")
			}
		}

		if currentChunk.Len() > 0 {
			currentChunk.WriteString(" ")
		}
		currentChunk.WriteString(word)
		wordCount++
	}

	if currentChunk.Len() > 0 {
		chunks = append(chunks, currentChunk.String())
	}

	return chunks
}

// LoadIncident loads a structured incident document
func (kb *KnowledgeBase) LoadIncident(ctx context.Context, incident *types.PastIncident) error {
	content, err := json.Marshal(incident)
	if err != nil {
		return err
	}

	doc := &types.RAGDocument{
		ID:      fmt.Sprintf("incident_%s", incident.ID),
		Type:    "incident",
		Title:   incident.Title,
		Content: string(content),
		Tags:    append(incident.AffectedServices, incident.Symptoms...),
		Metadata: map[string]string{
			"severity":   string(incident.Severity),
			"date":       incident.Date.Format(time.RFC3339),
			"root_cause": incident.RootCause,
		},
	}

	return kb.store.Add(ctx, doc)
}

// Search searches the knowledge base
func (kb *KnowledgeBase) Search(ctx context.Context, query string, topK int) (*types.RAGResult, error) {
	return kb.store.Query(ctx, &types.RAGQuery{
		Query: query,
		TopK:  topK,
	})
}

// SearchByType searches the knowledge base filtered by document type
func (kb *KnowledgeBase) SearchByType(ctx context.Context, query string, docType string, topK int) (*types.RAGResult, error) {
	return kb.store.Query(ctx, &types.RAGQuery{
		Query: query,
		TopK:  topK,
		Types: []string{docType},
	})
}

// uniqueStrings returns unique strings from a slice
func uniqueStrings(strs []string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, s := range strs {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	return result
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
