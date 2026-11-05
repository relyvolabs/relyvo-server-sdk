package relyvo

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// fixture loads a conformance fixture shared with the TypeScript suite.
func fixture(t *testing.T, name string) []byte {
	t.Helper()

	path := filepath.Join("..", "spec", "fixtures", name)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading fixture %s: %v", name, err)
	}
	return raw
}

// decodeFixture decodes a fixture into T.
func decodeFixture[T any](t *testing.T, name string) T {
	t.Helper()

	var value T
	if err := json.Unmarshal(fixture(t, name), &value); err != nil {
		t.Fatalf("decoding fixture %s: %v", name, err)
	}
	return value
}

// newTestClient builds a client pointed at the two test servers.
func newTestClient(t *testing.T, apiURL, chatURL string) *Client {
	t.Helper()

	client, err := New(Config{
		APIKey:     "rv_test_key_secret",
		APIURL:     apiURL,
		ChatURL:    chatURL,
		MaxRetries: 1,
		BaseDelay:  time.Millisecond,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return client
}

// respondWith writes a fixture with a status code.
func respondWith(t *testing.T, status int, fixtureName string) http.HandlerFunc {
	t.Helper()

	raw := fixture(t, fixtureName)
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		if _, err := w.Write(raw); err != nil {
			t.Errorf("writing response: %v", err)
		}
	}
}

// decodeJSONBody decodes a request body into a generic map so a test can compare
// it with a fixture without duplicating the wire struct.
func decodeJSONBody(t *testing.T, r *http.Request) map[string]any {
	t.Helper()

	var body map[string]any
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		t.Fatalf("decoding request body: %v", err)
	}
	return body
}

func TestNewValidatesConfig(t *testing.T) {
	t.Parallel()

	cases := map[string]Config{
		"missing key":     {APIURL: "https://example.com"},
		"missing api url": {APIKey: "rv_test_key_secret"},
		"bad api url":     {APIKey: "rv_test_key_secret", APIURL: "not a url"},
		"wrong scheme":    {APIKey: "rv_test_key_secret", APIURL: "ftp://example.com"},
		"bearer prefix":   {APIKey: "Bearer rv_test_key_secret", APIURL: "https://example.com"},
	}

	for name, cfg := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if _, err := New(cfg); err == nil {
				t.Fatal("New must reject an invalid configuration")
			}
		})
	}
}

func TestNewAppliesDefaults(t *testing.T) {
	t.Parallel()

	client, err := New(Config{
		APIKey: "rv_test_key_secret",
		APIURL: "https://example.com/api/",
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if client.apiURL != "https://example.com/api" {
		t.Fatalf("apiURL = %q, want the trailing slash stripped", client.apiURL)
	}
	if client.maxRetries != defaultMaxRetries {
		t.Fatalf("maxRetries = %d, want %d", client.maxRetries, defaultMaxRetries)
	}
	if client.Video == nil || client.Chat == nil {
		t.Fatal("services must be wired")
	}
	if !strings.HasPrefix(client.userAgent, "relyvo-server-sdk-go/") {
		t.Fatalf("userAgent = %q, want the SDK prefix", client.userAgent)
	}
}

// TestRequestsCarryTheKeyAsBearerAndJSON pins the headers every call needs. A
// missing Content-Type is rejected by the API's JSON guard, and a missing bearer
// header is an unauthenticated call.
func TestRequestsCarryTheKeyAsBearerAndJSON(t *testing.T) {
	t.Parallel()

	var seen http.Header
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = r.Header.Clone()
		respondWith(t, http.StatusOK, "responses/video-token.json")(w, r)
	}))
	defer server.Close()

	client := newTestClient(t, server.URL, "")
	if _, err := client.Video.CreateToken(context.Background(), VideoTokenInput{
		RoomName:            "support-42",
		ParticipantIdentity: "user_7",
	}); err != nil {
		t.Fatalf("CreateToken: %v", err)
	}

	if got := seen.Get("Authorization"); got != "Bearer rv_test_key_secret" {
		t.Fatalf("Authorization = %q", got)
	}
	if got := seen.Get("Content-Type"); got != "application/json" {
		t.Fatalf("Content-Type = %q", got)
	}
	if got := seen.Get("Accept"); got != "application/json" {
		t.Fatalf("Accept = %q", got)
	}
	if got := seen.Get("User-Agent"); !strings.HasPrefix(got, "relyvo-server-sdk-go/0.1.0") {
		t.Fatalf("User-Agent = %q", got)
	}
}

