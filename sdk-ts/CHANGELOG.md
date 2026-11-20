# Changelog

## 0.2.0 — 2026-11-20

- Add project-key Realtime token, publish, batch and receipt APIs and a web credential adapter.
- Add explicit acting-user Chat channel, membership, message, read state, realtime and attachment APIs.
- Add `call` as an alias of `video`; existing video and Chat identity methods remain available.
- Bound streamed response bodies to 16 MiB even when Content-Length is missing or incorrect.
- Preserve `X-Request-Id` on HTTP errors when the response body does not provide a request ID, including ambiguous Realtime publishes.

New Chat user-scoped response models mirror backend JSON, including snake_case keys and ISO timestamp strings. Existing SDK response shapes are unchanged. All user-scoped writes and credential minting are non-retrying; read calls retain normal retry behavior.
