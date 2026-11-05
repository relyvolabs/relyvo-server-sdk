# Vendored contracts and conformance fixtures

Two things live here, and both are deliberately read-only inputs to the SDKs.

## `openapi/api.yaml` and `openapi/chat.yaml`

Snapshots of the contracts the SDKs implement:

- `api.yaml` — Relyvo API (`relyvo-web/apps/api/openapi.yaml`), the programmatic
  `/v1/api/*` surface being the part this SDK owns.
- `chat.yaml` — Relyvo Chat (`relyvo-chat/api/openapi/openapi.yaml`), for the
  user and token endpoints a server SDK calls.

They are snapshots, not generated code: the SDKs are small, handwritten clients, and
generation would add a toolchain for two endpoints. Refresh a snapshot when the
upstream contract changes, and check whether the change is additive. A breaking
change needs a major version, not a quiet fixture edit.

## `fixtures/`

The conformance contract. `sdk-go` and `sdk-ts` both load these exact files, so a
field renamed on one side fails the other side's suite instead of shipping.

| Fixture | Pins |
| --- | --- |
| `requests/video-token.json` | The request body for a video token |
| `responses/video-token.json` | The token object, including `meter_code` |
| `responses/video-config.json` | Public connection details |
| `requests/chat-user.json` | The chat user upsert body |
| `responses/chat-user-created.json` | A new chat user (`created: true`) |
| `responses/chat-user-existing.json` | An upserted existing user (`created: false`) |
| `responses/chat-user-token.json` | A chat end-user token |
| `errors/*.json` | The error envelope and its codes |

Rules that keep this useful:

1. A fixture is **data**, never a mock object. Tests assert on decoded SDK types,
   not on the JSON shape being re-typed in the test.
2. Every error code the SDK maps specially has a fixture. A new code without a
   fixture is an untested branch.
3. Fixtures never contain a real key, token or signature. Placeholders are
   obviously fake and say so.
