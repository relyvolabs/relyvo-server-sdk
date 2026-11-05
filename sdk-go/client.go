package relyvo

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	defaultTimeout    = 10 * time.Second
	defaultMaxRetries = 2
	defaultBaseDelay  = 200 * time.Millisecond

	// maxResponseBytes bounds what the SDK will read from one response. An error
	// page from a proxy must never be able to exhaust a server's memory.
	maxResponseBytes = 1 << 20

	// maxRetryDelay caps a single backoff, including a hostile Retry-After.
	maxRetryDelay = 30 * time.Second
)

// Config configures the client.
//
// APIKey and APIURL are required. ChatURL is required only when chat methods are
// used; without it they return ErrNotConfigured instead of guessing an origin.
type Config struct {
	// APIKey is the project API key, `rv_<keyID>_<secret>`. Server-side only.
	APIKey string

	// APIURL is the origin where `/v1/api/...` resolves. Through a dashboard
	// deployment that is `https://<host>/api`; against the API directly it is
	// `http://localhost:8080`. The SDK appends the versioned path and never
	// rewrites the prefix.
	APIURL string

	// ChatURL is the chat service origin, for chat methods.
	ChatURL string

	// HTTPClient is optional. When set, Timeout is ignored and the supplied
	// client's own configuration is respected.
	HTTPClient *http.Client

	// Timeout bounds one request attempt. Defaults to 10s.
	Timeout time.Duration

	// MaxRetries is the number of *additional* attempts for idempotent calls.
	// Defaults to 2; zero means one attempt only.
	MaxRetries int

	// BaseDelay is the first backoff step. Defaults to 200ms. Tests set it low.
	BaseDelay time.Duration

	// UserAgent adds a suffix to the SDK User-Agent, so a service can be
	// identified in Relyvo's logs.
	UserAgent string
}

// Client is safe for concurrent use by multiple goroutines.
type Client struct {
	apiKey     string
	apiURL     string
	chatURL    string
	http       *http.Client
	maxRetries int
	baseDelay  time.Duration
	userAgent  string

	// Video mints media credentials.
	Video *VideoService
	// Chat manages chat identities and tokens.
	Chat *ChatService
}

// New validates the configuration and builds a client.
func New(cfg Config) (*Client, error) {
	apiKey := strings.TrimSpace(cfg.APIKey)
	if apiKey == "" {
		return nil, errors.New("relyvo: Config.APIKey is required")
	}
	if strings.HasPrefix(strings.ToLower(apiKey), "bearer ") {
		return nil, errors.New("relyvo: Config.APIKey must be the raw rv_<keyID>_<secret> value, without a Bearer prefix")
	}

	apiURL, err := normalizeBaseURL("APIURL", cfg.APIURL, true)
	if err != nil {
		return nil, err
	}
	chatURL, err := normalizeBaseURL("ChatURL", cfg.ChatURL, false)
	if err != nil {
		return nil, err
	}

	httpClient := cfg.HTTPClient
	if httpClient == nil {
		timeout := cfg.Timeout
		if timeout <= 0 {
			timeout = defaultTimeout
		}
		httpClient = &http.Client{Timeout: timeout}
	}

	maxRetries := cfg.MaxRetries
	if maxRetries == 0 {
		maxRetries = defaultMaxRetries
	}
	if maxRetries < 0 {
		maxRetries = 0
	}

	baseDelay := cfg.BaseDelay
	if baseDelay <= 0 {
		baseDelay = defaultBaseDelay
	}

	agent := userAgent
	if suffix := strings.TrimSpace(cfg.UserAgent); suffix != "" {
		agent += " " + suffix
	}

	client := &Client{
		apiKey:     apiKey,
		apiURL:     apiURL,
		chatURL:    chatURL,
		http:       httpClient,
		maxRetries: maxRetries,
		baseDelay:  baseDelay,
		userAgent:  agent,
	}
	client.Video = &VideoService{client: client}
	client.Chat = &ChatService{client: client}

	return client, nil
}

