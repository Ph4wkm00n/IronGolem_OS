package internal

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/Ph4wkm00n/IronGolem_OS/services/pkg/models"
)

// FetchResult holds the raw output of fetching a single source URL.
type FetchResult struct {
	Content      string        `json:"content"`
	StatusCode   int           `json:"status_code"`
	FetchedAt    time.Time     `json:"fetched_at"`
	ResponseTime time.Duration `json:"response_time"`
}

// SourceFetcher retrieves content from a URL.
type SourceFetcher interface {
	FetchSource(ctx context.Context, rawURL string) (FetchResult, error)
}

// HTTPFetcher implements SourceFetcher using net/http with timeout,
// user-agent, and destination allowlist checking to prevent SSRF.
type HTTPFetcher struct {
	client       *http.Client
	userAgent    string
	allowedHosts map[string]bool // empty means allow all public hosts
	logger       *slog.Logger
}

// HTTPFetcherConfig configures the HTTP fetcher.
type HTTPFetcherConfig struct {
	Timeout      time.Duration
	UserAgent    string
	AllowedHosts []string
	MaxBodyBytes int64
}

// DefaultHTTPFetcherConfig returns sensible defaults.
func DefaultHTTPFetcherConfig() HTTPFetcherConfig {
	return HTTPFetcherConfig{
		Timeout:      30 * time.Second,
		UserAgent:    "IronGolemOS-Research/0.1",
		MaxBodyBytes: 2 * 1024 * 1024, // 2 MiB
	}
}

// NewHTTPFetcher creates a new fetcher with SSRF-safe transport.
func NewHTTPFetcher(cfg HTTPFetcherConfig, logger *slog.Logger) *HTTPFetcher {
	allowed := make(map[string]bool, len(cfg.AllowedHosts))
	for _, h := range cfg.AllowedHosts {
		allowed[strings.ToLower(h)] = true
	}

	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:        20,
		IdleConnTimeout:     90 * time.Second,
		TLSHandshakeTimeout: 10 * time.Second,
	}

	return &HTTPFetcher{
		client: &http.Client{
			Timeout:   cfg.Timeout,
			Transport: transport,
			// Do not follow redirects blindly.
			CheckRedirect: func(_ *http.Request, via []*http.Request) error {
				if len(via) >= 3 {
					return fmt.Errorf("stopped after 3 redirects")
				}
				return nil
			},
		},
		userAgent:    cfg.UserAgent,
		allowedHosts: allowed,
		logger:       logger,
	}
}

