import { describe, expect, it } from "vitest";

import { Relyvo, RelyvoConfigError, isRelyvoConfigError } from "../src/index.js";
import { jsonResponse, mockFetch } from "./helpers.js";

describe("configuration", () => {
  it("rejects a missing API key", () => {
    expect(() => new Relyvo({ apiKey: "", apiUrl: "https://example.com" })).toThrow(RelyvoConfigError);
  });

  it("rejects a Bearer-prefixed key", () => {
    expect(
      () => new Relyvo({ apiKey: "Bearer rv_key_secret", apiUrl: "https://example.com" }),
    ).toThrow(/without a Bearer prefix/);
  });

  it("requires an absolute http(s) API URL", () => {
    expect(() => new Relyvo({ apiKey: "rv_key_secret", apiUrl: "" })).toThrow(/apiUrl is required/);
    expect(() => new Relyvo({ apiKey: "rv_key_secret", apiUrl: "not a url" })).toThrow(/absolute URL/);
    expect(() => new Relyvo({ apiKey: "rv_key_secret", apiUrl: "ftp://example.com" })).toThrow(
      /http or https/,
    );
  });

  it("refuses to construct in a browser", () => {
    const runtime = globalThis as { window?: unknown };
    runtime.window = {};
    try {
      expect(() => new Relyvo({ apiKey: "rv_key_secret", apiUrl: "https://example.com" })).toThrow(
        /refusing to run in a browser/,
      );

      // The escape hatch exists for tools that emulate a window, not for pages.
      const client = new Relyvo({
        apiKey: "rv_key_secret",
        apiUrl: "https://example.com",
        allowBrowser: true,
      });
      expect(client.video).toBeDefined();
    } finally {
      delete runtime.window;
    }
  });

  it("rejects a negative retry count", () => {
    expect(
      () =>
        new Relyvo({
          apiKey: "rv_key_secret",
          apiUrl: "https://example.com",
          maxRetries: -1,
        }),
    ).toThrow(/must not be negative/);
  });

  it("exposes config errors through the type guard", () => {
    try {
      new Relyvo({ apiKey: "", apiUrl: "https://example.com" });
      expect.unreachable("constructor must throw");
    } catch (error) {
      expect(isRelyvoConfigError(error)).toBe(true);
    }
  });
});

describe("whoami", () => {
  it("resolves the project and normalizes the base URL", async () => {
    const double = mockFetch(() =>
      jsonResponse({
        api_key: {
          id: "6a0d2f54-9b3c-4c1e-8f27-1d2b3c4d5e6f",
          key_id: "key_abc",
          project_id: "8c1e4b62-7d5a-4f39-9a11-2e3f4a5b6c7d",
          environment: "production",
          scopes: ["video:write"],
        },
      }),
    );

    const client = new Relyvo({
      apiKey: "rv_key_secret",
      apiUrl: "https://example.com/api/",
      fetch: double.fetch,
    });

    const info = await client.whoami();

    expect(double.calls).toHaveLength(1);
    expect(double.calls[0]?.url).toBe("https://example.com/api/v1/api/whoami");
    expect(double.calls[0]?.init.method).toBe("GET");
    expect(double.headers(0)).toMatchObject({
      Authorization: "Bearer rv_key_secret",
      Accept: "application/json",
    });
    expect(double.headers(0)["User-Agent"]).toMatch(/^relyvo-server-sdk-node\/0\.1\.0/);
    expect(info.projectId).toBe("8c1e4b62-7d5a-4f39-9a11-2e3f4a5b6c7d");
    expect(info.scopes).toEqual(["video:write"]);
  });

  it("rejects a response without a project", async () => {
    const double = mockFetch(() => jsonResponse({ api_key: {} }));
    const client = new Relyvo({
      apiKey: "rv_key_secret",
      apiUrl: "https://example.com",
      fetch: double.fetch,
    });

    await expect(client.whoami()).rejects.toMatchObject({ code: "invalid_response" });
  });
});
