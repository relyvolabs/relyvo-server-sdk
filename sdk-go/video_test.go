package relyvo

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync/atomic"
	"testing"
	"time"
)

// TestCreateTokenUsesTheDocumentedContract drives one call end to end: the
// request body must equal the shared fixture, and the decoded token must expose
// every field the browser needs.
func TestCreateTokenUsesTheDocumentedContract(t *testing.T) {
	t.Parallel()

	wantBody := decodeFixture[map[string]any](t, "requests/video-token.json")
	response := fixture(t, "responses/video-token.json")

	var (
		gotMethod string
		gotPath   string
		gotBody   map[string]any
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotBody = decodeJSONBody(t, r)

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(response)
	}))
	defer server.Close()

	client := newTestClient(t, server.URL, "")
	token, err := client.Video.CreateToken(context.Background(), VideoTokenInput{
		RoomName:            "support-42",
		ParticipantIdentity: "user_7",
		DisplayName:         "Ada",
		Mode:                ModeVideo,
	})
	if err != nil {
		t.Fatalf("CreateToken: %v", err)
	}

	if gotMethod != http.MethodPost {
		t.Fatalf("method = %s, want POST", gotMethod)
	}
	if gotPath != "/v1/api/video/tokens" {
		t.Fatalf("path = %s", gotPath)
	}
	if !reflect.DeepEqual(gotBody, wantBody) {
		t.Fatalf("body = %#v, want %#v", gotBody, wantBody)
	}

	if token.ServerURL != "wss://rtc.example.com" {
		t.Fatalf("ServerURL = %q", token.ServerURL)
	}
	if token.Token != "eyJhbGciOiJIUzI1NiJ9.fake-livekit-token.signature" {
		t.Fatalf("Token = %q", token.Token)
	}
	if token.RoomName != "support-42" || token.Identity != "user_7" {
		t.Fatalf("unexpected identity: %+v", token)
	}
	if token.Mode != ModeVideo {
		t.Fatalf("Mode = %q", token.Mode)
	}
	if token.MeterCode != "participant_seconds" {
		t.Fatalf("MeterCode = %q", token.MeterCode)
	}
	if !token.ExpiresAt.Equal(time.Date(2026, 1, 1, 0, 15, 0, 0, time.UTC)) {
		t.Fatalf("ExpiresAt = %v", token.ExpiresAt)
	}
}

func TestCreateTokenMapsErrorEnvelope(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(respondWith(t, http.StatusPaymentRequired, "errors/insufficient-credits.json"))
	defer server.Close()

	_, err := newTestClient(t, server.URL, "").Video.CreateToken(context.Background(), VideoTokenInput{
		RoomName:            "support-42",
		ParticipantIdentity: "user_7",
	})
	if err == nil {
		t.Fatal("expected an error")
	}

	apiErr, ok := AsError(err)
	if !ok {
		t.Fatalf("error is not *Error: %v", err)
	}
	if apiErr.StatusCode != http.StatusPaymentRequired {
		t.Fatalf("StatusCode = %d", apiErr.StatusCode)
	}
	if apiErr.Code != "insufficient_credits" {
		t.Fatalf("Code = %q", apiErr.Code)
	}
	if apiErr.RequestID != "req_fixture_0002" {
		t.Fatalf("RequestID = %q", apiErr.RequestID)
	}
	if apiErr.Retryable() {
		t.Fatal("402 must not be retryable")
	}
}

// TestCreateTokenIsNotRetried pins the billing-safe choice: a retry would mint a
// second credential, so a 503 must surface on the first attempt.
func TestCreateTokenIsNotRetried(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		respondWith(t, http.StatusServiceUnavailable, "errors/livekit-not-configured.json")(w, r)
	}))
	defer server.Close()

	_, err := newTestClient(t, server.URL, "").Video.CreateToken(context.Background(), VideoTokenInput{
		RoomName:            "support-42",
		ParticipantIdentity: "user_7",
	})
	if err == nil {
		t.Fatal("expected an error")
	}
	if calls.Load() != 1 {
		t.Fatalf("made %d calls, want exactly 1", calls.Load())
	}
	if !IsCode(err, "livekit_not_configured") {
		t.Fatalf("error = %v", err)
	}
}

func TestCreateTokenValidatesInputLocally(t *testing.T) {
	t.Parallel()

	client, err := New(Config{APIKey: "rv_test_key_secret", APIURL: "https://example.com"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	cases := map[string]VideoTokenInput{
		"no room":     {ParticipantIdentity: "user_7"},
		"no identity": {RoomName: "support-42"},
	}

	for name, in := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := client.Video.CreateToken(context.Background(), in); err == nil {
				t.Fatal("expected a validation error")
			}
		})
	}
}

func TestVideoConfigReadsPublicConnectionDetails(t *testing.T) {
	t.Parallel()

	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		respondWith(t, http.StatusOK, "responses/video-config.json")(w, r)
	}))
	defer server.Close()

	cfg, err := newTestClient(t, server.URL, "").Video.Config(context.Background())
	if err != nil {
		t.Fatalf("Config: %v", err)
	}

	if gotPath != "/v1/api/video/config" {
		t.Fatalf("path = %s", gotPath)
	}
	if cfg.ServerURL != "wss://rtc.example.com" || cfg.TokenTTLSeconds != 900 {
		t.Fatalf("unexpected config: %+v", cfg)
	}
}
