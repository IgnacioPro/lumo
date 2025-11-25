# AI Provider Refactoring Plan

**Date:** 2025-11-20
**Status:** DESIGN PHASE
**Impact:** ~1,400 LOC reduction (60% of AI provider code)
**Estimated Effort:** 3-5 days

---

## Executive Summary

The 5 AI providers (Anthropic, OpenAI, Gemini, Ollama, OpenRouter) contain ~2,344 lines of code with **60-70% duplication**. This plan extracts common patterns into shared components while preserving provider-specific behavior.

**Benefits:**
- **Maintenance:** Bug fixes in one place instead of five
- **Extensibility:** Adding new providers becomes 3x faster
- **Testability:** Test common logic once, providers test only their specifics
- **Consistency:** Uniform error handling, logging, and retry behavior

---

## Current State Analysis

### File Sizes
| Provider | Lines | Duplication Est. |
|----------|-------|------------------|
| anthropic.go | 467 | ~65% |
| openai.go | 526 | ~70% |
| gemini.go | 462 | ~60% |
| ollama.go | 376 | ~55% |
| openrouter.go | 513 | ~70% |
| **Total** | **2,344** | **~1,400 duplicated** |

### Common Patterns (Found in ALL 5 providers)

**1. Constructor Validation (~40 lines each = 200 LOC total)**
```go
- Check API key (except Ollama)
- Set default model
- Set default timeout (120s or 300s)
- Set default max_tokens (4096)
- Set default temperature (1.0)
- Set default endpoint
- Validate endpoint security
- Create HTTP client
```

**2. Analyze() Method (~60 lines each = 300 LOC total)**
```go
- Build system and user prompts
- Log prompt details
- Build API request
- Call API
- Parse response content
- Parse AI analysis response
- Set metadata (timestamp, duration, tokens)
- Log completion
```

**3. AnalyzeStream() Method (~35 lines each = 175 LOC total)**
```go
- Build prompts
- Build streaming request
- Create channel
- Spawn goroutine
- Handle errors with channel
```

**4. HTTP Operations (~100 lines each = 500 LOC total)**
```go
- Marshal request to JSON
- Create HTTP request with context
- Set headers (provider-specific)
- Execute request
- Defer close body
- Read response body
- Handle non-200 status codes
- Log request/response
- Parse JSON response
```

**5. Streaming Logic (~50 lines each = 250 LOC total)**
```go
- Create HTTP request
- Read SSE stream line-by-line
- Parse "data: {json}" format
- Handle [DONE] marker
- Accumulate full content
- Send chunks to channel
- Handle scanner errors
```

### Provider-Specific Differences

| Feature | Anthropic | OpenAI | Gemini | Ollama | OpenRouter |
|---------|-----------|--------|--------|--------|------------|
| **Auth** | x-api-key | Bearer | URL param | None | Bearer |
| **System Prompt** | Separate field | Message | Combined | Message | Message |
| **Streaming** | SSE events | SSE + [DONE] | SSE + alt=sse | JSON lines | SSE + [DONE] |
| **Token Usage** | ✅ | ✅ | ✅ | ❌ | ✅ |
| **Special Features** | Content blocks | Reasoning effort, Refusal | Thinking tokens | - | Reasoning field |

---

## Refactoring Strategy

### Approach: **Composition over Inheritance**

Instead of Go's traditional embedding, we'll use **strategy pattern**:
1. Extract common logic into utility functions/types
2. Each provider implements thin adapter methods
3. Providers compose shared components

**Rationale:** Go doesn't have inheritance, and embedded structs can be confusing for method overriding. Explicit composition is clearer.

---

## Phase 1: Extract HTTP Client (~800 LOC savings)

### Create: `internal/ai/http_client.go`

**Purpose:** Centralize HTTP operations with provider-agnostic error handling.

```go
// HTTPClient handles common HTTP operations for AI providers
type HTTPClient struct {
    client *http.Client
    log    *logrus.Logger
}

// RequestOptions configures HTTP request behavior
type RequestOptions struct {
    Method          string
    URL             string
    Headers         map[string]string
    Body            interface{}
    ExpectedStatus  int  // Default: 200
}

// Response contains parsed HTTP response
type Response struct {
    StatusCode int
    Headers    http.Header
    Body       []byte
    Duration   time.Duration
}

// Methods to implement:
func NewHTTPClient(timeout time.Duration, log *logrus.Logger) *HTTPClient
func (c *HTTPClient) Do(ctx context.Context, opts RequestOptions) (*Response, error)
func (c *HTTPClient) DoStreaming(ctx context.Context, opts RequestOptions) (*http.Response, error)
```

**Benefits:**
- Single place for request/response logging
- Consistent error wrapping
- Automatic body closing
- Status code validation
- JSON marshaling/unmarshaling

**LOC Reduction:** ~160 lines per provider × 5 = **~800 lines**

---

## Phase 2: Extract Base Provider Logic (~400 LOC savings)

### Create: `internal/ai/base_provider.go`

**Purpose:** Common Analyze/AnalyzeStream scaffolding.

