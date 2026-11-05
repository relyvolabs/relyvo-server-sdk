# Changelog

All notable changes to the Relyvo server SDKs. Versions are independent per
language: a Go release does not imply a TypeScript release, and vice versa.

## Unreleased

## 0.1.0 — 2026-11-05

First release of `sdk-go` and `sdk-ts`.

### Added

- `Video.CreateToken` / `video.createToken`: mints a join-only participant token
  for the project the API key belongs to (`POST /v1/api/video/tokens`).
- `Video.Config` / `video.config`: public signalling URL and token lifetime
  (`GET /v1/api/video/config`).
- `Whoami` / `whoami`: resolves the key's project, environment and scopes
  (`GET /v1/api/whoami`).
- `Chat.UpsertUser` / `chat.upsertUser`: idempotent chat identity provisioning
  (`POST {chat}/v1/chat/users`).
- `Chat.CreateUserToken` / `chat.createUserToken`: scoped end-user token
  (`POST {chat}/v1/chat/users/{id}/tokens`).
- One typed error per failure (`RelyvoError` / `*relyvo.Error`) carrying status,
  stable code, message and request id, with `Retryable`/`retryable` semantics.
- Retries for idempotent calls only, with exponential backoff, jitter and
  `Retry-After` support.
- Conformance fixtures under `spec/fixtures`, exercised by both language suites.
- Browser guard in the TypeScript SDK so an API key cannot be bundled by accident.

### Known limitations

- No meetings, channels, messages, read state or attachments yet.
- API-key scopes are sent but not enforced by the services, so the SDK does not
  rely on them.
- The chat REST API must be exposed to the backend at `chatUrl`; the deployment
  notes in the README explain why.
