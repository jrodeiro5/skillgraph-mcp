// Package embed provides embedding-based tool retrieval for the skillgraph gateway.
// It embeds tool descriptions once at boot, rebuilds on config change, and exposes
// cosine-similarity search with optional two-tier (server → tool) hierarchy.
package embed

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"strings"
	"sync"
)

// Provider selects the embedding backend.
type Provider int

const (
	ProviderNone     Provider = iota
	ProviderOllama            // LLM_BASE_URL pointing at Ollama, or OLLAMA_HOST
	ProviderOpenAI            // OPENAI_API_KEY
)

// Config holds embedding configuration, resolved from env vars.
type Config struct {
	Provider  Provider
	Endpoint  string // e.g. http://localhost:11434 or https://api.openai.com
	APIKey    string
	Model     string
	Dimension int // 0 = model default; set via EMBED_DIMENSIONS (e.g. 256 for Matryoshka)
}

// ResolveConfig detects the embedding provider using the same priority chain
// as the LLM provider in refine/engine.go.
// Priority: LLM_BASE_URL (Ollama/LiteLLM) > OPENAI_API_KEY
func ResolveConfig() Config {
	dim := 0
	if v := os.Getenv("EMBED_DIMENSIONS"); v != "" {
		fmt.Sscanf(v, "%d", &dim)
	}

	if base := os.Getenv("LLM_BASE_URL"); base != "" {
		model := os.Getenv("EMBED_MODEL")
		if model == "" {
			model = "nomic-embed-text"
		}
		return Config{
			Provider:  ProviderOllama,
			Endpoint:  strings.TrimRight(base, "/"),
			APIKey:    os.Getenv("LLM_API_KEY"),
			Model:     model,
			Dimension: dim,
		}
	}
	if base := os.Getenv("OLLAMA_HOST"); base != "" {
		model := os.Getenv("EMBED_MODEL")
		if model == "" {
			model = "nomic-embed-text"
		}
		return Config{
			Provider:  ProviderOllama,
			Endpoint:  strings.TrimRight(base, "/"),
			Model:     model,
			Dimension: dim,
		}
	}
	if key := os.Getenv("OPENAI_API_KEY"); key != "" {
		model := os.Getenv("EMBED_MODEL")
		if model == "" {
			model = "text-embedding-3-small"
		}
		return Config{
			Provider:  ProviderOpenAI,
			Endpoint:  "https://api.openai.com",
			APIKey:    key,
			Model:     model,
			Dimension: dim,
		}
	}
	return Config{Provider: ProviderNone}
}

// Embedder calls the embedding API and returns a float32 vector.
type Embedder struct {
	cfg Config
}

// NewEmbedder creates an Embedder from a resolved Config.
func NewEmbedder(cfg Config) *Embedder {
	return &Embedder{cfg: cfg}
}

// Embed returns the embedding vector for text.
func (e *Embedder) Embed(ctx context.Context, text string) ([]float32, error) {
	switch e.cfg.Provider {
	case ProviderOllama:
		return embedOllama(ctx, e.cfg, text)
	case ProviderOpenAI:
		return embedOpenAI(ctx, e.cfg, text)
	default:
		return nil, fmt.Errorf("no embedding provider configured (set LLM_BASE_URL/OLLAMA_HOST or OPENAI_API_KEY)")
	}
}