```go
// ProviderAdapter defines provider-specific operations
type ProviderAdapter interface {
    Name() string
    BuildRequest(systemPrompt, userPrompt string) (interface{}, error)
    ParseResponse(body []byte) (content string, usage *TokenUsage, err error)
    SetHeaders(req *http.Request)
    GetEndpoint() string
}

// BaseProvider provides common implementation
type BaseProvider struct {
    adapter ProviderAdapter
    config  *ProviderConfig
    client  *HTTPClient
    log     *logrus.Logger
}

// Methods:
func NewBaseProvider(adapter ProviderAdapter, config *ProviderConfig, log *logrus.Logger) *BaseProvider
func (p *BaseProvider) Analyze(ctx context.Context, req *AnalysisRequest) (*AnalysisResponse, error)
func (p *BaseProvider) AnalyzeStream(ctx context.Context, req *AnalysisRequest) (<-chan StreamChunk, error)
```

**How it works:**
1. `Analyze()` builds prompts (common)
2. Calls `adapter.BuildRequest()` (provider-specific)
3. Executes HTTP call (common via HTTPClient)
4. Calls `adapter.ParseResponse()` (provider-specific)
5. Parses AI response (common via `ParseAnalysisResponse`)
6. Sets metadata (common)

**Benefits:**
- Analyze() logic written once, reused 5 times
- Prompt building centralized
- Metadata handling consistent
- Logging uniform

**LOC Reduction:** ~80 lines per provider × 5 = **~400 lines**

---

## Phase 3: Extract Streaming Handler (~200 LOC savings)

### Create: `internal/ai/stream_handler.go`

**Purpose:** SSE stream parsing utilities.

```go
// StreamParser parses SSE streams
type StreamParser interface {
    ParseLine(line string) (*StreamEvent, error)
    IsDone(event *StreamEvent) bool
}

// StreamEvent represents a parsed stream event
type StreamEvent struct {
    Type    string
    Content string
    Done    bool
    Error   error
}

// Functions:
func ReadSSEStream(scanner *bufio.Scanner, parser StreamParser) (string, error)
func StreamToChannel(resp *http.Response, parser StreamParser, ch chan<- StreamChunk) error
```

**Provider-specific parsers:**
- `AnthropicStreamParser` - handles content_block_delta
- `OpenAIStreamParser` - handles delta.content + [DONE]
- `GeminiStreamParser` - handles candidates[].content.parts[]
- `OllamaStreamParser` - handles JSON lines (not SSE)
- `OpenRouterStreamParser` - same as OpenAI

**Benefits:**
- SSE parsing logic centralized
- Channel management uniform
- Error handling consistent

**LOC Reduction:** ~40 lines per provider × 5 = **~200 lines**

---

## Phase 4: Refactor Individual Providers

### Before (anthropic.go - 467 lines)
```go
type AnthropicProvider struct {
    config *ProviderConfig
    client *http.Client
    log    *logrus.Logger
}

func (p *AnthropicProvider) Analyze(...) {
    // 60 lines of duplicated logic
}

func (p *AnthropicProvider) callAPI(...) {
    // 90 lines of HTTP boilerplate
}

func (p *AnthropicProvider) streamAPI(...) {
    // 90 lines of SSE parsing
}
```

### After (anthropic.go - ~180 lines)
```go
type AnthropicProvider struct {
    *BaseProvider  // Composition
}

func NewAnthropicProvider(config *ProviderConfig, log *logrus.Logger) (*AnthropicProvider, error) {
    adapter := &anthropicAdapter{config: config}
    base := NewBaseProvider(adapter, config, log)
    return &AnthropicProvider{BaseProvider: base}, nil
}

// anthropicAdapter implements ProviderAdapter (thin adapter)
type anthropicAdapter struct {
    config *ProviderConfig
}

func (a *anthropicAdapter) Name() string {
    return "anthropic"
}

func (a *anthropicAdapter) BuildRequest(systemPrompt, userPrompt string) (interface{}, error) {
    return &anthropicRequest{
        Model:       a.config.Model,
        MaxTokens:   a.config.MaxTokens,
        System:      systemPrompt,
        Messages:    []anthropicMessage{{Role: "user", Content: userPrompt}},
    }, nil
}

func (a *anthropicAdapter) ParseResponse(body []byte) (string, *TokenUsage, error) {
    var resp anthropicResponse
    if err := json.Unmarshal(body, &resp); err != nil {
        return "", nil, err
    }

    var content strings.Builder
    for _, block := range resp.Content {
        if block.Type == "text" {
            content.WriteString(block.Text)
        }
    }

    usage := &TokenUsage{
        InputTokens:  resp.Usage.InputTokens,
        OutputTokens: resp.Usage.OutputTokens,
        TotalTokens:  resp.Usage.InputTokens + resp.Usage.OutputTokens,
    }

    return content.String(), usage, nil
}

func (a *anthropicAdapter) SetHeaders(req *http.Request) {
    req.Header.Set("x-api-key", a.config.APIKey)
    req.Header.Set("anthropic-version", AnthropicAPIVersion)
}
```

