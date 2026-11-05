import { RelyvoError } from "./errors.js";

/** A contract violation: the service answered 2xx with an unexpected shape. */
export function invalidResponse(what: string): RelyvoError {
  return new RelyvoError(`relyvo: the response does not match the contract (${what})`, {
    status: 0,
    code: "invalid_response",
  });
}

export function asRecord(value: unknown, what: string): Record<string, unknown> {
  if (typeof value !== "object" || value === null || Array.isArray(value)) {
    throw invalidResponse(what);
  }
  return value as Record<string, unknown>;
}

export function asText(source: Record<string, unknown>, field: string, what: string): string {
  const value = source[field];
  if (typeof value !== "string" || value === "") {
    throw invalidResponse(`${what}.${field}`);
  }
  return value;
}

export function asOptionalText(
  source: Record<string, unknown>,
  field: string,
): string | undefined {
  const value = source[field];
  return typeof value === "string" && value !== "" ? value : undefined;
}

export function asNumber(source: Record<string, unknown>, field: string, what: string): number {
  const value = source[field];
  if (typeof value !== "number" || !Number.isFinite(value)) {
    throw invalidResponse(`${what}.${field}`);
  }
  return value;
}

export function asTimestamp(source: Record<string, unknown>, field: string, what: string): Date {
  const value = source[field];
  if (typeof value !== "string") {
    throw invalidResponse(`${what}.${field}`);
  }

  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) {
    throw invalidResponse(`${what}.${field}`);
  }
  return parsed;
}

export function asMetadata(
  source: Record<string, unknown>,
  field: string,
): Record<string, unknown> | null {
  const value = source[field];
  if (value === null || value === undefined) {
    return null;
  }
  if (typeof value !== "object" || Array.isArray(value)) {
    throw invalidResponse(field);
  }
  return value as Record<string, unknown>;
}