// TestAPIErrorKeepsRequestIDFromNestedEnvelope covers the Relyvo API shape, where
// request_id sits inside `error`, rather than chat's top-level field.
func TestAPIErrorKeepsRequestIDFromNestedEnvelope(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusPaymentRequired)
		_, _ = w.Write([]byte(`{"error":{"code":"insufficient_credits","message":"Top up first.","request_id":"req_nested"}}`))
	}))
	defer server.Close()

	_, err := newTestClient(t, server.URL, "").Video.CreateToken(context.Background(), VideoTokenInput{
		RoomName:            "support-42",
		ParticipantIdentity: "user_7",
	})

	apiErr, ok := AsError(err)
	if !ok {
		t.Fatalf("error is not *Error: %v", err)
	}
	if apiErr.RequestID != "req_nested" {
		t.Fatalf("RequestID = %q, want req_nested", apiErr.RequestID)
	}
	if apiErr.Code != "insufficient_credits" || apiErr.StatusCode != http.StatusPaymentRequired {
		t.Fatalf("unexpected error: %+v", apiErr)
	}
}

func TestNonJSONErrorIsReportedHonestly(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte("<html>bad gateway</html>"))
	}))
	defer server.Close()

	_, err := newTestClient(t, server.URL, "").Whoami(context.Background())

	apiErr, ok := AsError(err)
	if !ok {
		t.Fatalf("error is not *Error: %v", err)
	}
	if apiErr.Code != "unexpected_response" {
		t.Fatalf("Code = %q, want unexpected_response", apiErr.Code)
	}
	if !apiErr.Retryable() {
		t.Fatal("502 must be retryable")
	}
}

func TestTransportFailureIsRetryable(t *testing.T) {
	t.Parallel()

	client, err := New(Config{
		APIKey:     "rv_test_key_secret",
		APIURL:     "http://127.0.0.1:1",
		MaxRetries: 0,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	_, err = client.Whoami(context.Background())
	apiErr, ok := AsError(err)
	if !ok {
		t.Fatalf("error is not *Error: %v", err)
	}
	if apiErr.StatusCode != 0 {
		t.Fatalf("StatusCode = %d, want 0 for a transport failure", apiErr.StatusCode)
	}
	if !apiErr.Retryable() {
		t.Fatal("a transport failure must be retryable")
	}
	if !strings.Contains(apiErr.Error(), "transport_error") {
		t.Fatalf("Error() = %q", apiErr.Error())
	}
}

func TestContextCancellationStopsRetries(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	client, err := New(Config{
		APIKey:     "rv_test_key_secret",
		APIURL:     server.URL,
		MaxRetries: 5,
		BaseDelay:  50 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	_, err = client.Whoami(ctx)
	if err == nil {
		t.Fatal("expected an error")
	}
	if calls.Load() > 2 {
		t.Fatalf("made %d calls, want retries to stop on cancellation", calls.Load())
	}
}

func TestParseRetryAfter(t *testing.T) {
	t.Parallel()

	cases := map[string]time.Duration{
		"":                              0,
		"0":                             0,
		"-3":                            0,
		"2":                             2 * time.Second,
		"soon":                          0,
		"Wed, 21 Oct 2015 07:28:00 GMT": 0,
	}

	for value, want := range cases {
		if got := parseRetryAfter(value); got != want {
			t.Fatalf("parseRetryAfter(%q) = %v, want %v", value, got, want)
		}
	}
}

// TestEveryErrorFixtureMapsToItsCode keeps the fixture directory and the error
// mapping honest: a code with no fixture is an untested branch.
func TestEveryErrorFixtureMapsToItsCode(t *testing.T) {
	t.Parallel()

	entries, err := os.ReadDir(filepath.Join("..", "spec", "fixtures", "errors"))
	if err != nil {
		t.Fatalf("reading error fixtures: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("no error fixtures found")
	}

	for _, entry := range entries {
		name := entry.Name()
		t.Run(name, func(t *testing.T) {
			raw := fixture(t, filepath.Join("errors", name))

			var envelope errorEnvelope
			if err := json.Unmarshal(raw, &envelope); err != nil {
				t.Fatalf("decoding fixture: %v", err)
			}

			response := &http.Response{StatusCode: http.StatusBadRequest, Header: http.Header{}}
			apiErr := newAPIError(response, raw)

			if apiErr.Code != envelope.Error.Code {
				t.Fatalf("Code = %q, want %q", apiErr.Code, envelope.Error.Code)
			}
			if apiErr.Message != envelope.Error.Message {
				t.Fatalf("Message = %q, want %q", apiErr.Message, envelope.Error.Message)
			}
			if apiErr.RequestID == "" {
				t.Fatal("RequestID must be preserved")
			}
		})
	}
}

func TestIsCodeAndAsError(t *testing.T) {
	t.Parallel()

	err := error(&Error{StatusCode: http.StatusForbidden, Code: "chat_disabled"})
	if !IsCode(err, "chat_disabled") {
		t.Fatal("IsCode must match")
	}
	if IsCode(err, "not_found") {
		t.Fatal("IsCode must not match a different code")
	}

	if _, ok := AsError(errors.New("plain")); ok {
		t.Fatal("AsError must reject a plain error")
	}
}
