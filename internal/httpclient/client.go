package httpclient

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"
)

const (
	defaultTimeout      = 15 * time.Second
	defaultMaxBodyBytes = 1 << 20 // 1 MiB
	defaultMaxLogBytes  = 64 << 10
	defaultMaxRetries   = 3
	defaultRetryBase    = 200 * time.Millisecond
)

var (
	ErrRequestTimeout   = errors.New("external request timeout")
	ErrResponseTooLarge = errors.New("external response too large")
)

type Config struct {
	Timeout      time.Duration
	MaxBodyBytes int64
	MaxLogBytes  int
	MaxRetries   int
	RetryBase    time.Duration
}

type Client struct {
	httpClient *http.Client
	cfg        Config
	logger     *slog.Logger
}

func New(cfg Config, logger *slog.Logger) *Client {
	if cfg.Timeout <= 0 {
		cfg.Timeout = defaultTimeout
	}

	if cfg.MaxBodyBytes <= 0 {
		cfg.MaxBodyBytes = defaultMaxBodyBytes
	}

	if cfg.MaxLogBytes <= 0 {
		cfg.MaxLogBytes = defaultMaxLogBytes
	}

	if cfg.MaxRetries < 0 {
		cfg.MaxRetries = defaultMaxRetries
	}

	if cfg.RetryBase <= 0 {
		cfg.RetryBase = defaultRetryBase
	}

	if logger == nil {
		logger = slog.Default()
	}

	return &Client{
		httpClient: &http.Client{
			Timeout: cfg.Timeout,
		},
		cfg:    cfg,
		logger: logger,
	}
}

type Request struct {
	Method string

	URL string

	Headers http.Header

	Query map[string]string

	Body any

	// RawBody allows callers such as webhook clients to provide
	// already-encoded request bodies.
	RawBody []byte

	// Retry controls whether this request is safe to retry.
	// POST requests should normally only set this when an idempotency
	// mechanism exists on the external API.
	Retry bool
}

type Response struct {
	StatusCode int
	Headers    http.Header
	Body       []byte
}

func (c *Client) Do(
	ctx context.Context,
	req Request,
) (*Response, error) {
	if req.Method == "" {
		req.Method = http.MethodGet
	}

	if req.URL == "" {
		return nil, errors.New("external request URL is required")
	}

	body, err := c.encodeBody(req)
	if err != nil {
		return nil, fmt.Errorf("encode external request body: %w", err)
	}

	url := req.URL

	if len(req.Query) > 0 {
		url, err = addQueryParams(url, req.Query)
		if err != nil {
			return nil, fmt.Errorf("build external request URL: %w", err)
		}
	}

	attempts := 1

	if req.Retry {
		attempts += c.cfg.MaxRetries
	}

	for attempt := 1; attempt <= attempts; attempt++ {
		response, err := c.doOnce(
			ctx,
			req.Method,
			url,
			req.Headers,
			body,
		)

		if err == nil {
			if shouldRetryStatus(response.StatusCode) &&
				attempt < attempts {

				delay := c.retryDelay(attempt)

				c.logger.Warn(
					"external API request retrying",
					"method", req.Method,
					"url", sanitizeURL(url),
					"status", response.StatusCode,
					"attempt", attempt,
					"retry_in", delay,
				)

				if err := sleepContext(ctx, delay); err != nil {
					return nil, err
				}

				continue
			}

			return response, nil
		}

		if !isRetryableError(err) || attempt >= attempts {
			return nil, err
		}

		delay := c.retryDelay(attempt)

		c.logger.Warn(
			"external API request failed, retrying",
			"method", req.Method,
			"url", sanitizeURL(url),
			"attempt", attempt,
			"retry_in", delay,
			"error", err,
		)

		if err := sleepContext(ctx, delay); err != nil {
			return nil, err
		}
	}

	return nil, errors.New("external request failed")
}

func (c *Client) doOnce(
	ctx context.Context,
	method string,
	url string,
	headers http.Header,
	body []byte,
) (*Response, error) {
	start := time.Now()

	requestBody := bytes.NewReader(body)

	req, err := http.NewRequestWithContext(
		ctx,
		method,
		url,
		requestBody,
	)
	if err != nil {
		return nil, fmt.Errorf("create external request: %w", err)
	}

	copyHeaders(req.Header, headers)

	if len(body) > 0 && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}

	if req.Header.Get("Accept") == "" {
		req.Header.Set("Accept", "application/json")
	}

	c.logRequest(req, body)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) ||
			errors.Is(err, context.DeadlineExceeded) {

			return nil, fmt.Errorf(
				"%w: %w",
				ErrRequestTimeout,
				err,
			)
		}

		return nil, fmt.Errorf("external request: %w", err)
	}

	defer resp.Body.Close()

	limited := io.LimitReader(
		resp.Body,
		c.cfg.MaxBodyBytes+1,
	)

	responseBody, err := io.ReadAll(limited)
	if err != nil {
		return nil, fmt.Errorf("read external response: %w", err)
	}

	if int64(len(responseBody)) > c.cfg.MaxBodyBytes {
		return nil, ErrResponseTooLarge
	}

	duration := time.Since(start)

	c.logResponse(
		req,
		resp,
		responseBody,
		duration,
	)

	return &Response{
		StatusCode: resp.StatusCode,
		Headers:    resp.Header.Clone(),
		Body:       responseBody,
	}, nil
}
