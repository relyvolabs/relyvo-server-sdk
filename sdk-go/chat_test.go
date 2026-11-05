package relyvo

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync/atomic"
	"testing"
)

func TestUpsertUserUsesTheDocumentedContract(t *testing.T) {
	t.Parallel()

	wantBody := decodeFixture[map[string]any](t, "requests/chat-user.json")
	response := fixture(t, "responses/chat-user-created.json")

	var (
		gotPath string
		gotBody map[string]any
	)

	chatServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotBody = decodeJSONBody(t, r)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write(response)
	}))
	defer chatServer.Close()

	client := newTestClient(t, "https://api.invalid", chatServer.URL)
	result, err := client.Chat.UpsertUser(context.Background(), ChatUserInput{
		ExternalUserID: "user_7",
		DisplayName:    "Ada",
		AvatarURL:      "https://example.com/ada.png",
		Metadata:       map[string]any{"plan": "pro"},
	})
	if err != nil {
		t.Fatalf("UpsertUser: %v", err)
	}

	if gotPath != "/v1/chat/users" {
		t.Fatalf("path = %s", gotPath)
	}
	if !reflect.DeepEqual(gotBody, wantBody) {
		t.Fatalf("body = %#v, want %#v", gotBody, wantBody)
	}

	if !result.Created {
		t.Fatal("Created must be true for the created fixture")
	}
	if result.User.ID != "0f5b3a7e-2c4d-4a91-9f60-5b1f0f4e6a11" {
		t.Fatalf("User.ID = %q", result.User.ID)
	}
	if result.User.ExternalUserID != "user_7" || result.User.DisplayName != "Ada" {
		t.Fatalf("unexpected user: %+v", result.User)
	}
}

// TestUpsertUserRetriesARetryableFailure pins that the idempotent call is retried,
// because the chat service deduplicates on (project, external_user_id).
func TestUpsertUserRetriesARetryableFailure(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32
	chatServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write(fixture(t, "errors/unauthenticated.json"))
			return
		}
		respondWith(t, http.StatusOK, "responses/chat-user-existing.json")(w, r)
	}))
	defer chatServer.Close()

	client := newTestClient(t, "https://api.invalid", chatServer.URL)
	result, err := client.Chat.UpsertUser(context.Background(), ChatUserInput{ExternalUserID: "user_7"})
	if err != nil {
		t.Fatalf("UpsertUser: %v", err)
	}
	if calls.Load() != 2 {
		t.Fatalf("made %d calls, want 2", calls.Load())
	}
	if result.Created {
		t.Fatal("the second response reports an existing user")
	}
}

func TestCreateUserTokenUsesTheDocumentedContract(t *testing.T) {
	t.Parallel()

	response := fixture(t, "responses/chat-user-token.json")

	var (
		gotMethod string
		gotPath   string
		gotBody   map[string]any
	)

	chatServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotBody = decodeJSONBody(t, r)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write(response)
	}))
	defer chatServer.Close()

	client := newTestClient(t, "https://api.invalid", chatServer.URL)
	token, err := client.Chat.CreateUserToken(context.Background(), "0f5b3a7e-2c4d-4a91-9f60-5b1f0f4e6a11")
	if err != nil {
		t.Fatalf("CreateUserToken: %v", err)
	}

	if gotMethod != http.MethodPost {
		t.Fatalf("method = %s", gotMethod)
	}
	if gotPath != "/v1/chat/users/0f5b3a7e-2c4d-4a91-9f60-5b1f0f4e6a11/tokens" {
		t.Fatalf("path = %s", gotPath)
	}
	if len(gotBody) != 0 {
		t.Fatalf("body = %#v, want an empty object", gotBody)
	}

	if token.Token == "" || token.TokenType != "Bearer" || token.Scope != "chat.user" {
		t.Fatalf("unexpected token: %+v", token)
	}
}

