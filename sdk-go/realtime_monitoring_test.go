package relyvo

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRealtimeUnresolvedAndMonitoring(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer rv_test_key_secret" || r.Method != "GET" {
			t.Error("invalid auth or method")
		}
		switch r.URL.Path {
		case "/prefix/v1/realtime/events/unresolved":
			if r.URL.Query().Get("cursor") != "a+b" || r.URL.Query().Get("limit") != "5" {
				t.Error("invalid page")
			}
			_, _ = w.Write([]byte(`{"candidates":[{"event_id":"e","channel":"c","status":"unresolved","created_at":"2026-01-01T00:00:00Z"}],"next_cursor":"next"}`))
		case "/prefix/v1/realtime/monitoring/overview":
			if r.URL.Query().Get("window") != "1h" {
				t.Error("invalid window")
			}
			_, _ = w.Write([]byte(`{"project_id":"p","window":"1h","published":3,"top_channels":[{"channel":"c","events":2}],"gateway_error":"unavailable","generated_at":"2026-01-01T00:00:00Z"}`))
		default:
			t.Errorf("unexpected %s", r.URL.Path)
		}
	}))
	defer server.Close()
	c := realtimeTestClient(t, server.URL)
	page, err := c.Realtime.ListUnresolved(context.Background(), RealtimeReceiptOptions{Limit: 5, Cursor: "a+b"})
	if err != nil || len(page.Candidates) != 1 || page.NextCursor != "next" || page.Candidates[0].Status != "unresolved" {
		t.Fatalf("%+v %v", page, err)
	}
	overview, err := c.Realtime.MonitoringOverview(context.Background(), "1h")
	if err != nil || overview.Published != 3 || len(overview.TopChannels) != 1 || overview.Gateway != nil || overview.GatewayError != "unavailable" {
		t.Fatalf("%+v %v", overview, err)
	}
}
