# AI Package

The `ai` package provides AI-powered analysis for diagnostic results using multiple providers.

## Overview

This package implements an adapter pattern for AI providers, allowing seamless switching between:

- **Anthropic** (Claude models)
- **OpenAI** (GPT models)
- **Gemini** (Google models)
- **Ollama** (Local models)
- **OpenRouter** (Multi-provider gateway)

## Key Types

### Provider Interface

```go
type Provider interface {
    Name() string
    Analyze(ctx context.Context, req *AnalysisRequest) (*AnalysisResponse, error)
    AnalyzeStream(ctx context.Context, req *AnalysisRequest) (<-chan StreamChunk, error)
    Health(ctx context.Context) error
    Ask(ctx context.Context, systemPrompt, userPrompt string) (string, *TokenUsage, error)
}
```

### Creating a Provider

```go
import "github.com/ignacio/lumo/internal/ai"

provider, err := ai.NewProvider(ai.ProviderAnthropic, &ai.ProviderConfig{
    APIKey:  os.Getenv("ANTHROPIC_API_KEY"),
    Model:   "claude-sonnet-4-20250514",
    Timeout: 30 * time.Second,
}, logger)
```

## Features

- **Streaming Responses** - Real-time analysis with `AnalyzeStream()`
- **Circuit Breaker** - Automatic failure protection via `internal/reliability`
- **TOON Format** - Token-optimized input format (30-60% reduction)
- **Custom HTTP Client** - No external SDKs, full control over requests

## Usage

```go
// Analyze diagnostic results
resp, err := provider.Analyze(ctx, &ai.AnalysisRequest{
    Report:     diagnosticReport,
    SystemInfo: sysInfo,
})

// Natural language query
response, tokens, err := provider.Ask(ctx, systemPrompt, "Why is the server slow?")
```

## Configuration

Set the API key via environment variable (never in config files):

```bash
export LUMO_ANTHROPIC_API_KEY=sk-ant-...
export LUMO_OPENAI_API_KEY=sk-...
export LUMO_GEMINI_API_KEY=...
```

## Files

| File | Purpose |
|------|---------|
| `types.go` | Core interfaces and types |
| `provider.go` | Provider factory |
| `base_provider.go` | Shared HTTP client and circuit breaker |
| `anthropic.go` | Anthropic Claude implementation |
| `openai.go` | OpenAI GPT implementation |
| `gemini.go` | Google Gemini implementation |
| `ollama.go` | Local Ollama implementation |
| `openrouter.go` | OpenRouter gateway implementation |
| `stream.go` | SSE streaming handler |