// TestCreateUserTokenIsNotRetried pins that a token is never minted twice by the
// SDK behind the caller's back.
func TestCreateUserTokenIsNotRetried(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32
	chatServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer chatServer.Close()

	client := newTestClient(t, "https://api.invalid", chatServer.URL)
	if _, err := client.Chat.CreateUserToken(context.Background(), "user-id"); err == nil {
		t.Fatal("expected an error")
	}
	if calls.Load() != 1 {
		t.Fatalf("made %d calls, want exactly 1", calls.Load())
	}
}

func TestChatRequiresChatURL(t *testing.T) {
	t.Parallel()

	client := newTestClient(t, "https://api.invalid", "")

	if _, err := client.Chat.UpsertUser(context.Background(), ChatUserInput{ExternalUserID: "user_7"}); !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("UpsertUser error = %v, want ErrNotConfigured", err)
	}
	if _, err := client.Chat.CreateUserToken(context.Background(), "user-id"); !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("CreateUserToken error = %v, want ErrNotConfigured", err)
	}
}

func TestWhoamiResolvesTheProject(t *testing.T) {
	t.Parallel()

	var gotPath string
	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"api_key":{"id":"6a0d2f54-9b3c-4c1e-8f27-1d2b3c4d5e6f","key_id":"key_abc","project_id":"8c1e4b62-7d5a-4f39-9a11-2e3f4a5b6c7d","environment":"production","scopes":["video:write"]}}`))
	}))
	defer apiServer.Close()

	info, err := newTestClient(t, apiServer.URL, "").Whoami(context.Background())
	if err != nil {
		t.Fatalf("Whoami: %v", err)
	}

	if gotPath != "/v1/api/whoami" {
		t.Fatalf("path = %s", gotPath)
	}
	if info.KeyID != "key_abc" || info.ProjectID != "8c1e4b62-7d5a-4f39-9a11-2e3f4a5b6c7d" {
		t.Fatalf("unexpected key info: %+v", info)
	}
	if len(info.Scopes) != 1 || info.Scopes[0] != "video:write" {
		t.Fatalf("Scopes = %#v", info.Scopes)
	}
}

func TestWhoamiRequiresAProject(t *testing.T) {
	t.Parallel()

	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"api_key":{}}`))
	}))
	defer apiServer.Close()

	if _, err := newTestClient(t, apiServer.URL, "").Whoami(context.Background()); err == nil {
		t.Fatal("Whoami must reject a response without a project")
	}
}

// TestChatErrorEnvelopeKeepsTopLevelRequestID covers the chat shape, where
// request_id is a sibling of `error` rather than nested inside it.
func TestChatErrorEnvelopeKeepsTopLevelRequestID(t *testing.T) {
	t.Parallel()

	chatServer := httptest.NewServer(respondWith(t, http.StatusForbidden, "errors/chat-disabled.json"))
	defer chatServer.Close()

	_, err := newTestClient(t, "https://api.invalid", chatServer.URL).
		Chat.UpsertUser(context.Background(), ChatUserInput{ExternalUserID: "user_7"})

	apiErr, ok := AsError(err)
	if !ok {
		t.Fatalf("error is not *Error: %v", err)
	}
	if apiErr.Code != "chat_disabled" || apiErr.RequestID != "req_fixture_0004" {
		t.Fatalf("unexpected error: %+v", apiErr)
	}
	if apiErr.Retryable() {
		t.Fatal("403 must not be retryable")
	}
}

// TestChatUserPayloadOmitsEmptyOptionalFields keeps a PATCH-like upsert from
// erasing data with empty strings: the payload must not contain absent fields.
func TestChatUserPayloadOmitsEmptyOptionalFields(t *testing.T) {
	t.Parallel()

	encoded, err := json.Marshal(chatUserPayload{ExternalUserID: "user_7"})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, ok := decoded["display_name"]; ok {
		t.Fatal("display_name must be omitted when empty")
	}
	if _, ok := decoded["metadata"]; ok {
		t.Fatal("metadata must be omitted when absent")
	}
}
