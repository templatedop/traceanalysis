// Package rag provides Qdrant vector store implementation
package rag

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/traceanalysis/rag-temporal/internal/config"
	"github.com/traceanalysis/rag-temporal/internal/types"
)

// QdrantStore is a Qdrant implementation of the RAG store
type QdrantStore struct {
	baseURL    string
	collection string
	embedder   EmbeddingProvider
	client     *http.Client
}

// NewQdrantStore creates a new Qdrant store
func NewQdrantStore(cfg *config.VectorStoreConfig, embedder EmbeddingProvider) (*QdrantStore, error) {
	baseURL := fmt.Sprintf("http://%s:%d", cfg.Host, cfg.Port)
	if cfg.Host == "" {
		baseURL = "http://localhost:6333"
	}

	store := &QdrantStore{
		baseURL:    baseURL,
		collection: cfg.Collection,
		embedder:   embedder,
		client:     &http.Client{Timeout: 30 * time.Second},
	}

	// Ensure collection exists
	if err := store.ensureCollection(context.Background(), cfg.Dimension); err != nil {
		return nil, fmt.Errorf("failed to ensure collection: %w", err)
	}

	return store, nil
}

// Qdrant API types

type qdrantCreateCollection struct {
	Vectors qdrantVectorConfig `json:"vectors"`
}

type qdrantVectorConfig struct {
	Size     int    `json:"size"`
	Distance string `json:"distance"`
}

type qdrantPoint struct {
	ID      string                 `json:"id"`
	Vector  []float64              `json:"vector"`
	Payload map[string]interface{} `json:"payload"`
}

type qdrantUpsertRequest struct {
	Points []qdrantPoint `json:"points"`
}

type qdrantSearchRequest struct {
	Vector      []float64              `json:"vector"`
	Limit       int                    `json:"limit"`
	WithPayload bool                   `json:"with_payload"`
	Filter      *qdrantFilter          `json:"filter,omitempty"`
	ScoreThreshold float64             `json:"score_threshold,omitempty"`
}

type qdrantFilter struct {
	Must   []qdrantCondition `json:"must,omitempty"`
	Should []qdrantCondition `json:"should,omitempty"`
}

type qdrantCondition struct {
	Key   string                 `json:"key"`
	Match map[string]interface{} `json:"match,omitempty"`
}

type qdrantSearchResponse struct {
	Result []qdrantSearchResult `json:"result"`
}

type qdrantSearchResult struct {
	ID      string                 `json:"id"`
	Score   float64                `json:"score"`
	Payload map[string]interface{} `json:"payload"`
}

type qdrantGetResponse struct {
	Result []qdrantPoint `json:"result"`
}

type qdrantCountResponse struct {
	Result struct {
		Count int `json:"count"`
	} `json:"result"`
}

func (s *QdrantStore) ensureCollection(ctx context.Context, dimension int) error {
	// Check if collection exists
	req, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("%s/collections/%s", s.baseURL, s.collection), nil)
	if err != nil {
		return err
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		return nil // Collection exists
	}

	// Create collection
	createReq := qdrantCreateCollection{
		Vectors: qdrantVectorConfig{
			Size:     dimension,
			Distance: "Cosine",
		},
	}

	body, err := json.Marshal(createReq)
	if err != nil {
		return err
	}

	req, err = http.NewRequestWithContext(ctx, "PUT", fmt.Sprintf("%s/collections/%s", s.baseURL, s.collection), bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err = s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to create collection: %s", string(body))
	}

	return nil
}

// Add adds a document to the store
func (s *QdrantStore) Add(ctx context.Context, doc *types.RAGDocument) error {
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

	point := qdrantPoint{
		ID:     doc.ID,
		Vector: doc.Embedding,
		Payload: map[string]interface{}{
			"type":       doc.Type,
			"title":      doc.Title,
			"content":    doc.Content,
			"tags":       doc.Tags,
			"metadata":   doc.Metadata,
			"created_at": doc.CreatedAt.Format(time.RFC3339),
			"updated_at": doc.UpdatedAt.Format(time.RFC3339),
		},
	}

	upsertReq := qdrantUpsertRequest{
		Points: []qdrantPoint{point},
	}

	body, err := json.Marshal(upsertReq)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "PUT", fmt.Sprintf("%s/collections/%s/points", s.baseURL, s.collection), bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to add document: %s", string(body))
	}

	return nil
}

