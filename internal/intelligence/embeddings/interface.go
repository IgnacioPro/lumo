package embeddings

import "context"

// Embedder generates vector embeddings from text
type Embedder interface {
	// Embed generates embeddings for a single text
	Embed(ctx context.Context, text string) ([]float32, error)

	// EmbedBatch generates embeddings for multiple texts
	EmbedBatch(ctx context.Context, texts []string) ([][]float32, error)

	// Dimensions returns the embedding vector size
	Dimensions() int
}
