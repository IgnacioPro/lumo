package vectorstore

import (
	"context"
	"time"
)

// VectorStore defines operations for semantic search
type VectorStore interface {
	// Store a document with its embedding
	Store(ctx context.Context, doc *Document) error

	// Store multiple documents in batch
	StoreBatch(ctx context.Context, docs []*Document) error

	// Query for similar documents
	Query(ctx context.Context, query string, k int) ([]*Match, error)

	// Query with pre-computed embedding
	QueryWithEmbedding(ctx context.Context, embedding []float32, k int) ([]*Match, error)

	// Delete a document by ID
	Delete(ctx context.Context, id string) error

	// Delete documents matching criteria
	DeleteBatch(ctx context.Context, filter map[string]interface{}) error

	// Get document count
	Count(ctx context.Context) (int, error)

	// Close and cleanup
	Close() error
}

// Document represents a stored document with metadata
type Document struct {
	ID        string                 // Unique identifier (UUID or hash)
	Content   string                 // Text content to embed
	Embedding []float32              // Vector representation (optional, computed if nil)
	Metadata  map[string]interface{} // Flexible metadata
	Timestamp time.Time              // When document was created
}

// Match represents a search result
type Match struct {
	Document *Document
	Score    float32 // Similarity score (0-1, higher is more similar)
}

// Metadata keys (standardized)
const (
	MetadataHostname   = "hostname"
	MetadataTimestamp  = "timestamp"
	MetadataSeverity   = "severity"
	MetadataCategory   = "category"
	MetadataCheckName  = "check_name"
	MetadataJobID      = "job_id"
	MetadataResolution = "resolution" // For successful remediations
	MetadataOutcome    = "outcome"    // success, failure
)
