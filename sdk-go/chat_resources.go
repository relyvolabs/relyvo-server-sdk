package relyvo

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// ChatActor explicitly selects the project-scoped chat user represented by a
// server-key request. UserID must be the canonical internal chat user ID returned
// by a trusted UpsertUser or GetUser call, not an application external ID. The
// server interprets UUID-shaped values exclusively as internal IDs, so using an
// external ID could select the wrong user. Authorize acting for this user in your
// application before making the request.
type ChatActor struct{ UserID string }

// ChatPageOptions selects a cursor page; zero Limit uses the server default.
type ChatPageOptions struct {
	Cursor string
	Limit  int
}

func (p ChatPageOptions) query() url.Values {
	q := url.Values{}
	if p.Cursor != "" {
		q.Set("cursor", p.Cursor)
	}
	if p.Limit != 0 {
		q.Set("limit", strconv.Itoa(p.Limit))
	}
	return q
}
func chatPathID(id string) string { return url.PathEscape(id) }
func chatQuery(path string, q url.Values) string {
	if len(q) > 0 {
		return path + "?" + q.Encode()
	}
	return path
}

var chatActorUUID = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

func (s *ChatService) userCall(ctx context.Context, actor ChatActor, method, path string, body, out any, retry bool) error {
	actor.UserID = strings.TrimSpace(actor.UserID)
	if !chatActorUUID.MatchString(actor.UserID) {
		return errors.New("relyvo: ChatActor.UserID must be an internal chat user UUID")
	}
	if err := s.client.requireChat(); err != nil {
		return err
	}
	var envelope struct {
		Data any `json:"data"`
	}
	envelope.Data = out
	var target any
	if out != nil {
		target = &envelope
	}
	return s.client.do(ctx, request{method: method, base: s.client.chatURL, path: path, body: body, out: target, retryable: retry, chatActor: actor.UserID})
}

// ChatChannel describes a conversation visible to the actor.
type ChatChannel struct {
	ID             string         `json:"id"`
	OrganizationID string         `json:"organization_id"`
	ProjectID      string         `json:"project_id"`
	Type           string         `json:"type"`
	Name           string         `json:"name"`
	Description    string         `json:"description"`
	CreatedBy      string         `json:"created_by"`
	Metadata       map[string]any `json:"metadata"`
	LastMessageID  *string        `json:"last_message_id"`
	LastSeq        int64          `json:"last_seq"`
	MaxMembers     int            `json:"max_members"`
	ExternalID     string         `json:"external_id"`
	SourceProduct  string         `json:"source_product"`
	SendingEnabled bool           `json:"sending_enabled"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	DeletedAt      *time.Time     `json:"deleted_at"`
	MembershipRole string         `json:"membership_role"`
}
type ChatChannelInput struct {
	Type        string         `json:"type"`
	Name        string         `json:"name,omitempty"`
	Description string         `json:"description,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
	MemberIDs   []string       `json:"member_ids,omitempty"`
	OtherUserID *string        `json:"other_user_id,omitempty"`
}
type ChatChannelUpdate struct {
	Name        *string        `json:"name,omitempty"`
	Description *string        `json:"description,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}
type ChatChannelResult struct {
	Channel ChatChannel `json:"channel"`
	Created bool        `json:"created"`
}
type ChatChannelPage struct {
	Items      []ChatChannel `json:"items"`
	NextCursor string        `json:"next_cursor"`
	HasMore    bool          `json:"has_more"`
}

func (s *ChatService) CreateChannel(ctx context.Context, a ChatActor, in ChatChannelInput) (*ChatChannelResult, error) {
	var v ChatChannelResult
	err := s.userCall(ctx, a, http.MethodPost, "/v1/chat/channels", in, &v, false)
	if err != nil {
		return nil, err
	}
	return &v, nil
}
func (s *ChatService) ListChannels(ctx context.Context, a ChatActor, p ChatPageOptions) (*ChatChannelPage, error) {
	var v ChatChannelPage
	err := s.userCall(ctx, a, http.MethodGet, chatQuery("/v1/chat/channels", p.query()), nil, &v, true)
	if err != nil {
		return nil, err
	}
	return &v, nil
}
func (s *ChatService) GetChannel(ctx context.Context, a ChatActor, id string) (*ChatChannel, error) {
	var v ChatChannel
	err := s.userCall(ctx, a, http.MethodGet, "/v1/chat/channels/"+chatPathID(id), nil, &v, true)
	if err != nil {
		return nil, err
	}
	return &v, nil
}
func (s *ChatService) UpdateChannel(ctx context.Context, a ChatActor, id string, in ChatChannelUpdate) (*ChatChannel, error) {
	var v ChatChannel
	err := s.userCall(ctx, a, http.MethodPatch, "/v1/chat/channels/"+chatPathID(id), in, &v, false)
	if err != nil {
		return nil, err
	}
	return &v, nil
}
func (s *ChatService) DeleteChannel(ctx context.Context, a ChatActor, id string) error {
	return s.userCall(ctx, a, http.MethodDelete, "/v1/chat/channels/"+chatPathID(id), nil, nil, false)
}

// ChatMember describes a channel membership.
type ChatMember struct {
	ID             string     `json:"id"`
	OrganizationID string     `json:"organization_id"`
	ProjectID      string     `json:"project_id"`
	ChannelID      string     `json:"channel_id"`
	UserID         string     `json:"user_id"`
	Role           string     `json:"role"`
	JoinedAt       time.Time  `json:"joined_at"`
	LeftAt         *time.Time `json:"left_at"`
	MutedUntil     *time.Time `json:"muted_until"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	User           *ChatUser  `json:"user"`
}
type ChatMemberInput struct {
	UserID string `json:"user_id"`
	Role   string `json:"role,omitempty"`
}
type ChatMemberResult struct {
	Member ChatMember `json:"member"`
	Added  bool       `json:"added"`
}
type ChatMemberPage struct {
	Items      []ChatMember `json:"items"`
	NextCursor string       `json:"next_cursor"`
	HasMore    bool         `json:"has_more"`
}