// normalizeBaseURL validates an origin and strips the trailing slash so path
// joining never produces a double slash.
func normalizeBaseURL(field, value string, required bool) (string, error) {
	trimmed := strings.TrimRight(strings.TrimSpace(value), "/")
	if trimmed == "" {
		if required {
			return "", fmt.Errorf("relyvo: Config.%s is required", field)
		}
		return "", nil
	}

	parsed, err := url.Parse(trimmed)
	if err != nil || parsed.Host == "" {
		return "", fmt.Errorf("relyvo: Config.%s must be an absolute URL", field)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("relyvo: Config.%s must use http or https", field)
	}

	return trimmed, nil
}

// request describes one HTTP call.
type request struct {
	method    string
	base      string
	path      string
	body      any
	out       any
	retryable bool
}

// do executes a request, retrying only when the call is idempotent and the
// failure is one that a later attempt could survive.
func (c *Client) do(ctx context.Context, req request) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	payload, err := encodeBody(req.body)
	if err != nil {
		return err
	}

	attempts := 1
	if req.retryable {
		attempts += c.maxRetries
	}

	var lastErr error
	for attempt := 0; attempt < attempts; attempt++ {
		if attempt > 0 && !sleep(ctx, c.delay(attempt-1, lastErr)) {
			return ctx.Err()
		}

		body, err := c.send(ctx, req, payload)
		if err == nil {
			return decodeBody(req.out, body)
		}

		lastErr = err
		if !isRetryable(err) {
			return err
		}
	}

	return lastErr
}

// send performs exactly one attempt.
func (c *Client) send(ctx context.Context, req request, payload []byte) ([]byte, error) {
	endpoint := strings.TrimRight(req.base, "/") + req.path

	var bodyReader io.Reader
	if payload != nil {
		bodyReader = bytes.NewReader(payload)
	}

	httpReq, err := http.NewRequestWithContext(ctx, req.method, endpoint, bodyReader)
	if err != nil {
		return nil, &Error{Code: "transport_error", Message: err.Error(), err: err}
	}

	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("User-Agent", c.userAgent)
	if payload != nil {
		httpReq.Header.Set("Content-Type", "application/json")
	}

	response, err := c.http.Do(httpReq)
	if err != nil {
		return nil, &Error{Code: "transport_error", Message: err.Error(), err: err}
	}
	defer func() {
		_ = response.Body.Close()
	}()

	raw, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes))
	if err != nil {
		return nil, &Error{Code: "transport_error", Message: err.Error(), err: err}
	}

	if response.StatusCode >= 200 && response.StatusCode < 300 {
		return raw, nil
	}

	return nil, newAPIError(response, raw)
}

// delay computes the wait before the next attempt: exponential with jitter, and
// never shorter than a Retry-After the service asked for.
func (c *Client) delay(attempt int, lastErr error) time.Duration {
	var retryAfter time.Duration
	var apiErr *Error
	if errors.As(lastErr, &apiErr) {
		retryAfter = apiErr.RetryAfter
	}

	base := c.baseDelay << attempt
	if base <= 0 {
		base = c.baseDelay
	}

	jitter := time.Duration(rand.Int63n(int64(base/2) + 1))
	wait := base + jitter

	if retryAfter > wait {
		wait = retryAfter
	}
	if wait > maxRetryDelay {
		wait = maxRetryDelay
	}

	return wait
}

// requireChat guards chat methods so a missing origin is a clear configuration
// error rather than a request to an empty URL.
func (c *Client) requireChat() error {
	if c.chatURL == "" {
		return ErrNotConfigured
	}
	return nil
}

func encodeBody(body any) ([]byte, error) {
	if body == nil {
		return nil, nil
	}

	encoded, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("relyvo: encoding request body: %w", err)
	}
	return encoded, nil
}

func decodeBody(out any, body []byte) error {
	if out == nil || len(body) == 0 {
		return nil
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("relyvo: decoding response: %w", err)
	}
	return nil
}

func sleep(ctx context.Context, d time.Duration) bool {
	if d <= 0 {
		return ctx.Err() == nil
	}

	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