// AddBatch adds multiple documents to the store
func (s *QdrantStore) AddBatch(ctx context.Context, docs []*types.RAGDocument) error {
	points := make([]qdrantPoint, 0, len(docs))

	for _, doc := range docs {
		if doc.ID == "" {
			doc.ID = generateID()
		}

		if len(doc.Embedding) == 0 && s.embedder != nil {
			embedding, err := s.embedder.Embed(ctx, doc.Content)
			if err != nil {
				return fmt.Errorf("failed to generate embedding: %w", err)
			}
			doc.Embedding = embedding
		}

		doc.CreatedAt = time.Now()
		doc.UpdatedAt = doc.CreatedAt

		points = append(points, qdrantPoint{
			ID:     doc.ID,
			Vector: doc.Embedding,
			Payload: map[string]interface{}{
				"type":       doc.Type,
				"title":      doc.Title,
				"content":    doc.Content,
				"tags":       doc.Tags,
				"metadata":   doc.Metadata,
				"created_at": doc.CreatedAt.Format(time.RFC3339),
				"updated_at": doc.UpdatedAt.Format(time.RFC3339),
			},
		})
	}

	upsertReq := qdrantUpsertRequest{Points: points}
	body, err := json.Marshal(upsertReq)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "PUT", fmt.Sprintf("%s/collections/%s/points", s.baseURL, s.collection), bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to add documents: %s", string(body))
	}

	return nil
}

// Query retrieves relevant documents for a query
func (s *QdrantStore) Query(ctx context.Context, query *types.RAGQuery) (*types.RAGResult, error) {
	if query.TopK <= 0 {
		query.TopK = 5
	}

	// Generate query embedding
	queryEmbedding, err := s.embedder.Embed(ctx, query.Query)
	if err != nil {
		return nil, fmt.Errorf("failed to generate query embedding: %w", err)
	}

	searchReq := qdrantSearchRequest{
		Vector:         queryEmbedding,
		Limit:          query.TopK,
		WithPayload:    true,
		ScoreThreshold: query.MinScore,
	}

	// Build filter
	if len(query.Types) > 0 {
		conditions := make([]qdrantCondition, 0, len(query.Types))
		for _, t := range query.Types {
			conditions = append(conditions, qdrantCondition{
				Key:   "type",
				Match: map[string]interface{}{"value": t},
			})
		}
		searchReq.Filter = &qdrantFilter{Should: conditions}
	}

	body, err := json.Marshal(searchReq)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", fmt.Sprintf("%s/collections/%s/points/search", s.baseURL, s.collection), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("search failed: %s", string(body))
	}

	var searchResp qdrantSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		return nil, err
	}

	docs := make([]types.RAGDocument, 0, len(searchResp.Result))
	for _, result := range searchResp.Result {
		doc := s.payloadToDocument(result.ID, result.Payload)
		docs = append(docs, doc)
	}

	return &types.RAGResult{
		Documents:  docs,
		Query:      query.Query,
		TotalFound: len(docs),
	}, nil
}

// Get retrieves a document by ID
func (s *QdrantStore) Get(ctx context.Context, id string) (*types.RAGDocument, error) {
	body, err := json.Marshal(map[string]interface{}{
		"ids":          []string{id},
		"with_payload": true,
	})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", fmt.Sprintf("%s/collections/%s/points", s.baseURL, s.collection), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("document not found: %s", id)
	}

	var getResp qdrantGetResponse
	if err := json.NewDecoder(resp.Body).Decode(&getResp); err != nil {
		return nil, err
	}

	if len(getResp.Result) == 0 {
		return nil, fmt.Errorf("document not found: %s", id)
	}

	doc := s.payloadToDocument(getResp.Result[0].ID, getResp.Result[0].Payload)
	return &doc, nil
}