func (s *ChatService) AddMember(ctx context.Context, a ChatActor, channelID string, in ChatMemberInput) (*ChatMemberResult, error) {
	var v ChatMemberResult
	err := s.userCall(ctx, a, http.MethodPost, "/v1/chat/channels/"+chatPathID(channelID)+"/members", in, &v, false)
	if err != nil {
		return nil, err
	}
	return &v, nil
}
func (s *ChatService) ListMembers(ctx context.Context, a ChatActor, channelID string, p ChatPageOptions) (*ChatMemberPage, error) {
	var v ChatMemberPage
	err := s.userCall(ctx, a, http.MethodGet, chatQuery("/v1/chat/channels/"+chatPathID(channelID)+"/members", p.query()), nil, &v, true)
	if err != nil {
		return nil, err
	}
	return &v, nil
}
func (s *ChatService) RemoveMember(ctx context.Context, a ChatActor, channelID, userID string) error {
	return s.userCall(ctx, a, http.MethodDelete, "/v1/chat/channels/"+chatPathID(channelID)+"/members/"+chatPathID(userID), nil, nil, false)
}

// ChatMessage is a persisted message and channel sequence.
type ChatMessage struct {
	ID               string         `json:"id"`
	OrganizationID   string         `json:"organization_id"`
	ProjectID        string         `json:"project_id"`
	ChannelID        string         `json:"channel_id"`
	SenderID         string         `json:"sender_id"`
	ChannelSeq       int64          `json:"channel_seq"`
	ClientMessageID  string         `json:"client_message_id"`
	Type             string         `json:"type"`
	Content          string         `json:"content"`
	Metadata         map[string]any `json:"metadata"`
	ReplyToMessageID *string        `json:"reply_to_message_id"`
	CreatedAt        time.Time      `json:"created_at"`
	EditedAt         *time.Time     `json:"edited_at"`
	DeletedAt        *time.Time     `json:"deleted_at"`
	AttachmentIDs    []string       `json:"attachment_ids"`
}
type ChatMessageInput struct {
	ClientMessageID  string         `json:"client_message_id,omitempty"`
	Type             string         `json:"type,omitempty"`
	Content          string         `json:"content"`
	Metadata         map[string]any `json:"metadata,omitempty"`
	ReplyToMessageID *string        `json:"reply_to_message_id,omitempty"`
	AttachmentIDs    []string       `json:"attachment_ids,omitempty"`
}
type ChatMessageUpdate struct {
	Content  string         `json:"content"`
	Metadata map[string]any `json:"metadata,omitempty"`
}
type ChatMessageResult struct {
	Message ChatMessage `json:"message"`
	Created bool        `json:"created"`
}
type ChatMessagePage struct {
	Items      []ChatMessage `json:"items"`
	NextCursor string        `json:"next_cursor"`
	HasMore    bool          `json:"has_more"`
}

