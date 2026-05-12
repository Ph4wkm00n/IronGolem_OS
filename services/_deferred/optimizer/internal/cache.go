package internal

import (
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"strings"
	"sync"
	"time"
)

// DefaultCacheTTL is the default time-to-live for cached prompt responses.
const DefaultCacheTTL = 30 * time.Minute

// CacheEntry stores a cached LLM response along with hit tracking metadata.
type CacheEntry struct {
	// Response is the cached completion content.
	Response string `json:"response"`

	// CachedAt records when this entry was stored.
	CachedAt time.Time `json:"cached_at"`

	// HitCount tracks how many times this entry has been served.
	HitCount int `json:"hit_count"`

	// TTL is the time-to-live for this entry.
	TTL time.Duration `json:"ttl_ns"`

	// Model records which model produced this response.
	Model string `json:"model"`

	// InputTokens is the token count of the original request.
	InputTokens int `json:"input_tokens"`

	// EstimatedCost is the cost that was saved per cache hit.
	EstimatedCost float64 `json:"estimated_cost"`
}

// IsExpired returns true if the entry has exceeded its TTL.
func (ce *CacheEntry) IsExpired() bool {
	return time.Since(ce.CachedAt) > ce.TTL
}

// CacheMetrics provides performance statistics for the prompt cache.
type CacheMetrics struct {
	// HitRate is the ratio of cache hits to total lookups (0.0 to 1.0).
	HitRate float64 `json:"hit_rate"`

	// MissRate is the ratio of cache misses to total lookups (0.0 to 1.0).
	MissRate float64 `json:"miss_rate"`

	// TotalHits is the absolute number of cache hits.
	TotalHits int64 `json:"total_hits"`

	// TotalMisses is the absolute number of cache misses.
	TotalMisses int64 `json:"total_misses"`

	// EvictionCount is the number of entries evicted due to TTL expiry.
	EvictionCount int64 `json:"eviction_count"`

	// CostSaved is the estimated cost saved by serving cached responses.
	CostSaved float64 `json:"cost_saved"`

	// EntryCount is the current number of entries in the cache.
	EntryCount int `json:"entry_count"`
}

// PromptCache provides a TTL-based cache for LLM prompt responses.
// Cache keys are derived from a hash of the model, system prompt, and
// the first N messages, ensuring semantically equivalent requests share
// a cache entry.
type PromptCache struct {
	mu      sync.RWMutex
	entries map[string]*CacheEntry
	ttl     time.Duration
	logger  *slog.Logger

	// Metrics tracking.
	hits      int64
	misses    int64
	evictions int64
	costSaved float64
}

// NewPromptCache creates a new cache with the given default TTL.
func NewPromptCache(ttl time.Duration, logger *slog.Logger) *PromptCache {
	if ttl <= 0 {
		ttl = DefaultCacheTTL
	}
	return &PromptCache{
		entries: make(map[string]*CacheEntry),
		ttl:     ttl,
		logger:  logger,
	}
}

// MakeCacheKey builds a deterministic cache key from the model identifier,
// system prompt, and the first N messages. This ensures that semantically
// equivalent requests produce the same key.
func MakeCacheKey(model, systemPrompt string, messages []string) string {
	h := sha256.New()
	h.Write([]byte(model))
	h.Write([]byte("|"))
	h.Write([]byte(systemPrompt))
	h.Write([]byte("|"))
	h.Write([]byte(strings.Join(messages, "|")))
	return hex.EncodeToString(h.Sum(nil))
}

// Get looks up a cache entry by key. If the entry exists and has not
// expired, it is returned and the hit counter is incremented. Expired
// entries are evicted on access.
func (pc *PromptCache) Get(key string) (string, bool) {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	entry, ok := pc.entries[key]
	if !ok {
		pc.misses++
		return "", false
	}

	if entry.IsExpired() {
		delete(pc.entries, key)
		pc.evictions++
		pc.misses++
		pc.logger.Debug("cache entry expired and evicted",
			slog.String("key", key[:16]),
		)
		return "", false
	}

	entry.HitCount++
	pc.hits++
	pc.costSaved += entry.EstimatedCost

	pc.logger.Debug("cache hit",
		slog.String("key", key[:16]),
		slog.Int("hit_count", entry.HitCount),
	)

	return entry.Response, true
}

// Put stores a response in the cache with the configured TTL.
func (pc *PromptCache) Put(key, response, model string, inputTokens int, estimatedCost float64) {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	pc.entries[key] = &CacheEntry{
		Response:      response,
		CachedAt:      time.Now().UTC(),
		HitCount:      0,
		TTL:           pc.ttl,
		Model:         model,
		InputTokens:   inputTokens,
		EstimatedCost: estimatedCost,
	}

	pc.logger.Debug("cache entry stored",
		slog.String("key", key[:16]),
		slog.String("model", model),
	)
}

// Evict removes expired entries from the cache. This can be called
// periodically to keep memory usage in check.
func (pc *PromptCache) Evict() int {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	evicted := 0
	for key, entry := range pc.entries {
		if entry.IsExpired() {
			delete(pc.entries, key)
			evicted++
		}
	}

	pc.evictions += int64(evicted)

	if evicted > 0 {
		pc.logger.Info("cache eviction sweep completed",
			slog.Int("evicted", evicted),
			slog.Int("remaining", len(pc.entries)),
		)
	}

	return evicted
}

// Metrics returns a snapshot of current cache performance statistics.
func (pc *PromptCache) Metrics() CacheMetrics {
	pc.mu.RLock()
	defer pc.mu.RUnlock()

	total := pc.hits + pc.misses
	var hitRate, missRate float64
	if total > 0 {
		hitRate = float64(pc.hits) / float64(total)
		missRate = float64(pc.misses) / float64(total)
	}

	return CacheMetrics{
		HitRate:       hitRate,
		MissRate:      missRate,
		TotalHits:     pc.hits,
		TotalMisses:   pc.misses,
		EvictionCount: pc.evictions,
		CostSaved:     pc.costSaved,
		EntryCount:    len(pc.entries),
	}
}

// Clear removes all entries from the cache and resets metrics.
func (pc *PromptCache) Clear() {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	pc.entries = make(map[string]*CacheEntry)
	pc.hits = 0
	pc.misses = 0
	pc.evictions = 0
	pc.costSaved = 0

	pc.logger.Info("cache cleared")
}

// RunEvictionLoop periodically evicts expired entries until the context
// is cancelled. Suitable for running as a background goroutine.
func (pc *PromptCache) RunEvictionLoop(done <-chan struct{}, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			pc.Evict()
		}
	}
}
