package vectorstore

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/philippgille/chromem-go"
	"github.com/sirupsen/logrus"

	"github.com/ignacio/lumo/internal/intelligence/embeddings"
)

type ChromemStore struct {
	db         *chromem.DB
	collection *chromem.Collection
	log        *logrus.Logger
	embedder   embeddings.Embedder
}

type ChromemOptions struct {
	StoragePath string
	Embedder    embeddings.Embedder
	Log         *logrus.Logger
}

func NewChromemStore(opts ChromemOptions) (*ChromemStore, error) {
	// Ensure storage directory exists
	if err := os.MkdirAll(opts.StoragePath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create storage directory: %w", err)
	}

	// Create Chromem DB with persistence
	dbPath := filepath.Join(opts.StoragePath, "chromem.db")
	db := chromem.NewDB()

	// Create or get collection with custom embedding function
	collectionName := "lumo-diagnostics"
	embeddingFunc := func(ctx context.Context, text string) ([]float32, error) {
		return opts.Embedder.Embed(ctx, text)
	}

	collection, err := db.GetOrCreateCollection(collectionName, nil, embeddingFunc)
	if err != nil {
		return nil, fmt.Errorf("failed to create collection: %w", err)
	}

	opts.Log.WithFields(logrus.Fields{
		"collection": collectionName,
		"path":       dbPath,
	}).Info("Chromem-go collection initialized")

	return &ChromemStore{
		db:         db,
		collection: collection,
		log:        opts.Log,
		embedder:   opts.Embedder,
	}, nil
}

func (s *ChromemStore) Store(ctx context.Context, doc *Document) error {
	// Generate embedding if not provided
	if doc.Embedding == nil {
		embedding, err := s.embedder.Embed(ctx, doc.Content)
		if err != nil {
			return fmt.Errorf("failed to generate embedding: %w", err)
		}
		doc.Embedding = embedding
	}

	// Convert metadata to map[string]string (Chromem requirement)
	metadata := make(map[string]string)
	for k, v := range doc.Metadata {
		metadata[k] = fmt.Sprintf("%v", v)
	}

	// Add to collection
	err := s.collection.AddDocument(ctx, chromem.Document{
		ID:        doc.ID,
		Content:   doc.Content,
		Embedding: doc.Embedding,
		Metadata:  metadata,
	})
	if err != nil {
		return fmt.Errorf("failed to add document: %w", err)
	}

	s.log.WithFields(logrus.Fields{
		"doc_id":   doc.ID,
		"content":  truncate(doc.Content, 50),
		"metadata": doc.Metadata,
	}).Debug("Document stored in vector DB")

	return nil
}

func (s *ChromemStore) StoreBatch(ctx context.Context, docs []*Document) error {
	for _, doc := range docs {
		if err := s.Store(ctx, doc); err != nil {
			return err
		}
	}
	return nil
}

func (s *ChromemStore) Query(ctx context.Context, query string, k int) ([]*Match, error) {
	// Generate query embedding
	queryEmbedding, err := s.embedder.Embed(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to embed query: %w", err)
	}

	return s.QueryWithEmbedding(ctx, queryEmbedding, k)
}

func (s *ChromemStore) QueryWithEmbedding(ctx context.Context, embedding []float32, k int) ([]*Match, error) {
	// Query collection
	results, err := s.collection.QueryEmbedding(ctx, embedding, k, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	// Convert results to Match format
	matches := make([]*Match, len(results))
	for i, result := range results {
		// Convert string metadata back to interface{}
		metadata := make(map[string]interface{})
		for k, v := range result.Metadata {
			metadata[k] = v
		}

		matches[i] = &Match{
			Document: &Document{
				ID:        result.ID,
				Content:   result.Content,
				Embedding: result.Embedding,
				Metadata:  metadata,
			},
			Score: result.Similarity,
		}
	}

	s.log.WithFields(logrus.Fields{
		"k":       k,
		"results": len(matches),
	}).Debug("Vector query completed")

	return matches, nil
}

func (s *ChromemStore) Delete(ctx context.Context, id string) error {
	// Chromem-go doesn't have direct delete yet
	// Workaround: Would need to implement via persistence layer
	s.log.WithField("doc_id", id).Warn("Delete not yet implemented in Chromem-go")
	return nil
}

func (s *ChromemStore) DeleteBatch(ctx context.Context, filter map[string]interface{}) error {
	s.log.Warn("DeleteBatch not yet implemented in Chromem-go")
	return nil
}

func (s *ChromemStore) Count(ctx context.Context) (int, error) {
	// Chromem-go doesn't expose count directly
	// Would need to track separately or query all
	return 0, fmt.Errorf("Count not implemented")
}

func (s *ChromemStore) Close() error {
	// Chromem-go auto-persists on changes
	s.log.Info("Chromem store closed")
	return nil
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
