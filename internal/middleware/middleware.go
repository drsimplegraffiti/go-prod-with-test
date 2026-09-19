package middleware

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"regexp"
	"runtime/debug"
	"strings"
	"sync"
	"time"
)

type (
	requestIDKeyType struct{}
	loggerKeyType    struct{}
)

var (
	requestIDKey = requestIDKeyType{}
	loggerKeyKey = loggerKeyType{}
)

// ---------------------------------------------------------------------------
// Response recorders
//
// Both recorders deliberately implement http.Flusher, http.Hijacker and
// http.Pusher by delegating to the wrapped writer, so streaming endpoints
// (SSE, websockets) keep working through the middleware chain.
// ---------------------------------------------------------------------------

type statusRecorder struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (r *statusRecorder) WriteHeader(code int) {
	if r.wroteHeader {
		return
	}
	r.wroteHeader = true
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func (r *statusRecorder) Flush() {
	if f, ok := r.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (r *statusRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	h, ok := r.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, errors.New("wrapped ResponseWriter does not support hijacking")
	}
	return h.Hijack()
}

func (r *statusRecorder) Push(target string, opts *http.PushOptions) error {
	if p, ok := r.ResponseWriter.(http.Pusher); ok {
		return p.Push(target, opts)
	}
	return http.ErrNotSupported
}

type bodyDumpRecorder struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
	body        bytes.Buffer
}

func (r *bodyDumpRecorder) WriteHeader(code int) {
	if r.wroteHeader {
		return
	}
	r.wroteHeader = true
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func (r *bodyDumpRecorder) Write(b []byte) (int, error) {
	if !r.wroteHeader {
		r.WriteHeader(http.StatusOK)
	}
	r.body.Write(b)
	return r.ResponseWriter.Write(b)
}

func (r *bodyDumpRecorder) Flush() {
	if f, ok := r.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (r *bodyDumpRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	h, ok := r.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, errors.New("wrapped ResponseWriter does not support hijacking")
	}
	return h.Hijack()
}

func (r *bodyDumpRecorder) Push(target string, opts *http.PushOptions) error {
	if p, ok := r.ResponseWriter.(http.Pusher); ok {
		return p.Push(target, opts)
	}
	return http.ErrNotSupported
}

// ---------------------------------------------------------------------------
// Request ID
// ---------------------------------------------------------------------------

// RequestID assigns a unique ID to each request, propagated via context and
// the X-Request-ID response header. It also injects a request-scoped logger
// carrying the request_id, so any code logging inside the request gets
// correlation for free via LoggerFromContext.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		if id == "" {
			id = generateRequestID()
		}
		w.Header().Set("X-Request-ID", id)
		ctx := context.WithValue(r.Context(), requestIDKey, id)
		ctx = context.WithValue(ctx, loggerKeyKey, slog.With("request_id", id))
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequestIDFromContext extracts the request ID set by RequestID.
func RequestIDFromContext(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey).(string)
	return id
}

// LoggerFromContext returns the request-scoped logger set by RequestID.
// Outside a request it returns the default logger.
func LoggerFromContext(ctx context.Context) *slog.Logger {
	if l, ok := ctx.Value(loggerKeyKey).(*slog.Logger); ok {
		return l
	}
	return slog.Default()
}

func generateRequestID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "unknown"
	}
	return hex.EncodeToString(b)
}

// ---------------------------------------------------------------------------
// Logging
// ---------------------------------------------------------------------------

// Logging logs method, path, status, duration and request ID for every
// request. 5xx responses are logged at error level so alerting can key on
// that without parsing messages.
func Logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

		next.ServeHTTP(rec, r)

		attrs := []any{
			"method", r.Method,
			"path", r.URL.Path,
			"status", rec.status,
			"duration_ms", time.Since(start).Milliseconds(),
			"remote_addr", r.RemoteAddr,
		}
		switch {
		case rec.status >= 500:
			LoggerFromContext(r.Context()).Error("http_request", attrs...)
		case rec.status >= 400:
			LoggerFromContext(r.Context()).Warn("http_request", attrs...)
		default:
			LoggerFromContext(r.Context()).Info("http_request", attrs...)
		}
	})
}