// Delete removes a document by ID
func (s *QdrantStore) Delete(ctx context.Context, id string) error {
	body, err := json.Marshal(map[string]interface{}{
		"points": []string{id},
	})
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", fmt.Sprintf("%s/collections/%s/points/delete", s.baseURL, s.collection), bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to delete document: %s", string(body))
	}

	return nil
}

// Update updates an existing document
func (s *QdrantStore) Update(ctx context.Context, doc *types.RAGDocument) error {
	// Generate new embedding
	if s.embedder != nil {
		embedding, err := s.embedder.Embed(ctx, doc.Content)
		if err != nil {
			return fmt.Errorf("failed to generate embedding: %w", err)
		}
		doc.Embedding = embedding
	}

	doc.UpdatedAt = time.Now()

	return s.Add(ctx, doc)
}

// List lists all documents with optional filtering
func (s *QdrantStore) List(ctx context.Context, docType string, limit int) ([]*types.RAGDocument, error) {
	if limit <= 0 {
		limit = 100
	}

	scrollReq := map[string]interface{}{
		"limit":        limit,
		"with_payload": true,
	}

	if docType != "" {
		scrollReq["filter"] = qdrantFilter{
			Must: []qdrantCondition{
				{Key: "type", Match: map[string]interface{}{"value": docType}},
			},
		}
	}

	body, err := json.Marshal(scrollReq)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", fmt.Sprintf("%s/collections/%s/points/scroll", s.baseURL, s.collection), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var scrollResp struct {
		Result struct {
			Points []qdrantPoint `json:"points"`
		} `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&scrollResp); err != nil {
		return nil, err
	}

	docs := make([]*types.RAGDocument, 0, len(scrollResp.Result.Points))
	for _, point := range scrollResp.Result.Points {
		doc := s.payloadToDocument(point.ID, point.Payload)
		docs = append(docs, &doc)
	}

	return docs, nil
}

// Count returns the total number of documents
func (s *QdrantStore) Count(ctx context.Context) (int, error) {
	req, err := http.NewRequestWithContext(ctx, "POST", fmt.Sprintf("%s/collections/%s/points/count", s.baseURL, s.collection), bytes.NewReader([]byte("{}")))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	var countResp qdrantCountResponse
	if err := json.NewDecoder(resp.Body).Decode(&countResp); err != nil {
		return 0, err
	}

	return countResp.Result.Count, nil
}

// Clear removes all documents
func (s *QdrantStore) Clear(ctx context.Context) error {
	// Delete and recreate collection
	req, err := http.NewRequestWithContext(ctx, "DELETE", fmt.Sprintf("%s/collections/%s", s.baseURL, s.collection), nil)
	if err != nil {
		return err
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()

	// Recreate collection
	dimension := 768
	if s.embedder != nil {
		dimension = s.embedder.Dimension()
	}

	return s.ensureCollection(ctx, dimension)
}

func (s *QdrantStore) payloadToDocument(id string, payload map[string]interface{}) types.RAGDocument {
	doc := types.RAGDocument{ID: id}

	if v, ok := payload["type"].(string); ok {
		doc.Type = v
	}
	if v, ok := payload["title"].(string); ok {
		doc.Title = v
	}
	if v, ok := payload["content"].(string); ok {
		doc.Content = v
	}
	if v, ok := payload["tags"].([]interface{}); ok {
		for _, tag := range v {
			if t, ok := tag.(string); ok {
				doc.Tags = append(doc.Tags, t)
			}
		}
	}
	if v, ok := payload["metadata"].(map[string]interface{}); ok {
		doc.Metadata = make(map[string]string)
		for k, val := range v {
			if s, ok := val.(string); ok {
				doc.Metadata[k] = s
			}
		}
	}
	if v, ok := payload["created_at"].(string); ok {
		doc.CreatedAt, _ = time.Parse(time.RFC3339, v)
	}
	if v, ok := payload["updated_at"].(string); ok {
		doc.UpdatedAt, _ = time.Parse(time.RFC3339, v)
	}

	return doc
}
