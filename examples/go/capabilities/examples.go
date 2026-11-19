// Package capabilities contains compile-checked examples. The caller authenticates
// its user and authorizes access before invoking any of these functions.
package capabilities

import (
	"context"
	relyvo "github.com/relyvolabs/relyvo-server-sdk/sdk-go"
)

func IssueCallToken(ctx context.Context, client *relyvo.Client, room, user string) (*relyvo.VideoToken, error) {
	return client.Call.CreateToken(ctx, relyvo.VideoTokenInput{RoomName: room, ParticipantIdentity: user})
}

func CreateChatChannel(ctx context.Context, client *relyvo.Client, internalUserID, name string) (*relyvo.ChatChannelResult, error) {
	return client.Chat.CreateChannel(ctx, relyvo.ChatActor{UserID: internalUserID}, relyvo.ChatChannelInput{Type: "group", Name: name})
}

func IssueRealtimeToken(ctx context.Context, client *relyvo.Client, user string, channels []string) (*relyvo.RealtimeClientToken, error) {
	return client.Realtime.CreateClientToken(ctx, relyvo.RealtimeClientTokenInput{UserID: user, Channels: channels})
}
