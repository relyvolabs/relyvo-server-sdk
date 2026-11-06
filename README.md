# Relyvo Server SDK

Official server-side SDKs for the Relyvo platform, in Go and TypeScript. They
exist for one job: a trusted backend exchanges its **Relyvo API key** for the
short-lived credentials a browser needs — a video call participant token and a
chat end-user token — without a dashboard session and without ever shipping the
API key to the client.

> **The API key is a server credential.** It grants programmatic access to exactly
> one project. It must never appear in a browser bundle, a mobile app, a public
> repository or a log line. What a client receives is a short-lived token.

```
your backend ──(rv_ API key)──► Relyvo API    ──► video participant token
             ──(rv_ API key)──► Relyvo Chat   ──► chat end-user token
your browser ──(short-lived token)──► LiveKit / Centrifugo
```

## Install

**Go**

```bash
go get github.com/relyvolabs/relyvo-server-sdk/sdk-go@v0.1.0
```

The module lives in a subdirectory, so the version tag is `sdk-go/vX.Y.Z`. A bare
`vX.Y.Z` tag will not resolve.

**TypeScript / Node 18+**

```bash
npm install @relyvolabs/server-sdk
# or: pnpm add @relyvolabs/server-sdk
```

Both require the repository to be public. For a private fork, set
`GOPRIVATE=github.com/relyvolabs/relyvo-server-sdk` for Go (with a configured git
credential), and install the TypeScript package from a tarball or a private
registry instead of npmjs.

## Packages

| Path | Language | Import | Status |
| --- | --- | --- | --- |
| [`sdk-go/`](sdk-go) | Go 1.22+ | `github.com/relyvolabs/relyvo-server-sdk/sdk-go` | 0.1.0 |
| [`sdk-ts/`](sdk-ts) | TypeScript, Node 18+ | `@relyvolabs/server-sdk` | 0.1.0 |
| [`spec/`](spec) | — | Vendored contracts + conformance fixtures | — |
| [`examples/`](examples) | Go, TypeScript | Minimal servers | — |

Both SDKs implement the same surface and are tested against the same fixtures, so
they cannot drift apart silently.

## What is covered

| Capability | Go | TypeScript | Endpoint |
| --- | --- | --- | --- |
| Resolve the key's project | ✅ | ✅ | `GET /v1/api/whoami` |
| Public media connection details | ✅ | ✅ | `GET /v1/api/video/config` |
| Mint a video/voice participant token | ✅ | ✅ | `POST /v1/api/video/tokens` |
| Create or update a chat user | ✅ | ✅ | `POST {chat}/v1/chat/users` |
| Mint a chat end-user token | ✅ | ✅ | `POST {chat}/v1/chat/users/{id}/tokens` |

Not covered yet: meetings, channels, messages, read state and attachments. The
building blocks are typed and the transport is generic, so adding them is a
matter of adding methods — not of reworking the client.

## Quick start (Go)

```go
client, err := relyvo.New(relyvo.Config{
	APIKey:  os.Getenv("RELYVO_API_KEY"),
	APIURL:  "https://relyvo.example.com/api", // where /v1/api/... resolves
	ChatURL: "https://chat-api.example.com",   // optional, needed for chat
})
if err != nil {
	log.Fatal(err)
}

video, err := client.Video.CreateToken(ctx, relyvo.VideoTokenInput{
	RoomName:            "support-42",
	ParticipantIdentity: "user_7",
	DisplayName:         "Ada",
	Mode:                relyvo.ModeVideo,
})
// Hand video.ServerURL and video.Token to the browser. Never the API key.
```

## Quick start (TypeScript)

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
  mode: "video",
});

const { user } = await relyvo.chat.upsertUser({ externalUserId: "user_7", displayName: "Ada" });
const { token } = await relyvo.chat.createUserToken(user.id);
```

The TypeScript SDK refuses to construct in a browser (`typeof window !==
"undefined"`) unless `allowBrowser: true` is passed explicitly. That guard exists
because the constructor receives the API key.

## Prerequisites on the Relyvo side

1. **An API key** created in the dashboard for the project, with the environment
   you intend to use. This is `rv_<keyID>_<secret>`.
2. **Video**: the deployment must have LiveKit configured. Without it the SDK
   returns a clear `livekit_not_configured` error instead of a token that cannot
   connect. The organization needs enough credit for the initial reservation, and
   the number of live sessions is capped per organization.
3. **Chat**: the project must have the chat entitlement enabled (Playground →
   chat, or `PUT /v1/projects/{projectID}/entitlements`). An absent entitlement row
   means "nothing enabled", so chat answers `chat_disabled` until it is switched on.
   The chat REST API must be reachable from your backend at `ChatURL`.
4. **Base URLs**: `APIURL` is the origin where `/v1/api/...` resolves. Through the
   dashboard that is `https://<dashboard-host>/api`; against the API directly it
   is `http://localhost:8080`. The SDK appends `/v1/api/...` and never rewrites
   your prefix.

## Error model

Both SDKs map Relyvo's error envelope onto one typed error:

```json
{ "error": { "code": "insufficient_credits", "message": "…" }, "request_id": "…" }
```

| Field | Meaning |
| --- | --- |
| `status` | HTTP status, `0` when the request never completed |
| `code` | Stable machine code (`insufficient_credits`, `chat_disabled`, …) |
| `message` | Human-readable text from the service |
| `requestId` / `RequestID` | Correlates with server logs and support |
| `retryable` / `Retryable()` | True for `429`, `502`, `503`, `504` and transport failures |

Retry policy: only idempotent calls are retried, with exponential backoff, jitter
and `Retry-After` support. Minting a video token and a chat token are **not**
retried automatically, because a retry would mint a second credential; a chat user
upsert is idempotent and is retried.

## Repository layout

```
spec/            vendored OpenAPI + fixtures shared by both test suites
sdk-go/          Go module (zero runtime dependencies)
sdk-ts/          npm package (zero runtime dependencies)
examples/        one minimal backend per language
.github/         CI and release workflows
```

## Versioning and releases

Each language is released independently from this repository:

| SDK | Tag | Distribution |
| --- | --- | --- |
| Go | `sdk-go/v0.1.0` | Go module proxy reads the tag; import path includes the directory |
| TypeScript | `sdk-ts/v0.1.0` | `npm publish` from `sdk-ts/` |

Go requires the directory prefix in the tag because the module lives in a
subdirectory of the repository. A tag named only `v0.1.0` does not work for
`sdk-go`.

## Development

```bash
# TypeScript (workspace root installs both packages)
pnpm install
pnpm -C sdk-ts run typecheck && pnpm -C sdk-ts test && pnpm -C sdk-ts run build

# Go
cd sdk-go && go vet ./... && go test -race ./...

# Examples
cd examples/go && go build -o /tmp/relyvo-example .
pnpm -C examples/ts run typecheck
```

No live service is required for the test suites: they run against fixtures and
local HTTP servers that speak the documented contract. The examples do need a real
key (or any HTTP double) because they are the hand-off demonstration.

## Licence

Proprietary. Not for redistribution. `UNLICENSED` in package metadata.
