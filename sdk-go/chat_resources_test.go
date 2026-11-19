package relyvo

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestChatUserResources(t *testing.T) {
	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Header.Get("Authorization") != "Bearer rv_test_key_secret" || r.Header.Get("X-Relyvo-Chat-User-ID") != "123e4567-e89b-12d3-a456-426614174000" {
			t.Error("incorrect authorization")
		}
		switch r.URL.Path {
		case "/v1/chat/channels":
			if r.Method == "POST" {
				b := decodeJSONBody(t, r)
				if b["name"] != "Team" {
					t.Error(b)
				}
				_, _ = w.Write([]byte(`{"data":{"channel":{"id":"c","name":"Team"},"created":true}}`))
				return
			}
			if r.URL.Query().Get("cursor") != "a+b" || r.URL.Query().Get("limit") != "10" {
				t.Error("bad pagination")
			}
			_, _ = w.Write([]byte(`{"data":{"items":[{"id":"c"}],"next_cursor":"next","has_more":true}}`))
		case "/v1/chat/channels/c/members":
			b := decodeJSONBody(t, r)
			if b["user_id"] != "u" {
				t.Error(b)
			}
			_, _ = w.Write([]byte(`{"data":{"member":{"user_id":"u"},"added":true}}`))
		case "/v1/chat/channels/c/messages":
			if r.Method == "GET" {
				if r.URL.Query().Get("direction") != "after" {
					t.Error("missing direction")
				}
				_, _ = w.Write([]byte(`{"data":{"items":[{"id":"m","channel_seq":4}],"has_more":false}}`))
				return
			}
			b := decodeJSONBody(t, r)
			if b["client_message_id"] != "dedupe" {
				t.Error(b)
			}
			_, _ = w.Write([]byte(`{"data":{"message":{"id":"m","channel_seq":4},"created":true}}`))
		case "/v1/chat/channels/c/read":
			b := decodeJSONBody(t, r)
			if b["message_id"] != "m" {
				t.Error(b)
			}
			_, _ = w.Write([]byte(`{"data":{"last_read_seq":4}}`))
		case "/v1/chat/realtime/token":
			_, _ = w.Write([]byte(`{"data":{"token":"t","ws_url":"wss://example","ttl_seconds":30}}`))
		case "/v1/chat/realtime/subscriptions":
			_, _ = w.Write([]byte(`{"data":{"subscriptions":[{"channel":"channel","kind":"messages","token":"sub"}]}}`))
		case "/v1/chat/attachments/upload":
			_, _ = w.Write([]byte(`{"data":{"attachment":{"id":"a"},"upload_url":"https://storage","headers":{"X-Test":"v"},"expires_at":"2026-01-01T00:00:00Z"}}`))
		case "/v1/chat/attachments/a/complete":
			_, _ = w.Write([]byte(`{"data":{"id":"a","status":"ready"}}`))
		case "/v1/chat/attachments/a":
			_, _ = w.Write([]byte(`{"data":{"attachment":{"id":"a"},"download_url":"https://storage"}}`))
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()
	c := newTestClient(t, "http://example.test", server.URL)
	a := ChatActor{UserID: "123e4567-e89b-12d3-a456-426614174000"}
	ctx := context.Background()
	channel, err := c.Chat.CreateChannel(ctx, a, ChatChannelInput{Name: "Team"})
	if err != nil || !channel.Created || channel.Channel.ID != "c" {
		t.Fatalf("%+v %v", channel, err)
	}
	page, err := c.Chat.ListChannels(ctx, a, ChatPageOptions{Cursor: "a+b", Limit: 10})
	if err != nil || !page.HasMore || page.NextCursor != "next" {
		t.Fatalf("%+v %v", page, err)
	}
	member, err := c.Chat.AddMember(ctx, a, "c", ChatMemberInput{UserID: "u"})
	if err != nil || !member.Added || member.Member.UserID != "u" {
		t.Fatalf("%+v %v", member, err)
	}
	msg, err := c.Chat.SendMessage(ctx, a, "c", ChatMessageInput{ClientMessageID: "dedupe"})
	if err != nil || !msg.Created || msg.Message.ChannelSeq != 4 {
		t.Fatalf("%+v %v", msg, err)
	}
	messages, err := c.Chat.ListMessages(ctx, a, "c", ChatMessagePageOptions{After: true})
	if err != nil || len(messages.Items) != 1 || messages.Items[0].ID != "m" {
		t.Fatalf("%+v %v", messages, err)
	}
	read, err := c.Chat.MarkRead(ctx, a, "c", "m")
	if err != nil || read.LastReadSeq != 4 {
		t.Fatalf("%+v %v", read, err)
	}
	token, err := c.Chat.CreateRealtimeToken(ctx, a)
	if err != nil || token.Token != "t" {
		t.Fatalf("%+v %v", token, err)
	}
	subs, err := c.Chat.CreateSubscriptions(ctx, a, []string{"c"})
	if err != nil || len(subs) != 1 || subs[0].Token != "sub" {
		t.Fatalf("%+v %v", subs, err)
	}
	upload, err := c.Chat.PrepareUpload(ctx, a, ChatUploadInput{FileName: "x"})
	if err != nil || upload.Headers["X-Test"] != "v" {
		t.Fatalf("%+v %v", upload, err)
	}
	attachment, err := c.Chat.CompleteUpload(ctx, a, "a")
	if err != nil || attachment.Status != "ready" {
		t.Fatalf("%+v %v", attachment, err)
	}
	download, err := c.Chat.AttachmentDownload(ctx, a, "a")
	if err != nil || download.DownloadURL != "https://storage" {
		t.Fatalf("%+v %v", download, err)
	}
	if calls != 11 {
		t.Fatal(calls)
	}
}

func TestChatActorUUIDValidation(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if got := r.Header.Get("X-Relyvo-Chat-User-ID"); got != "123E4567-E89B-12D3-A456-426614174000" {
			t.Errorf("actor header = %q", got)
		}
		_, _ = w.Write([]byte(`{"data":{"id":"c"}}`))
	}))
	defer server.Close()
	c := newTestClient(t, "http://example.test", server.URL)
	for _, id := range []string{"", " ", "external-user", "123e4567e89b12d3a456426614174000", "123e4567-e89b-12d3-a456-42661417400g", "{123e4567-e89b-12d3-a456-426614174000}", "123e4567-e89b-12d3-a456-426614174000\nextra"} {
		t.Run(id, func(t *testing.T) {
			_, err := c.Chat.GetChannel(context.Background(), ChatActor{UserID: id}, "c")
			if err == nil || calls.Load() != 0 {
				t.Fatalf("invalid actor error = %v, calls = %d", err, calls.Load())
			}
		})
	}
	_, err := c.Chat.GetChannel(context.Background(), ChatActor{UserID: " 123E4567-E89B-12D3-A456-426614174000 "}, "c")
	if err != nil || calls.Load() != 1 {
		t.Fatalf("valid actor error = %v, calls = %d", err, calls.Load())
	}
}

