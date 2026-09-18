package service

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/example/goapi/internal/models"
	"github.com/example/goapi/internal/repository"
)

// maxLoggedBodyBytes caps how much of a request/response body gets
// persisted, so a large upload/download doesn't bloat the audit table or
// the write itself.
const maxLoggedBodyBytes = 8 * 1024 // 8KB

// AuditService persists a record of each HTTP request/response for later
// inspection (debugging, support, compliance), truncating oversized bodies.
type AuditService struct {
	logs *repository.RequestLogRepository
}

// NewAuditService constructs an AuditService.
func NewAuditService(logs *repository.RequestLogRepository) *AuditService {
	return &AuditService{logs: logs}
}

// LogRequest persists one request/response pair. It's designed to be called
// from the BodyDump middleware callback, after the response has already
// been sent to the client — so a slow or failed write here never delays or
// breaks the actual HTTP response; failures are only logged.
func (s *AuditService) LogRequest(
	ctx context.Context,
	r *http.Request,
	status int,
	duration time.Duration,
	requestID string,
	reqBody, resBody []byte,
) {
	entry := &models.RequestLog{
		RequestID:    requestID,
		Method:       r.Method,
		Path:         r.URL.Path,
		StatusCode:   status,
		RequestBody:  truncate(reqBody, maxLoggedBodyBytes),
		ResponseBody: truncate(resBody, maxLoggedBodyBytes),
		RemoteAddr:   r.RemoteAddr,
		DurationMS:   duration.Milliseconds(),
	}

	if _, err := s.logs.Create(ctx, entry); err != nil {
		slog.Error("failed to persist request log", "error", err, "request_id", requestID)
	}
}

func truncate(b []byte, max int) string {
	if len(b) > max {
		b = b[:max]
	}
	return string(b)
}
