import { describe, expect, it } from "vitest";

import { Relyvo, RelyvoError } from "../src/index.js";
import { fixtureJson, jsonResponse, mockFetch, sentBody } from "./helpers.js";

const config = {
  apiKey: "rv_key_secret",
  apiUrl: "https://relyvo.example.com/api",
  maxRetries: 1,
  baseDelayMs: 1,
};

describe("video.createToken", () => {
  it("sends the documented request and decodes the fixture", async () => {
    const response = fixtureJson("responses/video-token.json");
    const double = mockFetch(() => jsonResponse(response));

    const client = new Relyvo({ ...config, fetch: double.fetch });
    const token = await client.video.createToken({
      roomName: "support-42",
      participantIdentity: "user_7",
      displayName: "Ada",
      mode: "video",
    });

    expect(double.calls).toHaveLength(1);
    expect(double.calls[0]?.url).toBe("https://relyvo.example.com/api/v1/api/video/tokens");
    expect(double.calls[0]?.init.method).toBe("POST");
    expect(sentBody(double.calls[0])).toEqual(fixtureJson("requests/video-token.json"));

    expect(token.serverUrl).toBe("wss://rtc.example.com");
    expect(token.token).toMatch(/^eyJ/);
    expect(token.roomName).toBe("support-42");
    expect(token.participantIdentity).toBe("user_7");
    expect(token.mode).toBe("video");
    expect(token.meterCode).toBe("participant_seconds");
    expect(token.expiresAt.toISOString()).toBe("2026-01-01T00:15:00.000Z");
  });

  it("omits an absent display name instead of sending an empty string", async () => {
    const double = mockFetch(() => jsonResponse(fixtureJson("responses/video-token.json")));

    const client = new Relyvo({ ...config, fetch: double.fetch });
    await client.video.createToken({ roomName: "support-42", participantIdentity: "user_7" });

    expect(sentBody(double.calls[0])).toEqual({
      room_name: "support-42",
      participant_identity: "user_7",
      mode: "video",
    });
  });

  it("maps the error envelope", async () => {
    const double = mockFetch(() =>
      jsonResponse(fixtureJson("errors/insufficient-credits.json"), 402),
    );

    const client = new Relyvo({ ...config, fetch: double.fetch });

    await expect(
      client.video.createToken({ roomName: "support-42", participantIdentity: "user_7" }),
    ).rejects.toMatchObject({
      name: "RelyvoError",
      status: 402,
      code: "insufficient_credits",
      requestId: "req_fixture_0002",
      retryable: false,
    });
  });

  it("does not retry a token mint", async () => {
    const double = mockFetch(() =>
      jsonResponse(fixtureJson("errors/livekit-not-configured.json"), 503),
    );

    const client = new Relyvo({ ...config, fetch: double.fetch });

    await expect(
      client.video.createToken({ roomName: "support-42", participantIdentity: "user_7" }),
    ).rejects.toBeInstanceOf(RelyvoError);

    expect(double.calls).toHaveLength(1);
  });

  it("validates input locally", async () => {
    const double = mockFetch(() => jsonResponse({}));
    const client = new Relyvo({ ...config, fetch: double.fetch });

    await expect(client.video.createToken({ roomName: "", participantIdentity: "user_7" })).rejects.toThrow(
      /roomName is required/,
    );
    await expect(client.video.createToken({ roomName: "r", participantIdentity: " " })).rejects.toThrow(
      /participantIdentity is required/,
    );
    expect(double.calls).toHaveLength(0);
  });
});

describe("video.config", () => {
  it("reads the public connection details", async () => {
    const double = mockFetch(() => jsonResponse(fixtureJson("responses/video-config.json")));

    const client = new Relyvo({ ...config, fetch: double.fetch });
    const cfg = await client.video.config();

    expect(double.calls[0]?.url).toBe("https://relyvo.example.com/api/v1/api/video/config");
    expect(double.calls[0]?.init.method).toBe("GET");
    expect(cfg).toEqual({ serverUrl: "wss://rtc.example.com", tokenTtlSeconds: 900 });
  });

  it("retries a retryable failure", async () => {
    const double = mockFetch((_call, index) =>
      index === 0
        ? jsonResponse(fixtureJson("errors/livekit-not-configured.json"), 503, { "Retry-After": "0" })
        : jsonResponse(fixtureJson("responses/video-config.json")),
    );

    const client = new Relyvo({ ...config, fetch: double.fetch });
    const cfg = await client.video.config();

    expect(double.calls).toHaveLength(2);
    expect(cfg.tokenTtlSeconds).toBe(900);
  });
});
