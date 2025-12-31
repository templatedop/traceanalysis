package rag

import (
	"context"
	"testing"
	"time"

	"github.com/traceanalysis/rag-temporal/internal/types"
)

func TestMemoryStore_AddAndGet(t *testing.T) {
	store := NewMemoryStore(NewNoOpEmbedder(768))
	ctx := context.Background()

	doc := &types.RAGDocument{
		ID:      "test-doc-1",
		Type:    "runbook",
		Title:   "High CPU Troubleshooting",
		Content: "Steps to troubleshoot high CPU usage in production systems.",
		Tags:    []string{"cpu", "performance"},
	}

	// Test Add
	err := store.Add(ctx, doc)
	if err != nil {
		t.Fatalf("Failed to add document: %v", err)
	}

	// Test Get
	retrieved, err := store.Get(ctx, doc.ID)
	if err != nil {
		t.Fatalf("Failed to get document: %v", err)
	}

	if retrieved.ID != doc.ID {
		t.Errorf("Expected ID %s, got %s", doc.ID, retrieved.ID)
	}
	if retrieved.Title != doc.Title {
		t.Errorf("Expected Title %s, got %s", doc.Title, retrieved.Title)
	}
	if retrieved.Type != doc.Type {
		t.Errorf("Expected Type %s, got %s", doc.Type, retrieved.Type)
	}
}

func TestMemoryStore_Query(t *testing.T) {
	store := NewMemoryStore(NewNoOpEmbedder(768))
	ctx := context.Background()

	// Add test documents
	docs := []*types.RAGDocument{
		{
			ID:      "doc-1",
			Type:    "runbook",
			Title:   "High CPU Troubleshooting",
			Content: "Steps to troubleshoot high CPU usage.",
			Tags:    []string{"cpu", "performance"},
		},
		{
			ID:      "doc-2",
			Type:    "runbook",
			Title:   "Memory Leak Investigation",
			Content: "How to investigate memory leaks in Go applications.",
			Tags:    []string{"memory", "golang"},
		},
		{
			ID:      "doc-3",
			Type:    "incident",
			Title:   "API Outage 2024-01",
			Content: "Post-mortem for API outage due to database issues.",
			Tags:    []string{"api", "database"},
		},
	}

	for _, doc := range docs {
		if err := store.Add(ctx, doc); err != nil {
			t.Fatalf("Failed to add document: %v", err)
		}
	}

	// Test Query with type filter
	result, err := store.Query(ctx, &types.RAGQuery{
		Query: "cpu troubleshooting",
		TopK:  5,
		Types: []string{"runbook"},
	})
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	// Should only return runbooks
	for _, doc := range result.Documents {
		if doc.Type != "runbook" {
			t.Errorf("Expected type 'runbook', got '%s'", doc.Type)
		}
	}

	// Test Query with tag filter
	result, err = store.Query(ctx, &types.RAGQuery{
		Query: "issues",
		TopK:  5,
		Tags:  []string{"database"},
	})
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	if len(result.Documents) == 0 {
		t.Error("Expected at least one document with 'database' tag")
	}
}

func TestMemoryStore_Delete(t *testing.T) {
	store := NewMemoryStore(NewNoOpEmbedder(768))
	ctx := context.Background()

	doc := &types.RAGDocument{
		ID:      "test-doc",
		Type:    "runbook",
		Title:   "Test Document",
		Content: "Test content",
	}

	// Add document
	if err := store.Add(ctx, doc); err != nil {
		t.Fatalf("Failed to add document: %v", err)
	}

	// Delete document
	if err := store.Delete(ctx, doc.ID); err != nil {
		t.Fatalf("Failed to delete document: %v", err)
	}

	// Try to get deleted document
	_, err := store.Get(ctx, doc.ID)
	if err == nil {
		t.Error("Expected error when getting deleted document")
	}
}

func TestMemoryStore_Count(t *testing.T) {
	store := NewMemoryStore(NewNoOpEmbedder(768))
	ctx := context.Background()

	// Initial count should be 0
	count, err := store.Count(ctx)
	if err != nil {
		t.Fatalf("Failed to count: %v", err)
	}
	if count != 0 {
		t.Errorf("Expected count 0, got %d", count)
	}

	// Add documents
	for i := 0; i < 5; i++ {
		doc := &types.RAGDocument{
			ID:      string(rune('a' + i)),
			Type:    "test",
			Title:   "Test",
			Content: "Content",
		}
		store.Add(ctx, doc)
	}

	count, err = store.Count(ctx)
	if err != nil {
		t.Fatalf("Failed to count: %v", err)
	}
	if count != 5 {
		t.Errorf("Expected count 5, got %d", count)
	}
}