// ---------------------------------------------------------------------------
// Panic recovery
// ---------------------------------------------------------------------------

// Recover converts any panic in a downstream handler into a 500 response
// instead of crashing the server, and logs the panic with a stack trace.
// If response headers were already written, it cannot rewrite the status,
// so it only logs.
func Recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		defer func() {
			if err := recover(); err != nil {
				LoggerFromContext(r.Context()).Error("panic recovered",
					"error", err,
					"path", r.URL.Path,
					"stack", string(debug.Stack()),
				)
				if !rec.wroteHeader {
					w.Header().Set("Content-Type", "application/json; charset=utf-8")
					w.WriteHeader(http.StatusInternalServerError)
					w.Write([]byte(`{"error":"Internal Server Error","message":"an unexpected error occurred"}`))
				}
			}
		}()
		next.ServeHTTP(rec, r)
	})
}

// ---------------------------------------------------------------------------
// CORS
// ---------------------------------------------------------------------------

// CORS applies an explicit CORS policy suitable for a public JSON API.
// allowedOrigins comes from config — do not hardcode "*" outside dev.
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

// ---------------------------------------------------------------------------
// Security headers
// ---------------------------------------------------------------------------

// SecurityHeaders sets a baseline of defensive HTTP headers for a JSON API.
// Set hsts=true in production (it breaks plain-HTTP testing otherwise).
func SecurityHeaders(hsts bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("X-Frame-Options", "DENY")
			w.Header().Set("Referrer-Policy", "no-referrer")
			w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
			w.Header().Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
			if hsts {
				w.Header().Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains")
			}
			next.ServeHTTP(w, r)
		})
	}
}

// ---------------------------------------------------------------------------
// Rate limiting
// ---------------------------------------------------------------------------

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

// stale reports whether the bucket has been idle long enough to be deleted.
// lastSeen is only touched while holding rl's lock during sweeping.
func (b *tokenBucket) idle(now time.Time, maxIdle time.Duration) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return now.Sub(b.last) > maxIdle
}

// RateLimiter is a simple in-memory, per-client-IP rate limiter. It is
// process-local: in a multi-instance deployment, back this with Redis or a
// similar shared store instead.
type RateLimiter struct {
	mu      sync.Mutex
	buckets map[string]*tokenBucket
	rps     float64
	burst   float64

	// trustXFF enables reading the client IP from X-Forwarded-For. Only
	// enable this when the app is behind a proxy/LB you control; otherwise
	// clients can spoof the header to evade rate limits.
	trustXFF bool
}

// NewRateLimiter builds a limiter allowing `rps` requests per second per
// client IP, with a burst allowance equal to rps.
func NewRateLimiter(rps int, trustXFF bool) *RateLimiter {
	rl := &RateLimiter{
		buckets:  make(map[string]*tokenBucket),
		rps:      float64(rps),
		burst:    float64(rps),
		trustXFF: trustXFF,
	}
	go rl.sweep() // process-local cleanup goroutine; lifetime = process lifetime
	return rl
}

// sweep deletes buckets idle for more than 10 minutes so long-running
// processes don't accumulate one bucket per client IP ever seen.
func (rl *RateLimiter) sweep() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for now := range ticker.C {
		rl.mu.Lock()
		for k, b := range rl.buckets {
			if b.idle(now, 10*time.Minute) {
				delete(rl.buckets, k)
			}
		}
		rl.mu.Unlock()
	}
}

