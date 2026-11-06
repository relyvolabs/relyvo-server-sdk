// Command example runs the smallest useful Relyvo server integration: resolve the
// project, mint a video token, and mint a chat token for the same person.
//
// Run it with a real key:
//
//	RELYVO_API_KEY=rv_… RELYVO_API_URL=https://relyvo.example.com/api \
//	RELYVO_CHAT_URL=https://chat-api.example.com go run .
//
// Tokens are masked by default. Pass -show-token only for a local hand-off, and
// never paste it into a shared log.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	relyvo "github.com/relyvolabs/relyvo-server-sdk/sdk-go"
)

func main() {
	room := flag.String("room", "support-42", "room to join")
	identity := flag.String("identity", "user_7", "participant identity")
	displayName := flag.String("name", "Ada", "display name")
	showToken := flag.Bool("show-token", false, "print full tokens instead of a masked prefix")
	flag.Parse()

	apiKey := strings.TrimSpace(os.Getenv("RELYVO_API_KEY"))
	if apiKey == "" {
		fail("RELYVO_API_KEY is required")
	}

	client, err := relyvo.New(relyvo.Config{
		APIKey:  apiKey,
		APIURL:  envOr("RELYVO_API_URL", "http://localhost:8080"),
		ChatURL: strings.TrimSpace(os.Getenv("RELYVO_CHAT_URL")),
	})
	if err != nil {
		fail(err.Error())
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	info, err := client.Whoami(ctx)
	if err != nil {
		failCall("whoami", err)
	}
	fmt.Printf("project   %s (%s)\n", info.ProjectID, info.Environment)

	if media, cfgErr := client.Video.Config(ctx); cfgErr != nil {
		fmt.Printf("media     unavailable: %s\n", describe(cfgErr))
	} else {
		fmt.Printf("media     %s (token ttl %ds)\n", media.ServerURL, media.TokenTTLSeconds)
	}

	video, err := client.Video.CreateToken(ctx, relyvo.VideoTokenInput{
		RoomName:            *room,
		ParticipantIdentity: *identity,
		DisplayName:         *displayName,
		Mode:                relyvo.ModeVideo,
	})
	if err != nil {
		failCall("video token", err)
	}

	printJSON("video", map[string]any{
		"room":       video.RoomName,
		"identity":   video.Identity,
		"mode":       video.Mode,
		"expires_at": video.ExpiresAt,
		"server_url": video.ServerURL,
		"token":      present(video.Token, *showToken),
		"meter_code": video.MeterCode,
	})

	if strings.TrimSpace(os.Getenv("RELYVO_CHAT_URL")) == "" {
		fmt.Println("chat      skipped: RELYVO_CHAT_URL is not set")
		return
	}

	user, err := client.Chat.UpsertUser(ctx, relyvo.ChatUserInput{
		ExternalUserID: *identity,
		DisplayName:    *displayName,
	})
	if err != nil {
		failCall("chat user", err)
	}

	fmt.Printf("chat user %s (created=%t)\n", user.User.ID, user.Created)

	token, err := client.Chat.CreateUserToken(ctx, user.User.ID)
	if err != nil {
		failCall("chat token", err)
	}

	printJSON("chat", map[string]any{
		"user_id":    user.User.ID,
		"scope":      token.Scope,
		"expires_at": token.ExpiresAt,
		"token":      present(token.Token, *showToken),
	})
}

// present masks a credential unless the operator asked to see it.
func present(token string, show bool) string {
	if show {
		return token
	}
	if len(token) <= 12 {
		return "…"
	}
	return token[:12] + "…"
}

func printJSON(label string, value any) {
	encoded, err := json.MarshalIndent(value, "  ", "  ")
	if err != nil {
		fail(err.Error())
	}
	fmt.Printf("%s\n  %s\n", label, encoded)
}

func describe(err error) string {
	if apiErr, ok := relyvo.AsError(err); ok {
		if apiErr.RequestID != "" {
			return fmt.Sprintf("%s (status %d, request %s)", apiErr.Code, apiErr.StatusCode, apiErr.RequestID)
		}
		return fmt.Sprintf("%s (status %d)", apiErr.Code, apiErr.StatusCode)
	}
	return err.Error()
}

func failCall(what string, err error) {
	fail(fmt.Sprintf("%s: %s", what, describe(err)))
}

func fail(message string) {
	fmt.Fprintf(os.Stderr, "error: %s\n", message)
	os.Exit(1)
}

func envOr(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}
