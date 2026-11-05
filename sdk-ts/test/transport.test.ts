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

describe("error formatting", () => {
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