func TestChatUserAuthErrorsAndRetries(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Header.Get("X-Relyvo-Chat-User-ID") != "123e4567-e89b-12d3-a456-426614174000" {
			t.Error("missing actor")
		}
		w.WriteHeader(503)
		_, _ = w.Write([]byte(`{"error":{"code":"unavailable","message":"down"},"request_id":"req"}`))
	}))
	defer server.Close()
	c := newTestClient(t, "http://example.test", server.URL)
	a := ChatActor{UserID: "123e4567-e89b-12d3-a456-426614174000"}
	_, err := c.Chat.CreateChannel(context.Background(), a, ChatChannelInput{})
	api, ok := AsError(err)
	if !ok || api.Code != "unavailable" || api.RequestID != "req" || calls.Load() != 1 {
		t.Fatalf("%v calls=%d", err, calls.Load())
	}
	_, _ = c.Chat.GetChannel(context.Background(), a, "id")
	if calls.Load() != 3 {
		t.Fatalf("GET attempts %d", calls.Load())
	}
	_, err = c.Chat.GetChannel(context.Background(), ChatActor{}, "id")
	if err == nil || calls.Load() != 3 {
		t.Fatalf("missing actor %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = c.Chat.GetChannel(ctx, a, "id")
	if !errors.Is(err, context.Canceled) || calls.Load() != 3 {
		t.Fatal(err)
	}
}
