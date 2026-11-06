# @relyvolabs/server-sdk

Official TypeScript/Node server SDK for the Relyvo platform. It exchanges an API
key for the short-lived credentials a browser needs: a LiveKit participant token
and a chat end-user token.

> The API key is a server credential. This SDK refuses to construct in a browser
> unless `allowBrowser: true` is passed, because a bundled key is a leaked key.

```ts
import { Relyvo } from "@relyvolabs/server-sdk";

const relyvo = new Relyvo({
  apiKey: process.env.RELYVO_API_KEY!,
  apiUrl: "https://relyvo.example.com/api",
  chatUrl: "https://chat-api.example.com",
});

const video = await relyvo.video.createToken({
  roomName: "support-42",
  participantIdentity: "user_7",
  displayName: "Ada",
});

const { user } = await relyvo.chat.upsertUser({ externalUserId: "user_7", displayName: "Ada" });
const { token } = await relyvo.chat.createUserToken(user.id);
```

## Surface

| Method | Endpoint |
| --- | --- |
| `whoami()` | `GET /v1/api/whoami` |
| `video.config()` | `GET /v1/api/video/config` |
| `video.createToken(input)` | `POST /v1/api/video/tokens` |
| `chat.upsertUser(input)` | `POST {chat}/v1/chat/users` |
| `chat.createUserToken(userId)` | `POST {chat}/v1/chat/users/{id}/tokens` |

Errors are `RelyvoError` with `status`, `code`, `message`, `requestId` and
`retryable`. Configuration problems are `RelyvoConfigError`. Both are exported, as
are `isRelyvoError` and `isRelyvoConfigError`.

Retries apply to idempotent calls only (`whoami`, `video.config`, `chat.upsertUser`),
with exponential backoff and `Retry-After` support. Minting a token is never
retried, because a retry would mint a second credential.

## Notes

- `apiUrl` is the origin where `/v1/api/...` resolves: `https://<host>/api` through a
  dashboard deployment, or `http://localhost:8080` against the API directly.
- `chatUrl` is required for chat methods; without it they throw `RelyvoConfigError`.
- The chat entitlement must be enabled for the project, or chat answers
  `chat_disabled`.
- Requires Node 18+ for the global `fetch`. Pass `fetch` to use a custom transport.

See the repository [README](../README.md) for the full contract, prerequisites and
release process.
