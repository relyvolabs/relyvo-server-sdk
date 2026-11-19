package relyvo

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync/atomic"
	"testing"
	"time"
)

func realtimeTestClient(t *testing.T, base string) *Client {
	t.Helper()
	c, err := New(Config{APIKey: "rv_test_key_secret", APIURL: "http://example.test", RealtimeURL: base + "/prefix/", MaxRetries: 1, BaseDelay: time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestRealtimeContract(t *testing.T) {
	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Header.Get("Authorization") != "Bearer rv_test_key_secret" {
			t.Error("missing project bearer")
		}
		if r.Header.Get("Accept") != "application/json" {
			t.Error("missing accept")
		}
		switch r.URL.Path {
		case "/prefix/v1/realtime/client-tokens":
			if r.Method != "POST" || r.Header.Get("Content-Type") != "application/json" {
				t.Error("invalid token request")
			}
			body := decodeJSONBody(t, r)
			want := map[string]any{"user_id": "u", "display_name": "U", "channels": []any{"private-orders"}}
			if !reflect.DeepEqual(body, want) {
				t.Errorf("token body: %#v", body)
			}
			_, _ = w.Write([]byte(`{"ws_url":"wss://example/ws","connection_token":"conn","expires_at":"2026-01-01T00:00:00Z","channels":[{"channel":"private-orders","internal_channel":"rt:p:private-orders","type":"private","subscription_token":"sub","expires_at":"2026-01-01T00:00:00Z"}]}`))
		case "/prefix/v1/realtime/events":
			body := decodeJSONBody(t, r)
			if body["channel"] != "private-orders" || body["event"] != "order.shipped" {
				t.Errorf("event body: %#v", body)
			}
			_, _ = w.Write([]byte(`{"event_id":"e1","channel":"private-orders","event":"order.shipped","accepted":true,"published":true,"published_at":"2026-01-01T00:00:00Z"}`))
		case "/prefix/v1/realtime/events/batch":
			body := decodeJSONBody(t, r)
			if len(body["events"].([]any)) != 2 {
				t.Error("batch body")
			}
			_, _ = w.Write([]byte(`{"published":1,"failed":0,"unknown":1,"results":[{"event_id":"e1","accepted":true,"published":true},{"event_id":"e2","outcome":"unknown","error_code":"publish_outcome_unknown"}]}`))
		case "/prefix/v1/realtime/events/receipts":
			if r.Method != "GET" || r.URL.Query().Get("limit") != "1" {
				t.Error("invalid receipt request")
			}
			if r.URL.Query().Get("cursor") == "" {
				_, _ = w.Write([]byte(`{"events":[{"event_id":"e1","channel":"c","event":"e","payload_bytes":12,"request_id":"r","created_at":"2026-01-01T00:00:00Z"}],"next_cursor":"a+b/= &"}`))
			} else if r.URL.Query().Get("cursor") == "a+b/= &" {
				_, _ = w.Write([]byte(`{"events":[],"next_cursor":""}`))
			} else {
				t.Errorf("bad cursor %q", r.URL.Query().Get("cursor"))
			}
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()
	c := realtimeTestClient(t, server.URL)
	if c.Call != c.Video || c.Realtime == nil {
		t.Fatal("services not wired")
	}
	token, err := c.Realtime.CreateClientToken(context.Background(), RealtimeClientTokenInput{UserID: "u", DisplayName: "U", Channels: []string{"private-orders"}})
	if err != nil || token.ConnectionToken != "conn" || len(token.Channels) != 1 || token.Channels[0].InternalChannel != "rt:p:private-orders" {
		t.Fatalf("token=%+v err=%v", token, err)
	}
	ev := RealtimeEvent{Channel: "private-orders", Event: "order.shipped", Data: map[string]any{"n": 1}}
	result, err := c.Realtime.Publish(context.Background(), ev)
	if err != nil || result.Published == nil || !*result.Published || result.PublishedAt == nil {
		t.Fatalf("publish=%+v err=%v", result, err)
	}
	batch, err := c.Realtime.PublishBatch(context.Background(), []RealtimeEvent{ev, ev})
	if err != nil || batch.Unknown != 1 || len(batch.Results) != 2 || batch.Results[1].Accepted != nil {
		t.Fatalf("batch=%+v err=%v", batch, err)
	}
	page, err := c.Realtime.ListReceipts(context.Background(), RealtimeReceiptOptions{Limit: 1})
	if err != nil || len(page.Events) != 1 || page.Events[0].RequestID != "r" {
		t.Fatalf("page=%+v err=%v", page, err)
	}
	page, err = c.Realtime.ListReceipts(context.Background(), RealtimeReceiptOptions{Limit: 1, Cursor: page.NextCursor})
	if err != nil || page.NextCursor != "" || len(page.Events) != 0 {
		t.Fatalf("page=%+v err=%v", page, err)
	}
	if calls != 5 {
		t.Fatalf("calls=%d", calls)
	}
}

func TestRealtimeFailureAndRetryPolicy(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.URL.Path == "/prefix/v1/realtime/events" {
			w.Header().Set("X-Request-Id", "req")
			w.WriteHeader(503)
			_, _ = w.Write([]byte(`{"event_id":"uncertain","channel":"c","event":"e","outcome":"unknown","error_code":"publish_outcome_unknown","error":"may duplicate"}`))
			return
		}
		w.WriteHeader(503)
		_, _ = w.Write([]byte(`{"error":{"code":"unavailable","message":"down"}}`))
	}))
	defer server.Close()
	c := realtimeTestClient(t, server.URL)
	_, err := c.Realtime.Publish(context.Background(), RealtimeEvent{})
	api, ok := AsError(err)
	if !ok || api.Code != "publish_outcome_unknown" || api.RequestID != "req" || api.RealtimePublish == nil || api.RealtimePublish.EventID != "uncertain" || api.RealtimePublish.Accepted != nil {
		t.Fatalf("unknown error: %+v", err)
	}
	if api.Retryable() {
		t.Fatal("unknown publish outcome must not be marked retryable")
	}
	if calls.Load() != 1 {
		t.Fatal("publish retried")
	}
	_, _ = c.Realtime.PublishBatch(context.Background(), nil)
	_, _ = c.Realtime.CreateClientToken(context.Background(), RealtimeClientTokenInput{})
	if calls.Load() != 3 {
		t.Fatal("POST retried")
	}
	_, _ = c.Realtime.ListReceipts(context.Background(), RealtimeReceiptOptions{})
	if calls.Load() != 5 {
		t.Fatalf("GET not retried: %d", calls.Load())
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = c.Realtime.Publish(ctx, RealtimeEvent{})
	if !errors.Is(err, context.Canceled) || calls.Load() != 5 {
		t.Fatalf("cancellation: %v", err)
	}
}

func TestRealtimeConfigurationAndEncoding(t *testing.T) {
	c, err := New(Config{APIKey: "key", APIURL: "http://example.test"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = c.Realtime.ListReceipts(context.Background(), RealtimeReceiptOptions{}); !errors.Is(err, ErrRealtimeNotConfigured) {
		t.Fatal(err)
	}
	if _, err = New(Config{APIKey: "key", APIURL: "http://example.test", RealtimeURL: "ftp://bad"}); err == nil {
		t.Fatal("invalid URL accepted")
	}
	c = realtimeTestClient(t, "http://example.test")
	_, err = c.Realtime.Publish(context.Background(), RealtimeEvent{Data: make(chan int)})
	if err == nil {
		t.Fatal("invalid JSON accepted")
	}
}
