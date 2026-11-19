package relyvo

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestChatTypingAndPresence(t *testing.T) {
	var writes atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer rv_test_key_secret" || r.Header.Get("X-Relyvo-Chat-User-ID") != "123e4567-e89b-12d3-a456-426614174000" {
			t.Error("missing auth")
		}
		switch r.URL.Path {
		case "/v1/chat/channels/c/typing":
			writes.Add(1)
			if r.Method != "POST" {
				t.Error("method")
			}
			w.WriteHeader(503)
		case "/v1/chat/channels/c/presence":
			if r.Method != "GET" {
				t.Error("method")
			}
			_, _ = w.Write([]byte(`{"data":{"channel_id":"c","presence":[{"client_id":"conn","user_id":"u","info":{"x":1}}]}}`))
		default:
			t.Errorf("bad path %s", r.URL.Path)
		}
	}))
	defer server.Close()
	c := newTestClient(t, "http://example.test", server.URL)
	a := ChatActor{UserID: "123e4567-e89b-12d3-a456-426614174000"}
	if err := c.Chat.Typing(context.Background(), a, "c"); err == nil || writes.Load() != 1 {
		t.Fatalf("typing %v calls %d", err, writes.Load())
	}
	p, err := c.Chat.Presence(context.Background(), a, "c")
	if err != nil || p.ChannelID != "c" || len(p.Presence) != 1 || p.Presence[0].UserID != "u" || string(p.Presence[0].Info) != `{"x":1}` {
		t.Fatalf("presence %+v %v", p, err)
	}
}