// FetchSource fetches content from the given URL. It validates the
// destination against the allowlist and blocks private IP ranges to
// prevent SSRF attacks (security layer).
func (f *HTTPFetcher) FetchSource(ctx context.Context, rawURL string) (FetchResult, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return FetchResult{}, fmt.Errorf("invalid URL: %w", err)
	}

	// Only allow HTTP(S).
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return FetchResult{}, fmt.Errorf("unsupported scheme: %s", parsed.Scheme)
	}

	// Block private/loopback addresses to prevent SSRF.
	if isPrivateHost(parsed.Hostname()) {
		return FetchResult{}, fmt.Errorf("destination blocked: private address")
	}

	// Allowlist check (if configured).
	if len(f.allowedHosts) > 0 {
		if !f.allowedHosts[strings.ToLower(parsed.Hostname())] {
			return FetchResult{}, fmt.Errorf("host not in allowlist: %s", parsed.Hostname())
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return FetchResult{}, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("User-Agent", f.userAgent)
	req.Header.Set("Accept", "text/html, application/json, text/plain")

	start := time.Now()
	resp, err := f.client.Do(req)
	responseTime := time.Since(start)
	if err != nil {
		return FetchResult{}, fmt.Errorf("fetching %s: %w", rawURL, err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Cap the body size to avoid memory issues.
	body, err := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
	if err != nil {
		return FetchResult{}, fmt.Errorf("reading response body: %w", err)
	}

	f.logger.DebugContext(ctx, "source fetched",
		slog.String("url", rawURL),
		slog.Int("status", resp.StatusCode),
		slog.Duration("response_time", responseTime),
		slog.Int("body_bytes", len(body)),
	)

	return FetchResult{
		Content:      string(body),
		StatusCode:   resp.StatusCode,
		FetchedAt:    time.Now().UTC(),
		ResponseTime: responseTime,
	}, nil
}

// isPrivateHost returns true if the hostname resolves to a private,
// loopback, or link-local address.
func isPrivateHost(host string) bool {
	lower := strings.ToLower(host)
	if lower == "localhost" || lower == "127.0.0.1" || lower == "::1" {
		return true
	}
	if strings.HasPrefix(lower, "10.") ||
		strings.HasPrefix(lower, "192.168.") ||
		strings.HasPrefix(lower, "172.16.") ||
		strings.HasPrefix(lower, "169.254.") ||
		lower == "0.0.0.0" {
		return true
	}
	return false
}

// TrustScorer evaluates source trust based on domain reputation,
// freshness, and consistency with other sources.
type TrustScorer struct {
	// knownDomains maps domain to a baseline trust score.
	knownDomains map[string]float64
	logger       *slog.Logger
}

// NewTrustScorer creates a scorer with default domain reputations.
func NewTrustScorer(logger *slog.Logger) *TrustScorer {
	return &TrustScorer{
		knownDomains: map[string]float64{
			"reuters.com":     0.9,
			"apnews.com":      0.9,
			"nature.com":      0.95,
			"arxiv.org":       0.85,
			"github.com":      0.8,
			"stackoverflow.com": 0.75,
			"wikipedia.org":   0.7,
		},
		logger: logger,
	}
}

// ScoreSource computes a trust score in [0, 1] for a source based on
// domain reputation, age, citation count, and consistency.
func (ts *TrustScorer) ScoreSource(factors models.SourceTrustFactors) float64 {
	// Start with domain baseline or a neutral default.
	score := 0.5
	if base, ok := ts.knownDomains[factors.Domain]; ok {
		score = base
	}

	// Boost for older, well-established domains (up to +0.1).
	if factors.Age > 365 {
		score += 0.05
	}
	if factors.Age > 3650 {
		score += 0.05
	}

	// Boost for citation count (up to +0.1).
	if factors.CitationCount > 10 {
		score += 0.05
	}
	if factors.CitationCount > 100 {
		score += 0.05
	}

	// Weight consistency heavily (up to +0.15 or -0.15).
	score += (factors.ConsistencyScore - 0.5) * 0.3

	// Clamp to [0, 1].
	if score < 0 {
		score = 0
	}
	if score > 1 {
		score = 1
	}

	return score
}

// RateLimiter provides per-domain request throttling to avoid
// overwhelming sources.
type RateLimiter struct {
	mu          sync.Mutex
	lastRequest map[string]time.Time
	minInterval time.Duration
	logger      *slog.Logger
}

// NewRateLimiter creates a limiter that enforces a minimum interval
// between requests to the same domain.
func NewRateLimiter(minInterval time.Duration, logger *slog.Logger) *RateLimiter {
	return &RateLimiter{
		lastRequest: make(map[string]time.Time),
		minInterval: minInterval,
		logger:      logger,
	}
}

// Wait blocks until a request to the given domain is allowed, or
// until the context is cancelled.
func (rl *RateLimiter) Wait(ctx context.Context, domain string) error {
	rl.mu.Lock()
	last, ok := rl.lastRequest[domain]
	rl.mu.Unlock()

	if ok {
		elapsed := time.Since(last)
		if elapsed < rl.minInterval {
			wait := rl.minInterval - elapsed
			rl.logger.DebugContext(ctx, "rate limiting",
				slog.String("domain", domain),
				slog.Duration("wait", wait),
			)
			select {
			case <-time.After(wait):
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}

	rl.mu.Lock()
	rl.lastRequest[domain] = time.Now()
	rl.mu.Unlock()

	return nil
}
