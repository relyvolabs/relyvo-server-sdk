import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const testDir = dirname(fileURLToPath(import.meta.url));

/** Reads a conformance fixture shared with the Go suite. */
export function fixture(name: string): string {
  return readFileSync(join(testDir, "..", "..", "spec", "fixtures", name), "utf8");
}

export function fixtureJson<T>(name: string): T {
  return JSON.parse(fixture(name)) as T;
}

export interface RecordedCall {
  url: string;
  init: RequestInit;
}

export interface FetchDouble {
  fetch: typeof globalThis.fetch;
  calls: RecordedCall[];
  headers: (index: number) => Record<string, string>;
}

/**
 * A fetch double that records calls. Every test asserts on what the SDK actually
 * sent, not on an internal method being called.
 */
export function mockFetch(
  handler: (call: RecordedCall, index: number) => Response | Promise<Response>,
): FetchDouble {
  const calls: RecordedCall[] = [];

  const fetchImpl = (async (input: string | URL | Request, init?: RequestInit) => {
    const call: RecordedCall = {
      url: typeof input === "string" ? input : input instanceof URL ? input.toString() : input.url,
      init: init ?? {},
    };
    calls.push(call);
    return handler(call, calls.length - 1);
  }) as typeof globalThis.fetch;

  return {
    fetch: fetchImpl,
    calls,
    headers: (index: number): Record<string, string> => {
      const call = calls[index];
      return (call?.init.headers ?? {}) as Record<string, string>;
    },
  };
}

/** A JSON response with the given status. */
export function jsonResponse(body: unknown, status = 200, headers: Record<string, string> = {}): Response {
  return new Response(typeof body === "string" ? body : JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json", ...headers },
  });
}

/** Parses a recorded request body. */
export function sentBody(call: RecordedCall | undefined): unknown {
  if (call === undefined || typeof call.init.body !== "string") {
    return undefined;
  }
  return JSON.parse(call.init.body) as unknown;
}
