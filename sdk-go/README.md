# Relyvo Server SDK (Go)

Official Go server SDK for the Relyvo platform. It uses a project API key for
Call, Chat and Realtime server operations, and mints the short-lived credentials
a browser needs.

```go
client, err := relyvo.New(relyvo.Config{
	APIKey:  os.Getenv("RELYVO_API_KEY"),
	APIURL:  "https://relyvo.example.com/api",
	ChatURL:     "https://chat-api.example.com",
	RealtimeURL: "https://rt.example.com",
})
if err != nil {
	return err
}

video, err := client.Video.CreateToken(ctx, relyvo.VideoTokenInput{
	RoomName:            "support-42",
	ParticipantIdentity: "user_7",
})
```

## Surface

| Method | Endpoint |
| --- | --- |
| `Client.Whoami` | `GET /v1/api/whoami` |
| `Client.Video.Config` | `GET /v1/api/video/config` |
| `Client.Video.CreateToken` | `POST /v1/api/video/tokens` |
| `Client.Chat.UpsertUser` | `POST {chat}/v1/chat/users` |
| `Client.Chat.GetUser`, `Client.Chat.UpdateUser` | `GET`, `PATCH {chat}/v1/chat/users/{id}` |
| `Client.Chat.CreateUserToken`, `Client.Chat.CreateUserTokenWithOptions` | `POST {chat}/v1/chat/users/{id}/tokens` |
| `Client.Realtime.CreateClientToken` | `POST {realtime}/v1/realtime/client-tokens` |
| `Client.Realtime.Publish` | `POST {realtime}/v1/realtime/events` |
| `Client.Realtime.PublishBatch` | `POST {realtime}/v1/realtime/events/batch` |
| `Client.Realtime.ListReceipts` | `GET {realtime}/v1/realtime/events/receipts` |
| `Client.Realtime.ListUnresolved` | `GET {realtime}/v1/realtime/events/unresolved` |
| `Client.Realtime.MonitoringOverview` | `GET {realtime}/v1/realtime/monitoring/overview` |

`Client.Call` is an alias of `Client.Video`; both use the existing video endpoints.

## Acting as a Chat user

User-scoped Chat methods take an explicit `ChatActor{UserID: ...}`. The SDK
sends the project key as bearer authorization and the internal chat user ID in
`X-Relyvo-Chat-User-ID`. Set `UserID` to the canonical `User.ID` returned by a
trusted `UpsertUser` or `GetUser` call, not your external user ID: the service
interprets UUID-shaped identifiers exclusively as internal IDs, potentially
selecting a different user. The Chat service resolves the actor within the project
and applies its membership and role checks. Your backend must authorize the
caller to act for that user; never accept an arbitrary actor from a browser
without checking it. These calls use `ChatURL`, not `RealtimeURL`.

```go
actor := relyvo.ChatActor{UserID: chatUserID}
channel, err := client.Chat.CreateChannel(ctx, actor, relyvo.ChatChannelInput{
    Type: "group", Name: "Support", MemberIDs: []string{otherChatUserID},
})
if err != nil { return err }
message, err := client.Chat.SendMessage(ctx, actor, channel.Channel.ID, relyvo.ChatMessageInput{
    ClientMessageID: "your-unique-message-id", Content: "Hello",
})
_ = message
```

The SDK also supports channel and member listing/mutations, message history,
editing and deletion, read/unread state, Chat realtime credentials and
subscriptions, and attachment upload preparation/completion and download URLs.
List methods return `Items`, `HasMore`, and `NextCursor`. Mutating calls and
credential minting are not automatically retried; GETs can be retried. Upload
preparation returns a presigned URL and required headers; bytes are uploaded to
that URL separately, not through the SDK.

## Realtime

Set `RealtimeURL` to the standalone Realtime data-plane origin (for example,
`http://localhost:8091`), not the dashboard API origin. The project key stays on
your backend; authorize the user and requested channels before issuing tokens.
The key needs `realtime:subscribe` for tokens, `realtime:publish` for events,
and `realtime:read` for receipts when scopes are enforced.

```go
credentials, err := client.Realtime.CreateClientToken(ctx, relyvo.RealtimeClientTokenInput{
    UserID: "user_7", Channels: []string{"private-orders"},
})
// After checking err, return credentials (never the project key) to the browser.
_ = credentials

result, err := client.Realtime.Publish(ctx, relyvo.RealtimeEvent{
    Channel: "private-orders", Event: "order.shipped", Data: map[string]any{"id": "42"},
})
_ = result
_ = err

cursor := ""
for {
    page, err := client.Realtime.ListReceipts(ctx, relyvo.RealtimeReceiptOptions{Limit: 50, Cursor: cursor})
    if err != nil { return err }
    // Process page.Events here.
    cursor = page.NextCursor
    if cursor == "" { break }
}
```

Publish and batch are never automatically retried. Batch HTTP success can include
failed or unknown items: inspect `Results` and the counts. For a single publish
with an explicit unknown outcome, `AsError(err)` exposes `RealtimePublish` with
the event ID and nil acceptance flags. Transport failures may also leave the
outcome unknown; retrying risks duplicate events. Broker acceptance is not proof
of client delivery or processing. Receipts are best-effort metadata, can be
missing even after acceptance, and pagination is not a snapshot. Unresolved
usage candidates likewise do not establish whether an event was published or delivered.
Responses are limited to 16 MiB; larger responses return `ErrResponseTooLarge`.

Failures are `*relyvo.Error` with `StatusCode`, `Code`, `Message`, `RequestID` and
`Retryable()`. Use `relyvo.AsError` or `relyvo.IsCode` to inspect them.

Retries apply to idempotent calls only, with exponential backoff and `Retry-After`
support. Minting a token is never retried, because a retry would mint a second
credential.

## Notes

- `APIURL` is the origin where `/v1/api/...` resolves: `https://<host>/api` through a
  dashboard deployment, or `http://localhost:8080` against the API directly.
- `ChatURL` is required for chat methods; without it they return
  `relyvo.ErrNotConfigured`.
- The chat entitlement must be enabled for the project, or chat answers
  `chat_disabled`.
- The module has no runtime dependencies.

See the repository [README](../README.md) for the full contract, prerequisites and
release process.
