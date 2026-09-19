package httpclient

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"
)

func (c *Client) encodeBody(req Request) ([]byte, error) {
	if req.RawBody != nil {
		if int64(len(req.RawBody)) > c.cfg.MaxBodyBytes {
			return nil, fmt.Errorf(
				"request body exceeds %d bytes",
				c.cfg.MaxBodyBytes,
			)
		}

		return req.RawBody, nil
	}

	if req.Body == nil {
		return nil, nil
	}

	body, err := json.Marshal(req.Body)
	if err != nil {
		return nil, err
	}

	if int64(len(body)) > c.cfg.MaxBodyBytes {
		return nil, fmt.Errorf(
			"request body exceeds %d bytes",
			c.cfg.MaxBodyBytes,
		)
	}

	return body, nil
}

func addQueryParams(
	rawURL string,
	params map[string]string,
) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}

	q := u.Query()

	for key, value := range params {
		q.Set(key, value)
	}

	u.RawQuery = q.Encode()

	return u.String(), nil
}

func copyHeaders(dst, src http.Header) {
	for key, values := range src {
		for _, value := range values {
			dst.Add(key, value)
		}
	}
}

func shouldRetryStatus(status int) bool {
	switch status {
	case http.StatusRequestTimeout,
		http.StatusTooManyRequests,
		http.StatusInternalServerError,
		http.StatusBadGateway,
		http.StatusServiceUnavailable,
		http.StatusGatewayTimeout:
		return true

	default:
		return false
	}
}

func isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	if errors.Is(err, context.Canceled) {
		return false
	}

	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}

	var netErr net.Error

	if errors.As(err, &netErr) {
		return netErr.Timeout() || netErr.Temporary()
	}

	return true
}

func (c *Client) retryDelay(attempt int) time.Duration {
	delay := c.cfg.RetryBase

	for i := 1; i < attempt; i++ {
		delay *= 2
	}

	maxDelay := 5 * time.Second

	if delay > maxDelay {
		delay = maxDelay
	}

	return delay
}

func sleepContext(
	ctx context.Context,
	duration time.Duration,
) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()

	case <-timer.C:
		return nil
	}
}

func sanitizeURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}

	// Query parameters frequently contain API keys/tokens.
	q := u.Query()

	for key := range q {
		q.Set(key, "[REDACTED]")
	}

	u.RawQuery = q.Encode()

	return u.String()
}
