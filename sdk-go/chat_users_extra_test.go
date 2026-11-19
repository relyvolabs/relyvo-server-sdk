package relyvo

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestChatProjectUsersAndTokenOptions(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Header.Get("Authorization") != "Bearer rv_test_key_secret" || r.Header.Get("X-Relyvo-Chat-User-ID") != "" {
			t.Error("incorrect project authentication")
		}
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/chat/users/user":
			_, _ = w.Write([]byte(`{"data":{"id":"user","display_name":"Alice"}}`))
		case r.Method == http.MethodPatch && r.URL.Path == "/v1/chat/users/user":
			body := decodeJSONBody(t, r)
			if body["display_name"] != "" || body["metadata"] == nil {
				t.Errorf("update: %#v", body)
			}
			_, _ = w.Write([]byte(`{"data":{"id":"user","display_name":""}}`))
		case r.Method == http.MethodPost && r.URL.Path == "/v1/chat/users/user/tokens":
			body := decodeJSONBody(t, r)
			if calls.Load() == 3 && body["ttl_seconds"] != float64(30) {
				t.Errorf("ttl: %#v", body)
			}
			if calls.Load() == 4 && len(body) != 0 {
				t.Errorf("legacy body: %#v", body)
			}
			_, _ = w.Write([]byte(`{"data":{"token":"t","expires_at":"2026-01-01T00:00:00Z"}}`))
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()
	c := newTestClient(t, "http://example.test", server.URL)
	ctx := context.Background()
	user, err := c.Chat.GetUser(ctx, "user")
	if err != nil || user.DisplayName != "Alice" {
		t.Fatalf("%+v %v", user, err)
	}
	empty := ""
	user, err = c.Chat.UpdateUser(ctx, "user", ChatUserUpdate{DisplayName: &empty, Metadata: map[string]any{}})
	if err != nil || user.ID != "user" {
		t.Fatalf("%+v %v", user, err)
	}
	token, err := c.Chat.CreateUserTokenWithOptions(ctx, "user", ChatUserTokenOptions{TTLSeconds: 30})
	if err != nil || token.Token != "t" {
		t.Fatalf("%+v %v", token, err)
	}
	_, err = c.Chat.CreateUserToken(ctx, "user")
	if err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 4 {
		t.Fatal(calls.Load())
	}
}

func TestChatProjectUserRetryPolicy(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1); w.WriteHeader(503) }))
	defer server.Close()
	c := newTestClient(t, "http://example.test", server.URL)
	_, _ = c.Chat.UpdateUser(context.Background(), "user", ChatUserUpdate{})
	if calls.Load() != 1 {
		t.Fatal("PATCH retried")
	}
	_, _ = c.Chat.CreateUserTokenWithOptions(context.Background(), "user", ChatUserTokenOptions{TTLSeconds: 1})
	if calls.Load() != 2 {
		t.Fatal("token retried")
	}
	_, _ = c.Chat.GetUser(context.Background(), "user")
	if calls.Load() != 4 {
		t.Fatal("GET not retried")
	}
}