// Middleware enforces the configured rate limit, keyed by client IP.
func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := clientIP(r, rl.trustXFF)

		rl.mu.Lock()
		b, ok := rl.buckets[key]
		if !ok {
			b = &tokenBucket{tokens: rl.burst, capacity: rl.burst, refillPS: rl.rps, last: time.Now()}
			rl.buckets[key] = b
		}
		rl.mu.Unlock()

		if !b.allow() {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte(`{"error":"Too Many Requests","message":"rate limit exceeded"}`))
			return
		}
		next.ServeHTTP(w, r)
	})
}

// clientIP extracts the client IP. When trustXFF is false it always uses
// the connection's remote address (the safe default).
func clientIP(r *http.Request, trustXFF bool) string {
	if trustXFF {
		if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
			// X-Forwarded-For can be a list: client, proxy1, proxy2.
			// The client-supplied value is the first entry.
			first := strings.TrimSpace(strings.Split(fwd, ",")[0])
			if first != "" {
				return first
			}
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// ---------------------------------------------------------------------------
// Middleware chain
// ---------------------------------------------------------------------------

// Chain composes middleware left-to-right: Chain(a, b, c)(h) runs a, then b,
// then c, then h. Order matters: RequestID is outermost so every downstream
// middleware and handler can use RequestIDFromContext/LoggerFromContext.
func Chain(mws ...func(http.Handler) http.Handler) func(http.Handler) http.Handler {
	return func(h http.Handler) http.Handler {
		for i := len(mws) - 1; i >= 0; i-- {
			h = mws[i](h)
		}
		return h
	}
}

// ---------------------------------------------------------------------------
// Body dump (audit logging)
// ---------------------------------------------------------------------------

// sensitiveField redacts credentials and tokens from captured bodies before
// they are persisted anywhere. It matches JSON string fields case-sensitively.
var sensitiveField = regexp.MustCompile(
	`"(password|refresh_token|access_token|authorization|token)"\s*:\s*"[^"]*"`)

// Redact replaces sensitive JSON field values with [REDACTED].
func Redact(b []byte) []byte {
	if len(b) == 0 {
		return b
	}
	return sensitiveField.ReplaceAll(b, []byte(`"$1":"[REDACTED]"`))
}

// BodyDumpOptions configures the BodyDump middleware.
type BodyDumpOptions struct {
	// MaxBytes caps how much of the request body is buffered. Requests with
	// larger bodies are truncated for auditing purposes only — downstream
	// handlers still see the full body. Prevents memory exhaustion.
	MaxBytes int64

	// SkipPaths lists path prefixes (e.g. "/api/v1/profiles/avatar") that
	// must not be buffered — file uploads/downloads, SSE, etc.
	SkipPaths []string
}

// BodyDump captures the request/response bodies and hands them to fn for
// auditing. The request body is fully read into memory up front and then
// replaced with a fresh reader so downstream handlers can still consume it.
// Paths matching SkipPaths pass through untouched. Passwords and tokens are
// redacted before fn is called.
func BodyDump(opts BodyDumpOptions, fn func(r *http.Request, status int, duration time.Duration, reqBody, resBody []byte)) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			for _, p := range opts.SkipPaths {
				if strings.HasPrefix(r.URL.Path, p) {
					next.ServeHTTP(w, r)
					return
				}
			}

			maxBytes := opts.MaxBytes
			if maxBytes <= 0 {
				maxBytes = 64 << 10 // 64 KiB
			}

			var reqBody []byte
			if r.Body != nil {
				reqBody, _ = io.ReadAll(io.LimitReader(r.Body, maxBytes+1))
				r.Body.Close()
				if int64(len(reqBody)) > maxBytes {
					reqBody = reqBody[:maxBytes]
				}
				r.Body = io.NopCloser(bytes.NewReader(reqBody))
			}

			start := time.Now()
			rec := &bodyDumpRecorder{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(rec, r)

			fn(r, rec.status, time.Since(start), Redact(reqBody), Redact(rec.body.Bytes()))
		})
	}
}