func TestCosineSimilarity(t *testing.T) {
	tests := []struct {
		name     string
		a        []float64
		b        []float64
		expected float64
	}{
		{
			name:     "identical vectors",
			a:        []float64{1, 0, 0},
			b:        []float64{1, 0, 0},
			expected: 1.0,
		},
		{
			name:     "orthogonal vectors",
			a:        []float64{1, 0, 0},
			b:        []float64{0, 1, 0},
			expected: 0.0,
		},
		{
			name:     "opposite vectors",
			a:        []float64{1, 0, 0},
			b:        []float64{-1, 0, 0},
			expected: -1.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cosineSimilarity(tt.a, tt.b)
			if result < tt.expected-0.001 || result > tt.expected+0.001 {
				t.Errorf("Expected %f, got %f", tt.expected, result)
			}
		})
	}
}

func TestKnowledgeBase_ChunkContent(t *testing.T) {
	store := NewMemoryStore(NewNoOpEmbedder(768))
	kb := NewKnowledgeBase(store, nil, "")
	kb.chunkSize = 100
	kb.overlap = 20

	// Short content - no chunking needed
	shortContent := "This is a short piece of content."
	chunks := kb.chunkContent(shortContent)
	if len(chunks) != 1 {
		t.Errorf("Expected 1 chunk for short content, got %d", len(chunks))
	}

	// Long content - should be chunked
	longContent := ""
	for i := 0; i < 50; i++ {
		longContent += "This is a longer piece of content that needs chunking. "
	}
	chunks = kb.chunkContent(longContent)
	if len(chunks) < 2 {
		t.Errorf("Expected multiple chunks for long content, got %d", len(chunks))
	}
}

func TestKnowledgeBase_InferDocumentType(t *testing.T) {
	store := NewMemoryStore(NewNoOpEmbedder(768))
	kb := NewKnowledgeBase(store, nil, "")

	tests := []struct {
		path     string
		expected string
	}{
		{"/docs/runbooks/high-cpu.md", "runbook"},
		{"/incidents/api-outage.md", "incident"},
		{"/postmortems/2024-01.md", "incident"},
		{"/architecture/overview.md", "architecture"},
		{"/design/system.md", "architecture"},
		{"/patterns/retry.md", "pattern"},
		{"/docs/other.md", "general"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := kb.inferDocumentType(tt.path)
			if result != tt.expected {
				t.Errorf("Expected %s for path %s, got %s", tt.expected, tt.path, result)
			}
		})
	}
}

func TestNoOpEmbedder(t *testing.T) {
	embedder := NewNoOpEmbedder(768)
	ctx := context.Background()

	// Test dimension
	if embedder.Dimension() != 768 {
		t.Errorf("Expected dimension 768, got %d", embedder.Dimension())
	}

	// Test Embed
	embedding, err := embedder.Embed(ctx, "test text")
	if err != nil {
		t.Fatalf("Embed failed: %v", err)
	}
	if len(embedding) != 768 {
		t.Errorf("Expected embedding length 768, got %d", len(embedding))
	}

	// Test EmbedBatch
	embeddings, err := embedder.EmbedBatch(ctx, []string{"text1", "text2", "text3"})
	if err != nil {
		t.Fatalf("EmbedBatch failed: %v", err)
	}
	if len(embeddings) != 3 {
		t.Errorf("Expected 3 embeddings, got %d", len(embeddings))
	}
}

func BenchmarkMemoryStore_Query(b *testing.B) {
	store := NewMemoryStore(NewNoOpEmbedder(768))
	ctx := context.Background()

	// Add 1000 documents
	for i := 0; i < 1000; i++ {
		doc := &types.RAGDocument{
			ID:      string(rune(i)),
			Type:    "test",
			Title:   "Test Document",
			Content: "This is test content for benchmarking the query performance.",
		}
		store.Add(ctx, doc)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		store.Query(ctx, &types.RAGQuery{
			Query: "test content",
			TopK:  10,
		})
	}
}

func BenchmarkCosineSimilarity(b *testing.B) {
	a := make([]float64, 768)
	bb := make([]float64, 768)
	for i := range a {
		a[i] = float64(i) / 768
		bb[i] = float64(768-i) / 768
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cosineSimilarity(a, bb)
	}
}

// Ensure timestamp is set
func TestMemoryStore_Timestamps(t *testing.T) {
	store := NewMemoryStore(NewNoOpEmbedder(768))
	ctx := context.Background()

	before := time.Now()
	doc := &types.RAGDocument{
		ID:      "test-doc",
		Type:    "test",
		Title:   "Test",
		Content: "Content",
	}
	store.Add(ctx, doc)
	after := time.Now()

	retrieved, _ := store.Get(ctx, doc.ID)
	if retrieved.CreatedAt.Before(before) || retrieved.CreatedAt.After(after) {
		t.Error("CreatedAt not set correctly")
	}
	if retrieved.UpdatedAt.Before(before) || retrieved.UpdatedAt.After(after) {
		t.Error("UpdatedAt not set correctly")
	}
}
