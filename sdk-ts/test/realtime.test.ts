import { describe, expect, it } from "vitest";
import { Relyvo, RelyvoError, toRealtimeWebCredentials } from "../src/index.js";
import { jsonResponse, mockFetch, sentBody } from "./helpers.js";

const at = "2026-01-01T00:00:00Z";
function client(fetch: typeof globalThis.fetch) { return new Relyvo({ apiKey: "rv_key_secret", apiUrl: "https://api.test", realtimeUrl: "https://rt.test/", fetch, maxRetries: 2, baseDelayMs: 0 }); }

describe("Realtime project-key API", () => {
  it("mints client credentials with project bearer auth and decodes channel tokens", async () => {
    const mock = mockFetch(() => jsonResponse({ ws_url: "wss://rt.test/connection/websocket", connection_token: "conn", expires_at: at, channels: [{ channel: "room", internal_channel: "p:room", type: "public", subscription_token: "sub", expires_at: at }] }));
    const sdk = client(mock.fetch);
    expect(sdk.call).toBe(sdk.video);
    const token = await sdk.realtime.createClientToken({ userId: "u", displayName: "Ada", channels: ["room"] });
    expect(mock.calls[0]?.url).toBe("https://rt.test/v1/realtime/client-tokens");
    expect(mock.headers(0).Authorization).toBe("Bearer rv_key_secret");
    expect(sentBody(mock.calls[0])).toEqual({ user_id: "u", display_name: "Ada", channels: ["room"] });
    expect(token.channels[0]).toEqual({ channel: "room", internalChannel: "p:room", type: "public", subscriptionToken: "sub", expiresAt: new Date(at) });
    // The web SDK's getCredentials contract uses these exact snake_case fields.
    expect(toRealtimeWebCredentials(token)).toEqual({ ws_url: "wss://rt.test/connection/websocket", connection_token: "conn", expires_at: new Date(at).toISOString(), channels: [{ channel: "room", internal_channel: "p:room", type: "public", subscription_token: "sub", expires_at: new Date(at).toISOString() }] });
  });
  it("preserves batch partial failures and ambiguous outcomes without retrying", async () => {
    const mock = mockFetch(() => jsonResponse({ results: [{ event_id: "e1", channel: "a", event: "update", accepted: true, published: true, published_at: at }, { event_id: "", channel: "b", event: "update", accepted: false, published: false, error_code: "invalid_request", error: "bad" }, { event_id: "e3", channel: "c", event: "update", outcome: "unknown", error_code: "publish_outcome_unknown" }], published: 1, failed: 1, unknown: 1 }));
    const events = [{ channel: "a", event: "update", data: { x: 1 } }];
    const result = await client(mock.fetch).realtime.publishBatch(events);
    expect(mock.calls[0]?.url).toBe("https://rt.test/v1/realtime/events/batch");
    expect(sentBody(mock.calls[0])).toEqual({ events });
    expect(result.results[2]?.accepted).toBeUndefined();
    expect(result.results[1]?.errorCode).toBe("invalid_request");
    expect(result.unknown).toBe(1);
  });
  it("preserves the 503 unknown publish result and does not retry any writes", async () => {
    const mock = mockFetch(() => jsonResponse({ event_id: "e", channel: "a", event: "update", outcome: "unknown", error_code: "publish_outcome_unknown", error: "may duplicate" }, 503));
    const sdk = client(mock.fetch);
    await expect(sdk.realtime.publish({ channel: "a", event: "update", data: null })).rejects.toMatchObject({ status: 503, code: "publish_outcome_unknown", retryable: false, details: { event_id: "e", outcome: "unknown" } });
    await expect(sdk.realtime.publishBatch([])).rejects.toBeInstanceOf(RelyvoError);
    await expect(sdk.realtime.createClientToken({ userId: "u", channels: ["a"] })).rejects.toBeInstanceOf(RelyvoError);
    expect(mock.calls).toHaveLength(3);
  });
  it("encodes receipt pagination, retries reads, and carries signal", async () => {
    const controller = new AbortController();
    const mock = mockFetch((_call, index) => index === 0 ? jsonResponse({ error: { code: "unavailable" } }, 503) : jsonResponse({ events: [{ event_id: "e", channel: "a", event: "x", payload_bytes: 3, created_at: at }], next_cursor: "" }));
    const page = await client(mock.fetch).realtime.listReceipts({ limit: 10, cursor: "a&b=+" }, { signal: controller.signal });
    expect(mock.calls).toHaveLength(2);
    expect(mock.calls[0]?.url).toBe("https://rt.test/v1/realtime/events/receipts?limit=10&cursor=a%26b%3D%2B");
    expect(mock.calls[0]?.init.signal).toBeInstanceOf(AbortSignal);
    expect(page.events[0]?.createdAt).toEqual(new Date(at));
    expect(page.nextCursor).toBe("");
  });
  it("reads unresolved pages and monitoring windows without implying delivery", async () => {
    const mock = mockFetch((_call, index) => index === 0
      ? jsonResponse({ candidates: [{ event_id: "e", channel: "room", created_at: at, status: "unresolved" }], next_cursor: "next" })
      : jsonResponse({ project_id: "p", window: "1h", published: 4, failed: 1, payload_bytes: 40, top_channels: [{ channel: "room", events: 4 }], pending_usage: 2, generated_at: at, gateway_error: "project-scoped gateway metrics are unavailable" }));
    const sdk = client(mock.fetch);
    const page = await sdk.realtime.listUnresolved({ limit: 2, cursor: "a&b" });
    const overview = await sdk.realtime.monitoringOverview("1h");
    expect(mock.calls[0]?.url).toBe("https://rt.test/v1/realtime/events/unresolved?limit=2&cursor=a%26b");
    expect(mock.calls[1]?.url).toBe("https://rt.test/v1/realtime/monitoring/overview?window=1h");
    expect(page).toEqual({ candidates: [{ eventId: "e", channel: "room", createdAt: new Date(at), status: "unresolved" }], nextCursor: "next" });
    expect(overview.topChannels).toEqual([{ channel: "room", events: 4 }]);
    expect(overview.gateway).toBeUndefined();
    expect(overview.gatewayError).toContain("unavailable");
  });
  it("propagates cancellation to an in-flight write without retry", async () => {
    const controller = new AbortController();
    const mock = mockFetch(({ init }) => new Promise((_resolve, reject) => {
      init.signal?.addEventListener("abort", () => reject(new Error("aborted")), { once: true });
    }));
    const pending = client(mock.fetch).realtime.publish({ channel: "a", event: "x", data: null }, { signal: controller.signal });
    controller.abort();
    await expect(pending).rejects.toMatchObject({ code: "aborted" });
    expect(mock.calls).toHaveLength(1);
  });
  it("requires realtimeUrl without making requests", async () => {
    const mock = mockFetch(() => jsonResponse({}));
    const sdk = new Relyvo({ apiKey: "key", apiUrl: "https://api.test", fetch: mock.fetch });
    await expect(sdk.realtime.listReceipts()).rejects.toThrow("realtimeUrl");
    expect(mock.calls).toHaveLength(0);
  });
});
