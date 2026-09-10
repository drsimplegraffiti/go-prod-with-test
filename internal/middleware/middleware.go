package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

type requestIDKeyType struct{}

var requestIDKey = requestIDKeyType{}

// responseRecorder captures the status code so logging middleware can report
// it, since http.ResponseWriter doesn't expose it after the fact.
type responseRecorder struct {
	http.ResponseWriter
	status int
}

func (r *responseRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

// RequestID assigns a unique ID to each request, propagated via context and
// the X-Request-ID response header, for correlating logs across a request's
// lifecycle and across services.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		if id == "" {
			id = generateRequestID()
		}
		w.Header().Set("X-Request-ID", id)
		ctx := context.WithValue(r.Context(), requestIDKey, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequestIDFromContext extracts the request ID set by RequestID.
func RequestIDFromContext(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey).(string)
	return id
}

func generateRequestID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "unknown"
	}
	return hex.EncodeToString(b)
}

// Logging logs method, path, status, duration and request ID for every request.
func Logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &responseRecorder{ResponseWriter: w, status: http.StatusOK}

		next.ServeHTTP(rec, r)

		slog.Info("http_request",
			"request_id", RequestIDFromContext(r.Context()),
			"method", r.Method,
			"path", r.URL.Path,
			"status", rec.status,
			"duration_ms", time.Since(start).Milliseconds(),
			"remote_addr", r.RemoteAddr,
		)
	})
}

// Recover converts any panic in a downstream handler into a 500 response
// instead of crashing the server, and logs the panic with a stack trace.
func Recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				slog.Error("panic recovered",
					"request_id", RequestIDFromContext(r.Context()),
					"error", err,
					"path", r.URL.Path,
				)
				w.Header().Set("Content-Type", "application/json; charset=utf-8")
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte(`{"error":"Internal Server Error","message":"an unexpected error occurred"}`))
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// CORS applies a permissive-but-explicit CORS policy suitable for a public
// JSON API. Tighten allowedOrigins for production deployments.
func CORS(allowedOrigins []string) func(http.Handler) http.Handler {
	allowed := make(map[string]struct{}, len(allowedOrigins))
	allowAll := false
	for _, o := range allowedOrigins {
		if o == "*" {
			allowAll = true
		}
		allowed[o] = struct{}{}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin != "" {
				if allowAll {
					w.Header().Set("Access-Control-Allow-Origin", "*")
				} else if _, ok := allowed[origin]; ok {
					w.Header().Set("Access-Control-Allow-Origin", origin)
					w.Header().Set("Vary", "Origin")
				}
			}
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// SecurityHeaders sets a baseline of defensive HTTP headers for a JSON API.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		next.ServeHTTP(w, r)
	})
}

// tokenBucket is a minimal per-key rate limiter using the token bucket
// algorithm, safe for concurrent use.
type tokenBucket struct {
	mu       sync.Mutex
	tokens   float64
	capacity float64
	refillPS float64
	last     time.Time
}

func (b *tokenBucket) allow() bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(b.last).Seconds()
	b.last = now

	b.tokens += elapsed * b.refillPS
	if b.tokens > b.capacity {
		b.tokens = b.capacity
	}
	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

// RateLimiter is a simple in-memory, per-client-IP rate limiter. It is
// process-local: in a multi-instance deployment, back this with Redis or a
// similar shared store instead.
type RateLimiter struct {
	mu      sync.Mutex
	buckets map[string]*tokenBucket
	rps     float64
	burst   float64
}

// NewRateLimiter builds a limiter allowing `rps` requests per second per
// client IP, with a burst allowance equal to rps.
func NewRateLimiter(rps int) *RateLimiter {
	return &RateLimiter{
		buckets: make(map[string]*tokenBucket),
		rps:     float64(rps),
		burst:   float64(rps),
	}
}

// Middleware enforces the configured rate limit, keyed by remote IP.
func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := clientIP(r)

		rl.mu.Lock()
		b, ok := rl.buckets[key]
		if !ok {
			b = &tokenBucket{tokens: rl.burst, capacity: rl.burst, refillPS: rl.rps, last: time.Now()}
			rl.buckets[key] = b
		}
		rl.mu.Unlock()

		if !b.allow() {
			w.Header().Set("Retry-After", "1")
			http.Error(w, `{"error":"Too Many Requests","message":"rate limit exceeded"}`, http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func clientIP(r *http.Request) string {
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		return fwd
	}
	return r.RemoteAddr
}

// Chain composes middleware left-to-right: Chain(a, b, c)(h) runs a, then b,
// then c, then h.
func Chain(mws ...func(http.Handler) http.Handler) func(http.Handler) http.Handler {
	return func(h http.Handler) http.Handler {
		for i := len(mws) - 1; i >= 0; i-- {
			h = mws[i](h)
		}
		return h
	}
}
