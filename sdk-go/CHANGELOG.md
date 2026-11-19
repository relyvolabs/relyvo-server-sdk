# Changelog

## 0.2.0

- Add Realtime project-key client token, event publish, batch publish and
  paginated receipt and unresolved-candidate operations, and monitoring using
  the configured `RealtimeURL`. Explicitly
  unknown single-publish outcomes retain their event details on the SDK error.
- Add `Client.Call` as an alias of the existing `Client.Video` service.
- Add project-scoped Chat user lookup and update, and optional requested user-token TTL.
- Add user-scoped Chat channel, member, message, read-state, realtime-token and
  attachment, typing and presence operations with an explicit acting user.
- Increase the bounded response limit to 16 MiB and return an explicit error
  when it is exceeded, allowing default-sized Chat pages to be decoded.
- Clarify that zero `MaxRetries` selects the default and a negative value
  disables retries; existing behavior is unchanged.
- Validate user-scoped Chat actors as internal UUID-shaped IDs, matching the
  TypeScript SDK. Callers must still independently authorize the actor; external
  IDs previously accepted by the Go SDK are now rejected.