// ChatMessagePageOptions selects a cursor and optional forward direction.
type ChatMessagePageOptions struct {
	ChatPageOptions
	After bool
}

// SendMessage is not retried automatically, even when ClientMessageID is set.
func (s *ChatService) SendMessage(ctx context.Context, a ChatActor, channelID string, in ChatMessageInput) (*ChatMessageResult, error) {
	var v ChatMessageResult
	err := s.userCall(ctx, a, http.MethodPost, "/v1/chat/channels/"+chatPathID(channelID)+"/messages", in, &v, false)
	if err != nil {
		return nil, err
	}
	return &v, nil
}
func (s *ChatService) ListMessages(ctx context.Context, a ChatActor, channelID string, p ChatMessagePageOptions) (*ChatMessagePage, error) {
	q := p.ChatPageOptions.query()
	if p.After {
		q.Set("direction", "after")
	}
	var v ChatMessagePage
	err := s.userCall(ctx, a, http.MethodGet, chatQuery("/v1/chat/channels/"+chatPathID(channelID)+"/messages", q), nil, &v, true)
	if err != nil {
		return nil, err
	}
	return &v, nil
}
func (s *ChatService) GetMessage(ctx context.Context, a ChatActor, id string) (*ChatMessage, error) {
	var v ChatMessage
	err := s.userCall(ctx, a, http.MethodGet, "/v1/chat/messages/"+chatPathID(id), nil, &v, true)
	if err != nil {
		return nil, err
	}
	return &v, nil
}
func (s *ChatService) EditMessage(ctx context.Context, a ChatActor, id string, in ChatMessageUpdate) (*ChatMessage, error) {
	var v ChatMessage
	err := s.userCall(ctx, a, http.MethodPatch, "/v1/chat/messages/"+chatPathID(id), in, &v, false)
	if err != nil {
		return nil, err
	}
	return &v, nil
}
func (s *ChatService) DeleteMessage(ctx context.Context, a ChatActor, id string) error {
	return s.userCall(ctx, a, http.MethodDelete, "/v1/chat/messages/"+chatPathID(id), nil, nil, false)
}

