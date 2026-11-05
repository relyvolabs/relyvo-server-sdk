package relyvo

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"
)

// Mode selects the billed meter for a media session.
type Mode string

const (
	// ModeVideo bills the video meter. It is the default.
	ModeVideo Mode = "video"
	// ModeAudio bills the audio meter.
	ModeAudio Mode = "audio"
)

// VideoTokenInput describes the participant to admit.
//
// The credential is join-only: the SDK cannot request room administration, and
// the project is taken from the API key, so there is no project or grant field
// here to get wrong.
type VideoTokenInput struct {
	// RoomName is the room to join. 1-128 chars: letters, digits, dot, dash,
	// colon or underscore, starting with a letter or digit.
	RoomName string

	// ParticipantIdentity is the identity the media server will see. 1-128 chars:
	// letters, digits, dot, dash, colon, at sign or underscore.
	ParticipantIdentity string

	// DisplayName is shown to other participants. Optional, at most 128 chars.
	DisplayName string

	// Mode defaults to ModeVideo when empty.
	Mode Mode
}

// VideoToken is a minted participant credential.
//
// Token is short-lived and scoped to one room. Send ServerURL and Token to the
// browser; never send the API key.
type VideoToken struct {
	ServerURL string    `json:"server_url"`
	Token     string    `json:"token"`
	RoomName  string    `json:"room_name"`
	Identity  string    `json:"participant_identity"`
	ExpiresAt time.Time `json:"expires_at"`
	Mode      Mode      `json:"mode"`
	// MeterCode names the meter that will be billed for this session.
	MeterCode string `json:"meter_code"`
}

// VideoConfig is the public connection detail for a project.
//
// It contains no secret: it is only useful together with a token minted for the
// same project, so it is safe to log.
type VideoConfig struct {
	ServerURL       string `json:"server_url"`
	TokenTTLSeconds int    `json:"token_ttl_seconds"`
}

// VideoService mints media credentials.
type VideoService struct {
	client *Client
}

// Config returns the public signalling URL and token lifetime.
func (s *VideoService) Config(ctx context.Context) (*VideoConfig, error) {
	var envelope struct {
		LiveKit VideoConfig `json:"livekit"`
	}
	if err := s.client.do(ctx, request{
		method:    http.MethodGet,
		base:      s.client.apiURL,
		path:      "/v1/api/video/config",
		out:       &envelope,
		retryable: true,
	}); err != nil {
		return nil, err
	}
	return &envelope.LiveKit, nil
}

// CreateToken mints a join-only participant token.
//
// The call is not retried automatically. A retry would mint a second credential,
// and a caller that needs one can simply ask again after inspecting the error.
func (s *VideoService) CreateToken(ctx context.Context, in VideoTokenInput) (*VideoToken, error) {
	if strings.TrimSpace(in.RoomName) == "" {
		return nil, errors.New("relyvo: VideoTokenInput.RoomName is required")
	}
	if strings.TrimSpace(in.ParticipantIdentity) == "" {
		return nil, errors.New("relyvo: VideoTokenInput.ParticipantIdentity is required")
	}

	payload := videoTokenPayload{
		RoomName:            in.RoomName,
		ParticipantIdentity: in.ParticipantIdentity,
		DisplayName:         in.DisplayName,
		Mode:                string(in.Mode),
	}

	var token VideoToken
	if err := s.client.do(ctx, request{
		method:    http.MethodPost,
		base:      s.client.apiURL,
		path:      "/v1/api/video/tokens",
		body:      payload,
		out:       &token,
		retryable: false,
	}); err != nil {
		return nil, err
	}
	return &token, nil
}

// videoTokenPayload is the wire shape, kept separate from VideoTokenInput so a
// field rename on either side is a compile error rather than a silent mismatch.
type videoTokenPayload struct {
	RoomName            string `json:"room_name"`
	ParticipantIdentity string `json:"participant_identity"`
	DisplayName         string `json:"display_name,omitempty"`
	Mode                string `json:"mode,omitempty"`
}
