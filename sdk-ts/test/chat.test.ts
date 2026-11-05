import { describe, expect, it } from "vitest";

import { Relyvo, RelyvoConfigError } from "../src/index.js";
import { fixtureJson, jsonResponse, mockFetch, sentBody } from "./helpers.js";

const config = {
  apiKey: "rv_key_secret",
  apiUrl: "https://relyvo.example.com/api",
  chatUrl: "https://chat.example.com",
  maxRetries: 1,
  baseDelayMs: 1,
};

describe("chat.upsertUser", () => {
  it("sends the documented request to the chat origin", async () => {
    const double = mockFetch(() => jsonResponse(fixtureJson("responses/chat-user-created.json"), 201));

    const client = new Relyvo({ ...config, fetch: double.fetch });
    const result = await client.chat.upsertUser({
      externalUserId: "user_7",
      displayName: "Ada",
      avatarUrl: "https://example.com/ada.png",
      metadata: { plan: "pro" },
    });

    expect(double.calls[0]?.url).toBe("https://chat.example.com/v1/chat/users");
    expect(double.calls[0]?.init.method).toBe("POST");
    expect(sentBody(double.calls[0])).toEqual(fixtureJson("requests/chat-user.json"));

    expect(result.created).toBe(true);
    expect(result.user.id).toBe("0f5b3a7e-2c4d-4a91-9f60-5b1f0f4e6a11");
    expect(result.user.externalUserId).toBe("user_7");
    expect(result.user.metadata).toEqual({ plan: "pro" });
    expect(result.user.createdAt.toISOString()).toBe("2026-01-01T00:00:00.000Z");
  });

  it("reports an existing user from the second response", async () => {
    const double = mockFetch(() => jsonResponse(fixtureJson("responses/chat-user-existing.json"), 200));

    const client = new Relyvo({ ...config, fetch: double.fetch });
    const result = await client.chat.upsertUser({ externalUserId: "user_7" });

    expect(result.created).toBe(false);
    expect(result.user.metadata).toBeNull();
  });

  it("retries because an upsert is idempotent", async () => {
    const double = mockFetch((_call, index) =>
      index === 0
        ? jsonResponse({ error: { code: "unavailable", message: "try again" }, request_id: "req_1" }, 503, {
            "Retry-After": "0",
          })
        : jsonResponse(fixtureJson("responses/chat-user-existing.json")),
    );

    const client = new Relyvo({ ...config, fetch: double.fetch });
    const result = await client.chat.upsertUser({ externalUserId: "user_7" });

    expect(double.calls).toHaveLength(2);
    expect(result.user.externalUserId).toBe("user_7");
  });

  it("surfaces chat_disabled with the top-level request id", async () => {
    const double = mockFetch(() => jsonResponse(fixtureJson("errors/chat-disabled.json"), 403));

    const client = new Relyvo({ ...config, fetch: double.fetch });

    await expect(client.chat.upsertUser({ externalUserId: "user_7" })).rejects.toMatchObject({
      code: "chat_disabled",
      status: 403,
      requestId: "req_fixture_0004",
      retryable: false,
    });
  });
});

describe("chat.createUserToken", () => {
  it("mints a token for one user", async () => {
    const double = mockFetch(() => jsonResponse(fixtureJson("responses/chat-user-token.json"), 201));

    const client = new Relyvo({ ...config, fetch: double.fetch });
    const token = await client.chat.createUserToken("0f5b3a7e-2c4d-4a91-9f60-5b1f0f4e6a11");

    expect(double.calls[0]?.url).toBe(
      "https://chat.example.com/v1/chat/users/0f5b3a7e-2c4d-4a91-9f60-5b1f0f4e6a11/tokens",
    );
    expect(sentBody(double.calls[0])).toEqual({});
    expect(token.token).toMatch(/^eyJ/);
    expect(token.tokenType).toBe("Bearer");
    expect(token.scope).toBe("chat.user");
    expect(token.expiresAt.toISOString()).toBe("2026-01-01T01:00:00.000Z");
  });

  it("does not retry a token mint", async () => {
    const double = mockFetch(() => jsonResponse({ error: { code: "unavailable", message: "down" } }, 503));

    const client = new Relyvo({ ...config, fetch: double.fetch });

    await expect(client.chat.createUserToken("user-id")).rejects.toMatchObject({ status: 503 });
    expect(double.calls).toHaveLength(1);
  });

  it("requires a user id", async () => {
    const double = mockFetch(() => jsonResponse({}));
    const client = new Relyvo({ ...config, fetch: double.fetch });

    await expect(client.chat.createUserToken("")).rejects.toThrow(/userId is required/);
    expect(double.calls).toHaveLength(0);
  });
});

describe("chat configuration", () => {
  it("fails clearly when chatUrl is absent", async () => {
    const double = mockFetch(() => jsonResponse({}));
    const client = new Relyvo({
      apiKey: "rv_key_secret",
      apiUrl: "https://relyvo.example.com/api",
      fetch: double.fetch,
    });

    await expect(client.chat.upsertUser({ externalUserId: "user_7" })).rejects.toBeInstanceOf(
      RelyvoConfigError,
    );
    expect(double.calls).toHaveLength(0);
  });
});