**Result:**
- **Before:** 467 lines (Anthropic) → **After:** ~180 lines
- **Before:** 526 lines (OpenAI) → **After:** ~200 lines
- **Before:** 462 lines (Gemini) → **After:** ~170 lines
- **Before:** 376 lines (Ollama) → **After:** ~140 lines
- **Before:** 513 lines (OpenRouter) → **After:** ~190 lines

**Total:** 2,344 → ~880 lines (**~1,460 LOC savings, 62% reduction**)

---

## Implementation Order

### Step 1: Create Shared Components (2 days)
1. ✅ Create `internal/ai/http_client.go` + tests
2. ✅ Create `internal/ai/base_provider.go` + tests
3. ✅ Create `internal/ai/stream_handler.go` + tests

**Deliverables:**
- 3 new files (~400 LOC total)
- 3 test files (~300 LOC total)
- All tests passing

### Step 2: Refactor Anthropic (0.5 days)
1. ✅ Create `anthropicAdapter` implementing `ProviderAdapter`
2. ✅ Update `AnthropicProvider` to use `BaseProvider`
3. ✅ Create `AnthropicStreamParser`
4. ✅ Run existing tests (no changes needed to test suite)

**Validation:** All existing `anthropic_test.go` tests pass unchanged.

### Step 3: Refactor OpenAI (0.5 days)
- Same pattern as Anthropic
- Handle reasoning_effort parameter
- Handle refusal field

### Step 4: Refactor Gemini (0.5 days)
- Same pattern
- Handle combined system+user prompt
- Handle thinking tokens

### Step 5: Refactor Ollama (0.5 days)
- Same pattern
- Handle JSON line streaming (not SSE)
- No token usage

### Step 6: Refactor OpenRouter (0.5 days)
- Same pattern
- Handle reasoning field (DeepSeek R1)
- OpenAI-compatible format

### Step 7: Final Validation (0.5 days)
1. ✅ Run full test suite
2. ✅ Test all 5 providers end-to-end
3. ✅ Run `make ci` (linters, tests, build)
4. ✅ Update `CLAUDE.md` with new architecture
5. ✅ Update this report with "COMPLETE" status

---

## Testing Strategy

### Unit Tests (No Changes Required)
- Existing provider tests mock HTTP responses
- Tests only verify interface behavior
- Internal refactoring doesn't affect test API

### Integration Tests
```bash
# Test each provider (requires API keys)
go test -v ./internal/ai -run TestAnthropicProvider_Analyze
go test -v ./internal/ai -run TestOpenAIProvider_Analyze
go test -v ./internal/ai -run TestGeminiProvider_Analyze
go test -v ./internal/ai -run TestOllamaProvider_Analyze
go test -v ./internal/ai -run TestOpenRouterProvider_Analyze
```

### Manual Validation
```bash
# Test CLI with different providers
lumo diagnose localhost --analyze --ai-provider anthropic
lumo diagnose localhost --analyze --ai-provider openai
lumo diagnose localhost --analyze --ai-provider gemini
lumo diagnose localhost --analyze --ai-provider ollama
lumo diagnose localhost --analyze --ai-provider openrouter
```

---

## Risk Assessment

### Low Risk
- **Backwards Compatibility:** No public API changes
- **Test Coverage:** Existing tests verify behavior
- **Incremental:** Refactor one provider at a time

### Medium Risk
- **Streaming:** SSE parsing has subtle differences
- **Mitigation:** Comprehensive streaming tests

### Zero Risk
- **Provider Interface:** `Provider` interface unchanged
- **Callers:** `diagnose.go` sees no difference

---

## Success Metrics

✅ **Code Reduction:** 2,344 → ~880 LOC (62% reduction)
✅ **Test Coverage:** Maintain or improve 27.1% → 60%+
✅ **CI:** All checks green
✅ **Functionality:** No regressions
✅ **Maintenance:** Bug fixes apply to all providers

---

## Future Enhancements (Post-Refactor)

1. **Retry Logic:** Add exponential backoff to HTTPClient (applies to all providers)
2. **Circuit Breaker:** Add failure detection (applies to all providers)
3. **Metrics:** Add prometheus metrics to BaseProvider (applies to all providers)
4. **Caching:** Add response caching to BaseProvider (applies to all providers)
5. **New Providers:** Add Cohere, Mistral in ~50 LOC each instead of ~500

---

## Questions for Review

1. **Approach:** Composition via `BaseProvider` + `ProviderAdapter` interface OK?
2. **Priority:** Start with Phase 1 (HTTPClient) or tackle all phases together?
3. **Testing:** Happy with existing test coverage, or add more before refactoring?
4. **Scope:** Include any additional refactoring (e.g., error handling, logging)?

---

## Next Steps

**After approval:**
1. Create feature branch: `refactor/ai-provider-deduplication`
2. Implement Phase 1 (HTTPClient)
3. Validate with all providers
4. Proceed to Phase 2-4
5. PR with detailed testing report

**Estimated Timeline:** 3-5 days focused work

---

**Ready to proceed?** Let me know if you'd like me to:
- **Start implementing** (I'll begin with Phase 1)
- **Modify the plan** (different approach, scope, or priority)
- **Answer questions** (clarify any part of the design)
