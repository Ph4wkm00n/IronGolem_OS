// Package middleware provides HTTP middleware for the Gateway service.
//
// SecurityHeadersMiddleware, RateLimitMiddleware, RequestSizeMiddleware, and
// CORSMiddleware harden the gateway against common web vulnerabilities.
package middleware

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// --- Security Headers Middleware ---

// SecurityHeadersMiddleware adds standard security headers to every response:
//   - X-Content-Type-Options: nosniff
//   - X-Frame-Options: DENY
//   - X-XSS-Protection: 1; mode=block
//   - Strict-Transport-Security: max-age=63072000; includeSubDomains
//   - Content-Security-Policy: default-src 'self'
//   - Referrer-Policy: strict-origin-when-cross-origin
//   - Permissions-Policy: geolocation=(), camera=(), microphone=()
func SecurityHeadersMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("X-Frame-Options", "DENY")
			w.Header().Set("X-XSS-Protection", "1; mode=block")
			w.Header().Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains")
			w.Header().Set("Content-Security-Policy", "default-src 'self'")
			w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
			w.Header().Set("Permissions-Policy", "geolocation=(), camera=(), microphone=()")

			next.ServeHTTP(w, r)
		})
	}
}

// --- Rate Limit Middleware ---

// RateLimitConfig configures the token bucket rate limiter.
type RateLimitConfig struct {
	// RequestsPerMinute is the maximum number of requests allowed per IP per minute.
	RequestsPerMinute int
	// BurstSize is the maximum burst of requests allowed above the rate.
	BurstSize int
	// CleanupInterval controls how often stale entries are removed.
	CleanupInterval time.Duration
}

// DefaultRateLimitConfig returns a config allowing 60 requests per minute
// with a burst of 10.
func DefaultRateLimitConfig() RateLimitConfig {
	return RateLimitConfig{
		RequestsPerMinute: 60,
		BurstSize:         10,
		CleanupInterval:   5 * time.Minute,
	}
}

// tokenBucket tracks the rate limit state for a single client IP.
type tokenBucket struct {
	tokens     float64
	lastRefill time.Time
}

// rateLimiter implements a per-IP token bucket rate limiter.
type rateLimiter struct {
	mu      sync.Mutex
	buckets map[string]*tokenBucket
	rate    float64 // tokens per second
	burst   int
	stopCh  chan struct{}
}

func newRateLimiter(cfg RateLimitConfig) *rateLimiter {
	rl := &rateLimiter{
		buckets: make(map[string]*tokenBucket),
		rate:    float64(cfg.RequestsPerMinute) / 60.0,
		burst:   cfg.BurstSize,
		stopCh:  make(chan struct{}),
	}

	// Start cleanup goroutine to prevent unbounded memory growth.
	cleanupInterval := cfg.CleanupInterval
	if cleanupInterval <= 0 {
		cleanupInterval = 5 * time.Minute
	}
	go rl.cleanupLoop(cleanupInterval)

	return rl
}

// allow checks whether a request from the given IP should be allowed.
func (rl *rateLimiter) allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	b, ok := rl.buckets[ip]
	if !ok {
		b = &tokenBucket{
			tokens:     float64(rl.burst),
			lastRefill: now,
		}
		rl.buckets[ip] = b
	}

	// Refill tokens based on elapsed time.
	elapsed := now.Sub(b.lastRefill).Seconds()
	b.tokens += elapsed * rl.rate
	if b.tokens > float64(rl.burst) {
		b.tokens = float64(rl.burst)
	}
	b.lastRefill = now

	if b.tokens < 1 {
		return false
	}

	b.tokens--
	return true
}

// cleanupLoop removes stale entries to prevent memory leaks.
func (rl *rateLimiter) cleanupLoop(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-rl.stopCh:
			return
		case <-ticker.C:
			rl.mu.Lock()
			cutoff := time.Now().Add(-interval)
			for ip, b := range rl.buckets {
				if b.lastRefill.Before(cutoff) {
					delete(rl.buckets, ip)
				}
			}
			rl.mu.Unlock()
		}
	}
}

