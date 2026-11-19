package relyvo

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestResponseLimit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("large") == "yes" {
			_, _ = w.Write([]byte(strings.Repeat("x", maxResponseBytes+1)))
			return
		}
		// A page larger than the former 1 MiB limit must remain decodable.
		_, _ = w.Write([]byte(`{"data":{"items":[{"content":"` + strings.Repeat("x", (1<<20)+1) + `"}],"has_more":false}}`))
	}))
	defer server.Close()
	c := newTestClient(t, "http://example.test", server.URL)
	page, err := c.Chat.ListMessages(context.Background(), ChatActor{UserID: "123e4567-e89b-12d3-a456-426614174000"}, "c", ChatMessagePageOptions{})
	if err != nil || len(page.Items) != 1 || len(page.Items[0].Content) != (1<<20)+1 {
		t.Fatalf("large page error: %v", err)
	}
	err = c.do(context.Background(), request{method: http.MethodGet, base: server.URL, path: "/?large=yes", retryable: true})
	if !errors.Is(err, ErrResponseTooLarge) {
		t.Fatalf("expected overflow error, got %v", err)
	}
}
