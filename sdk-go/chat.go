package relyvo

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// ChatUserInput describes a chat identity to create or update.
//
// ExternalUserID is your own user id. It is the idempotency key: calling
// UpsertUser twice with the same value returns the same chat user instead of
// creating a second one.
type ChatUserInput struct {
	ExternalUserID string
	DisplayName    string
	AvatarURL      string
	Metadata       map[string]any
}

// ChatUser is the chat service's view of a person.
type ChatUser struct {
	ID             string         `json:"id"`
	OrganizationID string         `json:"organization_id"`
	ProjectID      string         `json:"project_id"`
	ExternalUserID string         `json:"external_user_id"`
	DisplayName    string         `json:"display_name"`
	AvatarURL      string         `json:"avatar_url,omitempty"`
	Metadata       map[string]any `json:"metadata"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
}

// ChatUserResult reports the user plus whether this call created it.
type ChatUserResult struct {
	User    ChatUser
	Created bool
}

// ChatUserToken is a short-lived end-user credential.
//
// It is scoped to one project and one chat user and carries only `chat.user`.
// Send it to the browser; the browser cannot escalate it.
type ChatUserToken struct {
	Token     string    `json:"token"`
	TokenType string    `json:"token_type"`
	Scope     string    `json:"scope"`
	ExpiresAt time.Time `json:"expires_at"`
}

// ChatService manages chat identities and tokens.
//
// It speaks to the chat service directly, because that is where these resources
// live: the control plane owns admission and project policy, not chat data. Every
// call still authenticates with the Relyvo API key, which chat validates against
// the control plane.
type ChatService struct {
	client *Client
}

// UpsertUser creates or updates a chat user.
//
// Safe to retry and safe to call on every sign-in: the service deduplicates on
// (project, external_user_id), so a retry returns the original user.
func (s *ChatService) UpsertUser(ctx context.Context, in ChatUserInput) (*ChatUserResult, error) {
	if strings.TrimSpace(in.ExternalUserID) == "" {
		return nil, errors.New("relyvo: ChatUserInput.ExternalUserID is required")
	}

	if err := s.client.requireChat(); err != nil {
		return nil, err
	}

	payload := chatUserPayload{
		ExternalUserID: in.ExternalUserID,
		DisplayName:    in.DisplayName,
		AvatarURL:      in.AvatarURL,
		Metadata:       in.Metadata,
	}

	var envelope struct {
		Data struct {
			User    ChatUser `json:"user"`
			Created bool     `json:"created"`
		} `json:"data"`
	}
	if err := s.client.do(ctx, request{
		method:    http.MethodPost,
		base:      s.client.chatURL,
		path:      "/v1/chat/users",
		body:      payload,
		out:       &envelope,
		retryable: true,
	}); err != nil {
		return nil, err
	}

	return &ChatUserResult{User: envelope.Data.User, Created: envelope.Data.Created}, nil
}

// CreateUserToken mints a short-lived end-user token for a chat user.
//
// Not retried automatically: a retry would mint a second token. Tokens are cheap,
// so asking again is the right recovery.
func (s *ChatService) CreateUserToken(ctx context.Context, userID string) (*ChatUserToken, error) {
	if strings.TrimSpace(userID) == "" {
		return nil, errors.New("relyvo: userID is required")
	}

	if err := s.client.requireChat(); err != nil {
		return nil, err
	}

	var envelope struct {
		Data ChatUserToken `json:"data"`
	}
	if err := s.client.do(ctx, request{
		method:    http.MethodPost,
		base:      s.client.chatURL,
		path:      "/v1/chat/users/" + url.PathEscape(userID) + "/tokens",
		body:      map[string]any{},
		out:       &envelope,
		retryable: false,
	}); err != nil {
		return nil, err
	}

	return &envelope.Data, nil
}

// chatUserPayload is the wire shape for an upsert.
type chatUserPayload struct {
	ExternalUserID string         `json:"external_user_id"`
	DisplayName    string         `json:"display_name,omitempty"`
	AvatarURL      string         `json:"avatar_url,omitempty"`
	Metadata       map[string]any `json:"metadata,omitempty"`
}

// APIKeyInfo is the project context behind an API key.
type APIKeyInfo struct {
	ID          string   `json:"id"`
	KeyID       string   `json:"key_id"`
	ProjectID   string   `json:"project_id"`
	Environment string   `json:"environment"`
	Scopes      []string `json:"scopes"`
}

// Whoami resolves the project the key belongs to.
//
// Useful as a startup check: it fails fast on a revoked or mistyped key, before
// the first user tries to start a call.
func (c *Client) Whoami(ctx context.Context) (*APIKeyInfo, error) {
	var envelope struct {
		APIKey APIKeyInfo `json:"api_key"`
	}
	if err := c.do(ctx, request{
		method:    http.MethodGet,
		base:      c.apiURL,
		path:      "/v1/api/whoami",
		out:       &envelope,
		retryable: true,
	}); err != nil {
		return nil, err
	}
	if envelope.APIKey.ProjectID == "" {
		return nil, errors.New("relyvo: whoami returned no project")
	}
	return &envelope.APIKey, nil
}