type ChatReadState struct {
	OrganizationID    string    `json:"organization_id"`
	ProjectID         string    `json:"project_id"`
	ChannelID         string    `json:"channel_id"`
	UserID            string    `json:"user_id"`
	LastReadMessageID *string   `json:"last_read_message_id"`
	LastReadSeq       int64     `json:"last_read_seq"`
	LastReadAt        time.Time `json:"last_read_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}
type ChatUnreadSummary struct {
	ChannelID   string `json:"channel_id"`
	LastSeq     int64  `json:"last_seq"`
	LastReadSeq int64  `json:"last_read_seq"`
	UnreadCount int64  `json:"unread_count"`
	HasUnread   bool   `json:"has_unread"`
}

func (s *ChatService) MarkRead(ctx context.Context, a ChatActor, channelID, messageID string) (*ChatReadState, error) {
	var v ChatReadState
	body := struct {
		MessageID string `json:"message_id"`
	}{messageID}
	err := s.userCall(ctx, a, http.MethodPost, "/v1/chat/channels/"+chatPathID(channelID)+"/read", body, &v, false)
	if err != nil {
		return nil, err
	}
	return &v, nil
}
func (s *ChatService) Unread(ctx context.Context, a ChatActor, channelID string) (*ChatUnreadSummary, error) {
	var v ChatUnreadSummary
	err := s.userCall(ctx, a, http.MethodGet, "/v1/chat/channels/"+chatPathID(channelID)+"/unread", nil, &v, true)
	if err != nil {
		return nil, err
	}
	return &v, nil
}

// ChatPresenceEntry describes one subscriber observed by the gateway.
type ChatPresenceEntry struct {
	ClientID string          `json:"client_id"`
	UserID   string          `json:"user_id"`
	Info     json.RawMessage `json:"info,omitempty"`
}

// ChatPresence is the gateway presence response for a channel.
type ChatPresence struct {
	ChannelID string              `json:"channel_id"`
	Presence  []ChatPresenceEntry `json:"presence"`
}

// Typing publishes a transient typing indication without automatic retry.
func (s *ChatService) Typing(ctx context.Context, a ChatActor, channelID string) error {
	return s.userCall(ctx, a, http.MethodPost, "/v1/chat/channels/"+chatPathID(channelID)+"/typing", nil, nil, false)
}

// Presence reports the gateway's current subscribers for an authorized channel.
func (s *ChatService) Presence(ctx context.Context, a ChatActor, channelID string) (*ChatPresence, error) {
	var v ChatPresence
	if err := s.userCall(ctx, a, http.MethodGet, "/v1/chat/channels/"+chatPathID(channelID)+"/presence", nil, &v, true); err != nil {
		return nil, err
	}
	return &v, nil
}

type ChatRealtimeCredentials struct {
	Token         string    `json:"token"`
	WSURL         string    `json:"ws_url"`
	ExpiresAt     time.Time `json:"expires_at"`
	ChannelPrefix string    `json:"channel_prefix"`
	TTLSeconds    int       `json:"ttl_seconds"`
}
type ChatSubscriptionCredential struct {
	Channel   string    `json:"channel"`
	Kind      string    `json:"kind"`
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
}

func (s *ChatService) CreateRealtimeToken(ctx context.Context, a ChatActor) (*ChatRealtimeCredentials, error) {
	var v ChatRealtimeCredentials
	err := s.userCall(ctx, a, http.MethodPost, "/v1/chat/realtime/token", nil, &v, false)
	if err != nil {
		return nil, err
	}
	return &v, nil
}
func (s *ChatService) CreateSubscriptions(ctx context.Context, a ChatActor, channelIDs []string) ([]ChatSubscriptionCredential, error) {
	var v struct {
		Subscriptions []ChatSubscriptionCredential `json:"subscriptions"`
	}
	body := struct {
		ChannelIDs []string `json:"channel_ids"`
	}{channelIDs}
	err := s.userCall(ctx, a, http.MethodPost, "/v1/chat/realtime/subscriptions", body, &v, false)
	if err != nil {
		return nil, err
	}
	return v.Subscriptions, nil
}

type ChatAttachment struct {
	ID              string     `json:"id"`
	OrganizationID  string     `json:"organization_id"`
	ProjectID       string     `json:"project_id"`
	UploaderID      string     `json:"uploader_id"`
	MessageID       *string    `json:"message_id"`
	StorageProvider string     `json:"storage_provider"`
	FileName        string     `json:"file_name"`
	MIMEType        string     `json:"mime_type"`
	SizeBytes       int64      `json:"size_bytes"`
	Status          string     `json:"status"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
	ExpiresAt       *time.Time `json:"expires_at"`
}
type ChatUploadInput struct {
	FileName  string `json:"file_name"`
	MIMEType  string `json:"mime_type"`
	SizeBytes int64  `json:"size_bytes"`
}
type ChatPreparedUpload struct {
	Attachment ChatAttachment    `json:"attachment"`
	UploadURL  string            `json:"upload_url"`
	Headers    map[string]string `json:"headers"`
	ExpiresAt  time.Time         `json:"expires_at"`
}
type ChatAttachmentDownload struct {
	Attachment  ChatAttachment `json:"attachment"`
	DownloadURL string         `json:"download_url"`
}

func (s *ChatService) PrepareUpload(ctx context.Context, a ChatActor, in ChatUploadInput) (*ChatPreparedUpload, error) {
	var v ChatPreparedUpload
	err := s.userCall(ctx, a, http.MethodPost, "/v1/chat/attachments/upload", in, &v, false)
	if err != nil {
		return nil, err
	}
	return &v, nil
}
func (s *ChatService) CompleteUpload(ctx context.Context, a ChatActor, id string) (*ChatAttachment, error) {
	var v ChatAttachment
	err := s.userCall(ctx, a, http.MethodPost, "/v1/chat/attachments/"+chatPathID(id)+"/complete", nil, &v, false)
	if err != nil {
		return nil, err
	}
	return &v, nil
}
func (s *ChatService) AttachmentDownload(ctx context.Context, a ChatActor, id string) (*ChatAttachmentDownload, error) {
	var v ChatAttachmentDownload
	err := s.userCall(ctx, a, http.MethodGet, "/v1/chat/attachments/"+chatPathID(id), nil, &v, true)
	if err != nil {
		return nil, err
	}
	return &v, nil
}
