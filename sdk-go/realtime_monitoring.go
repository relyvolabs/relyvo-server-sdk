package relyvo

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// RealtimeUnresolvedCandidate is an unconfirmed usage candidate. It is not proof
// of publication, non-publication, delivery, or a broker attempt.
type RealtimeUnresolvedCandidate struct {
	EventID   string    `json:"event_id"`
	Channel   string    `json:"channel"`
	CreatedAt time.Time `json:"created_at"`
	Status    string    `json:"status"`
}

// RealtimeUnresolvedPage contains candidates and the opaque continuation cursor.
type RealtimeUnresolvedPage struct {
	Candidates []RealtimeUnresolvedCandidate `json:"candidates"`
	NextCursor string                        `json:"next_cursor"`
}

// ListUnresolved returns one non-snapshot page of unresolved candidates.
func (s *RealtimeService) ListUnresolved(ctx context.Context, opts RealtimeReceiptOptions) (*RealtimeUnresolvedPage, error) {
	if s.client.realtimeURL == "" {
		return nil, ErrRealtimeNotConfigured
	}
	q := url.Values{}
	if opts.Limit != 0 {
		q.Set("limit", strconv.Itoa(opts.Limit))
	}
	if opts.Cursor != "" {
		q.Set("cursor", opts.Cursor)
	}
	path := "/v1/realtime/events/unresolved"
	if encoded := q.Encode(); encoded != "" {
		path += "?" + encoded
	}
	var v RealtimeUnresolvedPage
	if err := s.client.do(ctx, request{method: http.MethodGet, base: s.client.realtimeURL, path: path, out: &v, retryable: true}); err != nil {
		return nil, err
	}
	return &v, nil
}

// RealtimeChannelCount is one channel's published event count.
type RealtimeChannelCount struct {
	Channel string `json:"channel"`
	Events  int64  `json:"events"`
}

// RealtimeGatewayOverview describes gateway counters when the server has them.
type RealtimeGatewayOverview struct {
	Connections   int64 `json:"connections"`
	Subscriptions int64 `json:"subscriptions"`
}

// RealtimeOverview is the project-scoped monitoring response.
type RealtimeOverview struct {
	ProjectID    string                   `json:"project_id"`
	Window       string                   `json:"window"`
	Published    int64                    `json:"published"`
	Failed       int64                    `json:"failed"`
	PayloadBytes int64                    `json:"payload_bytes"`
	TopChannels  []RealtimeChannelCount   `json:"top_channels"`
	Gateway      *RealtimeGatewayOverview `json:"gateway,omitempty"`
	GatewayError string                   `json:"gateway_error,omitempty"`
	PendingUsage int64                    `json:"pending_usage"`
	GeneratedAt  time.Time                `json:"generated_at"`
}

// MonitoringOverview retrieves monitoring for the selected window (15m, 1h,
// 24h, or 7d); an empty window selects the server default.
func (s *RealtimeService) MonitoringOverview(ctx context.Context, window string) (*RealtimeOverview, error) {
	if s.client.realtimeURL == "" {
		return nil, ErrRealtimeNotConfigured
	}
	path := "/v1/realtime/monitoring/overview"
	if window != "" {
		q := url.Values{}
		q.Set("window", window)
		path += "?" + q.Encode()
	}
	var v RealtimeOverview
	if err := s.client.do(ctx, request{method: http.MethodGet, base: s.client.realtimeURL, path: path, out: &v, retryable: true}); err != nil {
		return nil, err
	}
	return &v, nil
}