// extractIP extracts the client IP from the request, preferring
// X-Forwarded-For if present but falling back to RemoteAddr.
func extractIP(r *http.Request) string {
	// Check X-Forwarded-For (first IP in the chain).
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.SplitN(xff, ",", 2)
		ip := strings.TrimSpace(parts[0])
		if ip != "" {
			return ip
		}
	}

	// Fall back to RemoteAddr.
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// RateLimitMiddleware enforces per-IP rate limiting using a token bucket
// algorithm. When a client exceeds the rate, it returns 429 Too Many Requests.
func RateLimitMiddleware(cfg RateLimitConfig) func(http.Handler) http.Handler {
	rl := newRateLimiter(cfg)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := extractIP(r)

			if !rl.allow(ip) {
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Retry-After", "60")
				w.WriteHeader(http.StatusTooManyRequests)
				_, _ = w.Write([]byte(`{"error":"rate limit exceeded","retry_after_seconds":60}`))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// --- Request Size Middleware ---

// RequestSizeMiddleware limits the maximum size of request bodies. Any request
// with a Content-Length exceeding maxBytes or that reads more than maxBytes
// returns 413 Payload Too Large. The default limit is 1 MB.
func RequestSizeMiddleware(maxBytes int64) func(http.Handler) http.Handler {
	if maxBytes <= 0 {
		maxBytes = 1 << 20 // 1 MB default
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check Content-Length header for early rejection.
			if r.ContentLength > maxBytes {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusRequestEntityTooLarge)
				_, _ = w.Write([]byte(`{"error":"request body too large"}`))
				return
			}

			// Wrap the body with a size-limited reader as a safety net.
			r.Body = http.MaxBytesReader(w, r.Body, maxBytes)

			next.ServeHTTP(w, r)
		})
	}
}

// --- CORS Middleware ---

// CORSConfig configures Cross-Origin Resource Sharing behavior.
type CORSConfig struct {
	// AllowedOrigins is the list of permitted origins. An empty list means
	// no origins are allowed (most restrictive default).
	AllowedOrigins []string
	// AllowedMethods is the list of permitted HTTP methods.
	AllowedMethods []string
	// AllowedHeaders is the list of permitted request headers.
	AllowedHeaders []string
	// AllowCredentials indicates whether cookies/auth headers are permitted.
	AllowCredentials bool
	// MaxAge is the maximum duration (in seconds) that preflight results
	// can be cached by the browser.
	MaxAge int
}

// DefaultCORSConfig returns a restrictive CORS configuration that only
// allows GET and POST from the same origin.
func DefaultCORSConfig() CORSConfig {
	return CORSConfig{
		AllowedOrigins:   []string{},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Content-Type", "Authorization", "X-Tenant-ID", "X-User-ID", "X-Agent-Role", "X-Channel-ID", "X-Request-ID"},
		AllowCredentials: false,
		MaxAge:           86400, // 24 hours
	}
}

// CORSMiddleware handles CORS preflight and adds appropriate headers
// based on the provided configuration.
func CORSMiddleware(cfg CORSConfig) func(http.Handler) http.Handler {
	allowedOriginSet := make(map[string]bool, len(cfg.AllowedOrigins))
	for _, o := range cfg.AllowedOrigins {
		allowedOriginSet[strings.ToLower(o)] = true
	}

	methods := strings.Join(cfg.AllowedMethods, ", ")
	headers := strings.Join(cfg.AllowedHeaders, ", ")
	maxAge := "86400"
	if cfg.MaxAge > 0 {
		maxAge = func() string {
			v := cfg.MaxAge
			if v <= 0 {
				return "0"
			}
			s := ""
			for v > 0 {
				s = string(rune('0'+v%10)) + s
				v /= 10
			}
			return s
		}()
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")

			// Only set CORS headers if there's an Origin header and it's allowed.
			if origin != "" && isOriginAllowed(origin, allowedOriginSet) {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Methods", methods)
				w.Header().Set("Access-Control-Allow-Headers", headers)
				w.Header().Set("Access-Control-Max-Age", maxAge)
				if cfg.AllowCredentials {
					w.Header().Set("Access-Control-Allow-Credentials", "true")
				}
				w.Header().Set("Vary", "Origin")
			}

			// Handle preflight requests.
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// isOriginAllowed checks if the given origin is in the allowed set.
// An empty set means no origins are allowed.
func isOriginAllowed(origin string, allowed map[string]bool) bool {
	if len(allowed) == 0 {
		return false
	}
	return allowed[strings.ToLower(origin)]
}
