# Server SDK examples

- `ts/call.ts` — issue a join-scoped Call token.
- `ts/chat.ts` — create a channel as an explicitly authorized internal Chat user.
- `ts/realtime.ts` — mint browser Realtime credentials and map them to wire JSON.
- `go/capabilities/examples.go` — compile-checked Go equivalents.
- `ts/main.ts` — original minimal backend hand-off example.

Run `pnpm -C examples/ts run typecheck` and `cd examples/go && go test ./...`. The examples use placeholder identifiers; authenticate and authorize the user and channels in your application before calling them. They require a real project/API key only when actually executed, not for typechecking.