func embedOllama(ctx context.Context, cfg Config, text string) ([]float32, error) {
	endpoint := cfg.Endpoint + "/api/embed"
	body, err := json.Marshal(map[string]any{
		"model": cfg.Model,
		"input": text,
	})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if cfg.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ollama embed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ollama embed HTTP %d: %s", resp.StatusCode, b)
	}

	var result struct {
		Embeddings [][]float32 `json:"embeddings"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("ollama embed decode: %w", err)
	}
	if len(result.Embeddings) == 0 || len(result.Embeddings[0]) == 0 {
		return nil, fmt.Errorf("ollama embed: empty response")
	}
	return result.Embeddings[0], nil
}

func embedOpenAI(ctx context.Context, cfg Config, text string) ([]float32, error) {
	endpoint := cfg.Endpoint + "/v1/embeddings"
	payload := map[string]any{
		"model":           cfg.Model,
		"input":           text,
		"encoding_format": "float",
	}
	if cfg.Dimension > 0 {
		payload["dimensions"] = cfg.Dimension
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("openai embed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("openai embed HTTP %d: %s", resp.StatusCode, b)
	}

	var result struct {
		Data []struct {
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("openai embed decode: %w", err)
	}
	if len(result.Data) == 0 {
		return nil, fmt.Errorf("openai embed: empty response")
	}
	return result.Data[0].Embedding, nil
}

// ToolEntry is a single indexed tool.
type ToolEntry struct {
	Server      string
	ToolName    string    // resolved name (e.g. "gitnexus_impact")
	Description string
	Vector      []float32
}

// Index is a thread-safe in-memory embedding index over tool descriptions.
type Index struct {
	mu      sync.RWMutex
	entries []ToolEntry
	embedder *Embedder
}

// NewIndex creates an empty Index.
func NewIndex(embedder *Embedder) *Index {
	return &Index{embedder: embedder}
}

// SearchResult is a single match returned by Find.
type SearchResult struct {
	Server      string
	ToolName    string
	Description string
	Score       float32
}

// Rebuild replaces the index with a fresh set of tool embeddings.
// Blocks until all embeddings are fetched. Called at boot and on config change.
func (idx *Index) Rebuild(ctx context.Context, tools []ToolEntry) error {
	embedded := make([]ToolEntry, 0, len(tools))
	for _, t := range tools {
		vec, err := idx.embedder.Embed(ctx, t.Description)
		if err != nil {
			return fmt.Errorf("embed %s/%s: %w", t.Server, t.ToolName, err)
		}
		t.Vector = vec
		embedded = append(embedded, t)
	}

	idx.mu.Lock()
	idx.entries = embedded
	idx.mu.Unlock()
	return nil
}

// Find returns the top-k tools most similar to query.
// If serverFilter is non-empty, only tools from that server are considered.
func (idx *Index) Find(ctx context.Context, query string, k int, serverFilter string) ([]SearchResult, error) {
	qvec, err := idx.embedder.Embed(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("embed query: %w", err)
	}

	idx.mu.RLock()
	entries := idx.entries
	idx.mu.RUnlock()

	var candidates []scoredEntry
	for _, e := range entries {
		if serverFilter != "" && e.Server != serverFilter {
			continue
		}
		s := cosine(qvec, e.Vector)
		candidates = append(candidates, scoredEntry{
			entry: SearchResult{
				Server:      e.Server,
				ToolName:    e.ToolName,
				Description: e.Description,
				Score:       s,
			},
			score: s,
		})
	}

	// Partial sort: find top-k without full sort.
	topK := topKScored(candidates, k)
	results := make([]SearchResult, len(topK))
	for i, c := range topK {
		results[i] = c.entry
	}
	return results, nil
}

// Len returns the number of indexed tools.
func (idx *Index) Len() int {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return len(idx.entries)
}

func cosine(a, b []float32) float32 {
	if len(a) != len(b) {
		return 0
	}
	var dot, normA, normB float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}
	denom := math.Sqrt(normA) * math.Sqrt(normB)
	if denom == 0 {
		return 0
	}
	return float32(dot / denom)
}

// topKScored returns the top-k elements by score (descending) using a min-heap approach.
type scoredEntry struct {
	entry SearchResult
	score float32
}

func topKScored(items []scoredEntry, k int) []scoredEntry {
	if k <= 0 || len(items) == 0 {
		return nil
	}
	if k >= len(items) {
		// Sort descending.
		sortDesc(items)
		return items
	}
	// Partial selection: O(n*k) but k is small (≤10) and n≤200.
	for i := 0; i < k; i++ {
		maxIdx := i
		for j := i + 1; j < len(items); j++ {
			if items[j].score > items[maxIdx].score {
				maxIdx = j
			}
		}
		items[i], items[maxIdx] = items[maxIdx], items[i]
	}
	return items[:k]
}

func sortDesc(items []scoredEntry) {
	// Insertion sort — fine for ≤200 items.
	for i := 1; i < len(items); i++ {
		key := items[i]
		j := i - 1
		for j >= 0 && items[j].score < key.score {
			items[j+1] = items[j]
			j--
		}
		items[j+1] = key
	}
}
