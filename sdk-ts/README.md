# @relyvolabs/server-sdk

Official TypeScript/Node server SDK for the Relyvo platform. It uses a project
API key for Call, Chat and Realtime server operations, and mints the short-lived
credentials a browser needs.

> The API key is a server credential. This SDK refuses to construct in a browser
> unless `allowBrowser: true` is passed, because a bundled key is a leaked key.

```ts
import { Relyvo } from "@relyvolabs/server-sdk";

const relyvo = new Relyvo({
  apiKey: process.env.RELYVO_API_KEY!,
  apiUrl: "https://relyvo.example.com/api",
  chatUrl: "https://chat-api.example.com",
  realtimeUrl: "https://rt.example.com",
});

const video = await relyvo.video.createToken({
  roomName: "support-42",
  participantIdentity: "user_7",
  displayName: "Ada",
});

const { user } = await relyvo.chat.upsertUser({ externalUserId: "user_7", displayName: "Ada" });
const { token } = await relyvo.chat.createUserToken(user.id);
const credentials = await relyvo.realtime.createClientToken({ userId: "user_7", channels: ["news"] });
const result = await relyvo.realtime.publish({ channel: "news", event: "update", data: { text: "hello" } });
const page = await relyvo.realtime.listReceipts({ limit: 50 });
const next = page.nextCursor ? await relyvo.realtime.listReceipts({ cursor: page.nextCursor }) : undefined;
```

## Surface

| Method | Endpoint |
| --- | --- |
| `whoami()` | `GET /v1/api/whoami` |
| `call` / `video` | Alias; both expose `config()` and `createToken(input)` |
| `video.config()` | `GET /v1/api/video/config` |
| `video.createToken(input)` | `POST /v1/api/video/tokens` |
| `chat.upsertUser(input)` | `POST {chat}/v1/chat/users` |
| `chat.getUser(userId)` / `chat.updateUser(userId, input)` | `GET` / `PATCH {chat}/v1/chat/users/{id}` |
| `chat.createUserToken(userId)` | `POST {chat}/v1/chat/users/{id}/tokens` |
| `realtime.createClientToken(input)` | `POST {realtime}/v1/realtime/client-tokens` |
| `realtime.publish(input)` | `POST {realtime}/v1/realtime/events` |
| `realtime.publishBatch(events)` | `POST {realtime}/v1/realtime/events/batch` |
| `realtime.listReceipts({ limit, cursor })` | `GET {realtime}/v1/realtime/events/receipts` |
| `realtime.listUnresolved({ limit, cursor })` | `GET {realtime}/v1/realtime/events/unresolved` |
| `realtime.monitoringOverview(window)` | `GET {realtime}/v1/realtime/monitoring/overview` |

## User-scoped Chat

All user-scoped Chat methods take an explicit actor as their first argument: the
canonical internal chat user UUID returned by `upsertUser`, which your server has
authenticated and is authorized to act for. UUID-shaped external IDs are ambiguous
on the backend, so never pass an external ID as the actor. The project key stays in the bearer header; the actor
is sent in `X-Relyvo-Chat-User-ID`. The server still enforces channel membership
and roles. Never trust a browser-supplied actor without your own authorization.

```ts
const { user: actor } = await relyvo.chat.upsertUser({ externalUserId: "user_7" });
const created = await relyvo.chat.createChannel(actor.id, { type: "group", name: "Support" });
const channelId = created.channel.id;
await relyvo.chat.sendMessage(actor.id, channelId, { content: "Hello", clientMessageId: "your-unique-id" });
const history = await relyvo.chat.listMessages(actor.id, channelId, { limit: 20 });
```

User-scoped methods also cover channel update/delete, members, message
edit/delete, read/unread, typing/presence, realtime credentials, and attachment
prepare/complete/download. Their typed response models mirror Chat JSON with
snake_case keys and ISO date strings; the existing identity/token responses are
unchanged. Lists expose `items`, `has_more`, and optional `next_cursor`.
Writes, including keyed sends, are never retried automatically; GETs may retry.
The SDK returns presigned URLs only; it does not transfer attachment bytes.

For the web Realtime SDK, map credentials in your authenticated server endpoint:

```ts
import { toRealtimeWebCredentials } from "@relyvolabs/server-sdk";
const credentials = await relyvo.realtime.createClientToken({ userId: "user_7", channels: ["news"] });
return Response.json(toRealtimeWebCredentials(credentials)); // never send the project API key
```

`call` is an alias of `video`. Realtime uses the project key and scopes: subscribe
for token issuance, publish for events, and read for receipts. Receipts are
best-effort metadata, not proof of delivery or failure. Batch results can include
failed and unknown outcomes. For an ambiguous single publish the SDK throws a
`RelyvoError` with code `publish_outcome_unknown` and the raw result in `details`;
retrying may duplicate an event.

Errors are `RelyvoError` with `status`, `code`, `message`, `requestId` and
`retryable`. Configuration problems are `RelyvoConfigError`. Both are exported, as
are `isRelyvoError` and `isRelyvoConfigError`.

Retries apply to idempotent calls only (`whoami`, `video.config`, `chat.upsertUser`,
`realtime.listReceipts`), with exponential backoff and `Retry-After` support.
Token issuance and Realtime publish calls are never automatically retried. Chat
`createUserToken(userId, { ttlSeconds, signal })` optionally requests a shorter
lifetime; the service clamps it to its configured maximum. Responses are bounded
to 16 MiB; use smaller page limits for unusually large deployments.

## Notes

- `apiUrl` is the origin where `/v1/api/...` resolves: `https://<host>/api` through a
  dashboard deployment, or `http://localhost:8080` against the API directly.
- `chatUrl` is required for chat methods; without it they throw `RelyvoConfigError`.
- `realtimeUrl` is the Realtime data-plane origin, required for Realtime methods.
- The chat entitlement must be enabled for the project, or chat answers
  `chat_disabled`.
- Requires Node 18+ for the global `fetch`. Pass `fetch` to use a custom transport.

See the repository [README](../README.md) for the full contract, prerequisites and
release process.
