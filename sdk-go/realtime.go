package relyvo

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// RealtimeService uses the project-key Realtime data plane, not the dashboard API.
type RealtimeService struct{ client *Client }

// RealtimeClientTokenInput selects the user and channels for browser credentials.
// Authorize the user and each requested channel in your application first.
type RealtimeClientTokenInput struct {
	UserID      string   `json:"user_id"`
	DisplayName string   `json:"display_name,omitempty"`
	Channels    []string `json:"channels"`
}

// RealtimeChannelToken contains credentials for one public channel name.
type RealtimeChannelToken struct {
	Channel           string    `json:"channel"`
	InternalChannel   string    `json:"internal_channel"`
	Type              string    `json:"type"`
	SubscriptionToken string    `json:"subscription_token"`
	ExpiresAt         time.Time `json:"expires_at"`
}

// RealtimeClientToken is a short-lived connection and subscription credential set.
type RealtimeClientToken struct {
	WSURL           string                 `json:"ws_url"`
	ConnectionToken string                 `json:"connection_token"`
	ExpiresAt       time.Time              `json:"expires_at"`
	Channels        []RealtimeChannelToken `json:"channels"`
}

// CreateClientToken mints credentials; it is never automatically retried.
func (s *RealtimeService) CreateClientToken(ctx context.Context, in RealtimeClientTokenInput) (*RealtimeClientToken, error) {
	if s.client.realtimeURL == "" {
		return nil, ErrRealtimeNotConfigured
	}
	var out RealtimeClientToken
	if err := s.client.do(ctx, request{method: http.MethodPost, base: s.client.realtimeURL, path: "/v1/realtime/client-tokens", body: in, out: &out}); err != nil {
		return nil, err
	}
	return &out, nil
}

// RealtimeEvent is one event to publish. Data is JSON-encoded as supplied; the
// server validates its payload and deployment-specific limits.
type RealtimeEvent struct {
	Channel string `json:"channel"`
	Event   string `json:"event"`
	Data    any    `json:"data"`
}

// RealtimePublishResult is the broker acknowledgement or a batch item outcome.
// Nil Accepted and Published indicate that the backend did not know the outcome.
type RealtimePublishResult struct {
	EventID     string     `json:"event_id"`
	Channel     string     `json:"channel"`
	Event       string     `json:"event"`
	Accepted    *bool      `json:"accepted"`
	Published   *bool      `json:"published"`
	PublishedAt *time.Time `json:"published_at,omitempty"`
	Outcome     string     `json:"outcome,omitempty"`
	Error       string     `json:"error,omitempty"`
	ErrorCode   string     `json:"error_code,omitempty"`
}

// Publish sends one event without automatic retry. A successful acknowledgement
// means broker acceptance, not delivery. On an explicit unknown outcome, the
// returned *Error.RealtimePublish contains the event ID; retrying risks duplicates.
func (s *RealtimeService) Publish(ctx context.Context, in RealtimeEvent) (*RealtimePublishResult, error) {
	if s.client.realtimeURL == "" {
		return nil, ErrRealtimeNotConfigured
	}
	var out RealtimePublishResult
	if err := s.client.do(ctx, request{method: http.MethodPost, base: s.client.realtimeURL, path: "/v1/realtime/events", body: in, out: &out}); err != nil {
		return nil, err
	}
	return &out, nil
}

// RealtimeBatchResult reports every item, including partial and unknown outcomes.
type RealtimeBatchResult struct {
	Results   []RealtimePublishResult `json:"results"`
	Published int                     `json:"published"`
	Failed    int                     `json:"failed"`
	Unknown   int                     `json:"unknown"`
}

// PublishBatch publishes a batch without automatic retry. Inspect each result;
// HTTP success does not mean that every item was published.
func (s *RealtimeService) PublishBatch(ctx context.Context, events []RealtimeEvent) (*RealtimeBatchResult, error) {
	if s.client.realtimeURL == "" {
		return nil, ErrRealtimeNotConfigured
	}
	var out RealtimeBatchResult
	body := struct {
		Events []RealtimeEvent `json:"events"`
	}{events}
	if err := s.client.do(ctx, request{method: http.MethodPost, base: s.client.realtimeURL, path: "/v1/realtime/events/batch", body: body, out: &out}); err != nil {
		return nil, err
	}
	return &out, nil
}

// RealtimeReceipt is best-effort metadata, not evidence of client delivery.
type RealtimeReceipt struct {
	EventID      string    `json:"event_id"`
	Channel      string    `json:"channel"`
	Event        string    `json:"event"`
	PayloadBytes int       `json:"payload_bytes"`
	RequestID    string    `json:"request_id,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

// RealtimeReceiptOptions specifies a page. Limit zero uses the server default;
// Cursor is the opaque NextCursor returned by the preceding page.
type RealtimeReceiptOptions struct {
	Limit  int
	Cursor string
}

// RealtimeReceiptPage is a single page. An empty NextCursor means the end.
type RealtimeReceiptPage struct {
	Events     []RealtimeReceipt `json:"events"`
	NextCursor string            `json:"next_cursor"`
}

// ListReceipts reads one page. A missing receipt does not prove non-publication;
// receipt persistence is best-effort. Pages are not a snapshot.
func (s *RealtimeService) ListReceipts(ctx context.Context, opts RealtimeReceiptOptions) (*RealtimeReceiptPage, error) {
	if s.client.realtimeURL == "" {
		return nil, ErrRealtimeNotConfigured
	}
	query := url.Values{}
	if opts.Limit != 0 {
		query.Set("limit", strconv.Itoa(opts.Limit))
	}
	if opts.Cursor != "" {
		query.Set("cursor", opts.Cursor)
	}
	path := "/v1/realtime/events/receipts"
	if q := query.Encode(); q != "" {
		path += "?" + q
	}
	var out RealtimeReceiptPage
	if err := s.client.do(ctx, request{method: http.MethodGet, base: s.client.realtimeURL, path: path, out: &out, retryable: true}); err != nil {
		return nil, err
	}
	return &out, nil
}
