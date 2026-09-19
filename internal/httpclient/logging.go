package httpclient

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

func (c *Client) logRequest(
	req *http.Request,
	body []byte,
) {
	headers := sanitizeHeaders(req.Header)

	logBody := sanitizeBody(
		req.Header.Get("Content-Type"),
		body,
		c.cfg.MaxLogBytes,
	)

	c.logger.Info(
		"external API request",
		"method", req.Method,
		"url", sanitizeURL(req.URL.String()),
		"headers", headers,
		"body", logBody,
	)
}

func (c *Client) logResponse(
	req *http.Request,
	resp *http.Response,
	body []byte,
	duration time.Duration,
) {
	headers := sanitizeHeaders(resp.Header)

	logBody := sanitizeBody(
		resp.Header.Get("Content-Type"),
		body,
		c.cfg.MaxLogBytes,
	)

	level := slog.LevelInfo

	if resp.StatusCode >= 500 {
		level = slog.LevelError
	} else if resp.StatusCode >= 400 {
		level = slog.LevelWarn
	}

	c.logger.Log(
		// nil,
		req.Context(),
		level,
		"external API response",
		"method", req.Method,
		"url", sanitizeURL(req.URL.String()),
		"status", resp.StatusCode,
		"duration", duration,
		"headers", headers,
		"body", logBody,
	)
}

func sanitizeHeaders(headers http.Header) map[string][]string {
	result := make(map[string][]string, len(headers))

	for key, values := range headers {
		lower := strings.ToLower(key)

		switch lower {
		case "authorization",
			"proxy-authorization",
			"cookie",
			"set-cookie",
			"x-api-key",
			"x-auth-token",
			"x-access-token":

			result[key] = []string{"[REDACTED]"}

		default:
			result[key] = values
		}
	}

	return result
}

func sanitizeBody(
	contentType string,
	body []byte,
	maxBytes int,
) string {
	if len(body) == 0 {
		return ""
	}

	if maxBytes <= 0 {
		maxBytes = defaultMaxLogBytes
	}

	if len(body) > maxBytes {
		body = body[:maxBytes]
	}

	// Never dump binary/multipart bodies.
	lowerContentType := strings.ToLower(contentType)

	if strings.Contains(lowerContentType, "multipart/") ||
		strings.Contains(lowerContentType, "application/octet-stream") ||
		strings.Contains(lowerContentType, "image/") ||
		strings.Contains(lowerContentType, "audio/") ||
		strings.Contains(lowerContentType, "video/") {

		return "[binary body omitted]"
	}

	if strings.Contains(lowerContentType, "application/json") {
		return sanitizeJSON(body)
	}

	if !isLikelyText(body) {
		return "[binary body omitted]"
	}

	return string(body)
}

func sanitizeJSON(body []byte) string {
	var value any

	if err := json.Unmarshal(body, &value); err != nil {
		return truncate(string(body), defaultMaxLogBytes)
	}

	redactJSON(value)

	encoded, err := json.Marshal(value)
	if err != nil {
		return "[invalid JSON]"
	}

	return string(encoded)
}

func redactJSON(value any) {
	switch v := value.(type) {
	case map[string]any:
		for key, value := range v {
			if isSensitiveField(key) {
				v[key] = "[REDACTED]"
				continue
			}

			redactJSON(value)
		}

	case []any:
		for _, item := range v {
			redactJSON(item)
		}
	}
}

func isSensitiveField(key string) bool {
	switch strings.ToLower(key) {
	case "password",
		"password_hash",
		"token",
		"access_token",
		"refresh_token",
		"id_token",
		"authorization",
		"api_key",
		"apikey",
		"secret",
		"client_secret",
		"private_key",
		"signature",
		"otp",
		"pin",
		"cvv",
		"card_number":
		return true

	default:
		return false
	}
}

func isLikelyText(body []byte) bool {
	return bytes.IndexByte(body, 0) == -1
}

func truncate(value string, maxBytes int) string {
	if len(value) <= maxBytes {
		return value
	}

	return value[:maxBytes] + "...[truncated]"
}
