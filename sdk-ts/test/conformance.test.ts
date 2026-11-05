import { readdirSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

import { describe, expect, it } from "vitest";

import { Relyvo } from "../src/index.js";
import { fixtureJson, jsonResponse, mockFetch } from "./helpers.js";

const fixturesDir = join(dirname(fileURLToPath(import.meta.url)), "..", "..", "spec", "fixtures", "errors");

interface Envelope {
  error: { code: string; message: string };
  request_id?: string;
}

const files = readdirSync(fixturesDir).filter((name) => name.endsWith(".json"));

/**
 * The fixture directory and the error mapping must stay in lockstep: a code with
 * no fixture is an untested branch, and a fixture the SDK cannot decode is a lie.
 */
describe("error fixtures", () => {
  it("has at least one fixture", () => {
    expect(files.length).toBeGreaterThan(0);
  });

  for (const file of files) {
    it(`maps ${file}`, async () => {
      const envelope = fixtureJson<Envelope>(`errors/${file}`);
      const double = mockFetch(() => jsonResponse(envelope, 400));

      const client = new Relyvo({
        apiKey: "rv_key_secret",
        apiUrl: "https://example.com",
        fetch: double.fetch,
        maxRetries: 0,
      });

      await expect(client.whoami()).rejects.toMatchObject({
        code: envelope.error.code,
        status: 400,
        requestId: envelope.request_id,
      });
    });
  }
});
