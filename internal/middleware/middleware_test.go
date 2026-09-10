package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequestID_GeneratesWhenMissing(t *testing.T) {
	var captured string
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = RequestIDFromContext(r.Context())
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	RequestID(next).ServeHTTP(rec, req)

	if captured == "" {
		t.Error("expected a request ID to be generated")
	}
	if rec.Header().Get("X-Request-ID") != captured {
		t.Error("expected X-Request-ID response header to match context value")
	}
}

func TestRequestID_PreservesIncoming(t *testing.T) {
	var captured string
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = RequestIDFromContext(r.Context())
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Request-ID", "client-supplied-id")
	rec := httptest.NewRecorder()

	RequestID(next).ServeHTTP(rec, req)

	if captured != "client-supplied-id" {
		t.Errorf("expected incoming request ID to be preserved, got %s", captured)
	}
}

func TestRecover_ConvertsPanicToInternalServerError(t *testing.T) {
	panicky := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("boom")
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	Recover(panicky).ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 after recovering panic, got %d", rec.Code)
	}
}

func TestRateLimiter_BlocksAfterBurstExhausted(t *testing.T) {
	rl := NewRateLimiter(1) // 1 request/sec, burst of 1
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := rl.Middleware(next)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "1.2.3.4:5555"

	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req)
	if rec1.Code != http.StatusOK {
		t.Fatalf("expected first request to succeed, got %d", rec1.Code)
	}

	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req)
	if rec2.Code != http.StatusTooManyRequests {
		t.Errorf("expected second immediate request to be rate limited, got %d", rec2.Code)
	}
}

func TestRateLimiter_TracksClientsIndependently(t *testing.T) {
	rl := NewRateLimiter(1)
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := rl.Middleware(next)

	reqA := httptest.NewRequest(http.MethodGet, "/", nil)
	reqA.RemoteAddr = "1.1.1.1:1"
	reqB := httptest.NewRequest(http.MethodGet, "/", nil)
	reqB.RemoteAddr = "2.2.2.2:2"

	recA := httptest.NewRecorder()
	handler.ServeHTTP(recA, reqA)
	recB := httptest.NewRecorder()
	handler.ServeHTTP(recB, reqB)

	if recA.Code != http.StatusOK || recB.Code != http.StatusOK {
		t.Error("expected independent clients to each get their own burst allowance")
	}
}
