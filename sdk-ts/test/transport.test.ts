import { describe, expect, it } from "vitest";

import { Relyvo, RelyvoError } from "../src/index.js";
import { jsonResponse, mockFetch } from "./helpers.js";

const config = {
  apiKey: "rv_key_secret",
  apiUrl: "https://example.com",
  maxRetries: 3,
  baseDelayMs: 1,
};

describe("cancellation", () => {
  it("reports an aborted request without retrying", async () => {
    let calls = 0;
    const double = mockFetch((call) => {
      calls += 1;
      if (call.init.signal?.aborted === true) {
        const error = new Error("aborted");
        error.name = "AbortError";
        throw error;
      }
      return jsonResponse({ api_key: {} });
    });

    const client = new Relyvo({ ...config, fetch: double.fetch });
    const controller = new AbortController();
    controller.abort();

    await expect(client.whoami({ signal: controller.signal })).rejects.toMatchObject({
      name: "RelyvoError",
      code: "aborted",
      status: 0,
      retryable: false,
    });

    expect(calls).toBe(1);
  });

  it("wraps a network failure as a retryable transport error", async () => {
    let calls = 0;
    const double = mockFetch(() => {
      calls += 1;
      throw new Error("connection refused");
    });

    const client = new Relyvo({ ...config, maxRetries: 1, fetch: double.fetch });

    await expect(client.whoami()).rejects.toMatchObject({
      code: "transport_error",
      status: 0,
      retryable: true,
    });
    expect(calls).toBe(2);
  });
});

describe("bounded response bodies", () => {
  it.each([undefined, "10"])("rejects chunked oversized bodies despite content-length %s", async (length) => {
    let cancelled = false;
    const stream = new ReadableStream<Uint8Array>({
      start(controller) { controller.enqueue(new Uint8Array(10_000_000)); controller.enqueue(new Uint8Array(7_000_000)); },
      cancel() { cancelled = true; },
    });
    const double = mockFetch(() => new Response(stream, { status: 200, headers: length === undefined ? {} : { "content-length": length } }));
    await expect(new Relyvo({ ...config, fetch: double.fetch }).whoami()).rejects.toMatchObject({ code: "unexpected_response" });
    expect(double.calls).toHaveLength(1);
    expect(cancelled).toBe(true);
  });
  it("accepts a representative default 100-message page larger than 1 MiB", async () => {
    const item = { id: "m", organization_id: "o", project_id: "p", channel_id: "c", sender_id: "u", channel_seq: 1, type: "text", content: "x".repeat(8192), metadata: { value: "y".repeat(4000) }, created_at: "2026-01-01T00:00:00Z" };
    const data = { data: { items: Array.from({ length: 100 }, (_, index) => ({ ...item, channel_seq: index + 1 })), has_more: false } };
    expect(new TextEncoder().encode(JSON.stringify(data)).length).toBeGreaterThan(1 << 20);
    const double = mockFetch(() => jsonResponse(data));
    const page = await new Relyvo({ ...config, chatUrl: "https://chat.example.com", fetch: double.fetch }).chat.listMessages("11111111-1111-4111-8111-111111111111", "channel");
    expect(page.items).toHaveLength(100);
    expect(page.items[99]?.channel_seq).toBe(100);
    expect(page.items[0]?.content).toHaveLength(8192);
  });
  it("decodes UTF-8 split across chunks", async () => {
    const bytes = new TextEncoder().encode(JSON.stringify({ api_key: { id: "é", project_id: "p" } }));
    const pos = bytes.indexOf(0xc3);
    const stream = new ReadableStream<Uint8Array>({ start(c) { c.enqueue(bytes.slice(0, pos + 1)); c.enqueue(bytes.slice(pos + 1)); c.close(); } });
    const double = mockFetch(() => new Response(stream));
    expect((await new Relyvo({ ...config, fetch: double.fetch }).whoami()).id).toBe("é");
  });
});

describe("error formatting", () => {
  it.each([
    [{ error: { code: "bad", request_id: "nested" }, request_id: "top" }, "header", "nested"],
    [{ error: { code: "bad", request_id: "" }, request_id: "top" }, "header", "top"],
    [{ error: { code: "bad", request_id: 12 }, request_id: null }, "header", "header"],
    [{ error: { code: "bad" } }, "", undefined],
    [{ error: { code: "bad" } }, undefined, undefined],
    ["not JSON", "header", "header"],
    [{ request_id: "top" }, "header", "top"],
  ])("selects a request ID from body before header (%j)", async (body, header, expected) => {
    const mock = mockFetch(() => jsonResponse(body, 400, header === undefined ? {} : { "X-Request-Id": header }));
    await expect(new Relyvo({ ...config, fetch: mock.fetch }).whoami()).rejects.toMatchObject({
      status: 400, requestId: expected,
    });
    expect(mock.calls).toHaveLength(1);
  });

  it("keeps the code, message, status and request id", async () => {
    const double = mockFetch(() =>
      jsonResponse(
        { error: { code: "concurrent_limit", message: "Too many live sessions." }, request_id: "req_9" },
        429,
      ),
    );

    const client = new Relyvo({ ...config, maxRetries: 0, fetch: double.fetch });

    try {
      await client.whoami();
      expect.unreachable("whoami must fail");
    } catch (error) {
      expect(error).toBeInstanceOf(RelyvoError);
      const apiError = error as RelyvoError;
      expect(apiError.retryable).toBe(true);
      expect(apiError.message).toBe("Too many live sessions.");
      expect(apiError.requestId).toBe("req_9");
    }
  });
});
