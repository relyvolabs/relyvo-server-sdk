/**
 * The one error type every failed call produces.
 *
 * `status` is 0 when the request never completed. `code` is the service's stable
 * machine code (`insufficient_credits`, `chat_disabled`, …); `transport_error` and
 * `unexpected_response` are produced by the SDK itself.
 */
export class RelyvoError extends Error {
  readonly status: number;
  readonly code: string;
  readonly requestId?: string;
  readonly details?: unknown;
  /** Milliseconds the service asked us to wait, when it said. */
  readonly retryAfterMs: number;

  constructor(
    message: string,
    options: {
      status: number;
      code: string;
      requestId?: string;
      details?: unknown;
      retryAfterMs?: number;
      cause?: unknown;
    },
  ) {
    super(message, options.cause === undefined ? undefined : { cause: options.cause });
    this.name = "RelyvoError";
    this.status = options.status;
    this.code = options.code;
    this.requestId = options.requestId;
    this.details = options.details;
    this.retryAfterMs = options.retryAfterMs ?? 0;
  }

  /**
   * Whether a later attempt could succeed. This describes the failure, not the
   * call: the caller still decides whether the operation is safe to repeat.
   */
  get retryable(): boolean {
    if (this.status === 0) {
      return this.code === "transport_error";
    }
    return this.status === 429 || this.status === 502 || this.status === 503 || this.status === 504;
  }
}

/** A configuration problem, thrown before any request is made. */
export class RelyvoConfigError extends Error {
  constructor(message: string) {
    super(message);
    this.name = "RelyvoConfigError";
  }
}

export function isRelyvoError(error: unknown): error is RelyvoError {
  return error instanceof RelyvoError;
}

export function isRelyvoConfigError(error: unknown): error is RelyvoConfigError {
  return error instanceof RelyvoConfigError;
}

/** Error envelope: the API nests `request_id` inside `error`, chat does not. */
export interface ErrorEnvelope {
  error?: {
    code?: unknown;
    message?: unknown;
    details?: unknown;
    request_id?: unknown;
  };
  request_id?: unknown;
}

function firstString(...values: unknown[]): string | undefined {
  for (const value of values) {
    if (typeof value === "string" && value !== "") {
      return value;
    }
  }
  return undefined;
}

/** Parses `Retry-After`, accepting seconds and HTTP dates. */
export function parseRetryAfter(value: string | null): number {
  if (!value) {
    return 0;
  }

  const trimmed = value.trim();
  if (trimmed === "") {
    return 0;
  }

  if (/^\d+$/.test(trimmed)) {
    return Number(trimmed) * 1000;
  }

  const when = Date.parse(trimmed);
  if (Number.isNaN(when)) {
    return 0;
  }
  return Math.max(0, when - Date.now());
}

/** Maps a non-2xx response body onto a RelyvoError. */
export function toRelyvoError(status: number, body: string, retryAfterMs = 0): RelyvoError {
  let envelope: ErrorEnvelope | undefined;
  try {
    const parsed: unknown = JSON.parse(body);
    if (typeof parsed === "object" && parsed !== null) {
      envelope = parsed as ErrorEnvelope;
    }
  } catch {
    envelope = undefined;
  }

  const code = firstString(envelope?.error?.code);
  if (!code) {
    return new RelyvoError(`the service returned HTTP ${status}`, {
      status,
      code: "unexpected_response",
      retryAfterMs,
    });
  }

  const message = firstString(envelope?.error?.message) ?? `HTTP ${status}`;
  const requestId = firstString(envelope?.error?.request_id, envelope?.request_id);

  return new RelyvoError(message, {
    status,
    code,
    requestId,
    details: envelope?.error?.details,
    retryAfterMs,
  });
}
