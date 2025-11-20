package embeddings

import (
	"context"
	"fmt"
)

type AnthropicEmbedder struct {
	// Anthropic doesn't have a dedicated embedding API yet
	// Could use voyage-ai or other providers
}

func NewAnthropicEmbedder() *AnthropicEmbedder {
	return &AnthropicEmbedder{}
}

func (e *AnthropicEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	return nil, fmt.Errorf("Anthropic embeddings not yet available, use Voyage AI or OpenAI")
}

func (e *AnthropicEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	return nil, fmt.Errorf("not implemented")
}

func (e *AnthropicEmbedder) Dimensions() int {
	return 0
}
