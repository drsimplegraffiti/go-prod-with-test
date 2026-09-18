package models

import "time"

// RequestLog is an audit record of a single HTTP request/response pair,
// captured by the BodyDump middleware and persisted for later inspection
// (debugging, support, compliance).
type RequestLog struct {
	ID           int64
	RequestID    string
	Method       string
	Path         string
	StatusCode   int
	RequestBody  string
	ResponseBody string
	RemoteAddr   string
	DurationMS   int64
	CreatedAt    time.Time
}
