import { RelyvoError, parseRetryAfter, toRelyvoError } from "./errors.js";

export const SDK_VERSION = "0.1.0";

export const DEFAULT_TIMEOUT_MS = 10_000;
export const DEFAULT_MAX_RETRIES = 2;
export const DEFAULT_BASE_DELAY_MS = 200;

const MAX_RETRY_DELAY_MS = 30_000;
const MAX_RESPONSE_BYTES = 1 << 20;

export interface TransportConfig {
  apiKey: string;
  fetch: typeof globalThis.fetch;
  timeoutMs: number;
  maxRetries: number;
  baseDelayMs: number;
  userAgent: string;
}

export interface SendRequest {
  method: "GET" | "POST";
  /** Absolute URL. */
  url: string;
  body?: unknown;
  /** Only idempotent calls may set this. */
  retryable: boolean;
  signal?: AbortSignal;
}

/**
 * HTTP transport shared by every service.
 *
 * It owns the pieces that must not differ between endpoints: the bearer header,
 * the JSON contract, the timeout, the retry policy and error mapping.
 */
export class Transport {
  constructor(private readonly config: TransportConfig) {}

  async send(request: SendRequest): Promise<unknown> {
    const attempts = 1 + (request.retryable ? this.config.maxRetries : 0);
    let lastError: unknown;

    for (let attempt = 0; attempt < attempts; attempt += 1) {
      if (attempt > 0) {
        await this.wait(this.delay(attempt - 1, lastError), request.signal);
      }

      try {
        return await this.attempt(request);
      } catch (error) {
        lastError = error;
        if (!(error instanceof RelyvoError) || !error.retryable) {
          throw error;
        }
      }
    }

    throw lastError;
  }

  private async attempt(request: SendRequest): Promise<unknown> {
    const controller = new AbortController();
    const timer = setTimeout(() => controller.abort(), this.config.timeoutMs);
    const onAbort = (): void => controller.abort();
    const alreadyAborted = request.signal?.aborted ?? false;
    if (alreadyAborted) {
      controller.abort();
    } else {
      request.signal?.addEventListener("abort", onAbort, { once: true });
    }

    try {
      const headers: Record<string, string> = {
        Authorization: `Bearer ${this.config.apiKey}`,
        Accept: "application/json",
        "User-Agent": this.config.userAgent,
      };

      let body: string | undefined;
      if (request.body !== undefined) {
        body = JSON.stringify(request.body);
        headers["Content-Type"] = "application/json";
      }

      let response: Response;
      try {
        response = await this.config.fetch(request.url, {
          method: request.method,
          headers,
          body,
          signal: controller.signal,
        });
      } catch (cause) {
        if (alreadyAborted || request.signal?.aborted === true) {
          throw new RelyvoError("relyvo: the request was aborted", {
            status: 0,
            code: "aborted",
            cause,
          });
        }
        const detail = cause instanceof Error ? cause.message : String(cause);
        throw new RelyvoError(`relyvo: the request did not complete: ${detail}`, {
          status: 0,
          code: "transport_error",
          cause,
        });
      }

      const declaredLength = Number(response.headers.get("content-length") ?? "0");
      if (Number.isFinite(declaredLength) && declaredLength > MAX_RESPONSE_BYTES) {
        throw new RelyvoError("relyvo: the response is too large", {
          status: response.status,
          code: "unexpected_response",
        });
      }

      let text: string;
      try {
        text = await response.text();
      } catch (cause) {
        throw new RelyvoError("relyvo: the response could not be read", {
          status: 0,
          code: "transport_error",
          cause,
        });
      }

      if (!response.ok) {
        throw toRelyvoError(
          response.status,
          text,
          parseRetryAfter(response.headers.get("retry-after")),
        );
      }

      if (text.trim() === "") {
        return undefined;
      }

      try {
        return JSON.parse(text) as unknown;
      } catch (cause) {
        throw new RelyvoError("relyvo: the response was not valid JSON", {
          status: response.status,
          code: "invalid_response",
          cause,
        });
      }
    } finally {
      clearTimeout(timer);
      request.signal?.removeEventListener("abort", onAbort);
    }
  }

  /** Exponential backoff with jitter, never shorter than Retry-After. */
  private delay(attempt: number, lastError: unknown): number {
    const backoff = this.config.baseDelayMs * 2 ** attempt;
    const jitter = Math.floor(Math.random() * (backoff / 2 + 1));
    let wait = backoff + jitter;

    if (lastError instanceof RelyvoError && lastError.retryAfterMs > wait) {
      wait = lastError.retryAfterMs;
    }

    return Math.min(wait, MAX_RETRY_DELAY_MS);
  }

  private wait(ms: number, signal?: AbortSignal): Promise<void> {
    if (ms <= 0) {
      return Promise.resolve();
    }

    return new Promise((resolve, reject) => {
      const cleanup = (): void => signal?.removeEventListener("abort", onAbort);
      const timer = setTimeout(() => {
        cleanup();
        resolve();
      }, ms);
      const onAbort = (): void => {
        clearTimeout(timer);
        cleanup();
        reject(new RelyvoError("relyvo: the request was aborted", { status: 0, code: "aborted" }));
      };

      if (signal?.aborted === true) {
        onAbort();
        return;
      }
      signal?.addEventListener("abort", onAbort, { once: true });
    });
  }
}

/** Joins an origin and a versioned path without double slashes. */
export function joinUrl(base: string, path: string): string {
  return `${base.replace(/\/+$/, "")}${path}`;
}
