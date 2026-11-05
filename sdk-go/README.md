# relyvo-server-sdk (Go)

Official Go server SDK for the Relyvo platform. It exchanges an API key for the
short-lived credentials a browser needs: a LiveKit participant token and a chat
end-user token.

```go
client, err := relyvo.New(relyvo.Config{
	APIKey:  os.Getenv("RELYVO_API_KEY"),
	APIURL:  "https://relyvo.example.com/api",
	ChatURL: "https://chat-api.example.com",
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
| `Client.Chat.CreateUserToken` | `POST {chat}/v1/chat/users/{id}/tokens` |

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
