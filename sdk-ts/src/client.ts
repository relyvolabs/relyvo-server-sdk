import { ChatAPI } from "./chat.js";
import { asOptionalText, asRecord, asText } from "./decode.js";
import { RelyvoConfigError } from "./errors.js";
import {
  DEFAULT_BASE_DELAY_MS,
  DEFAULT_MAX_RETRIES,
  DEFAULT_TIMEOUT_MS,
  SDK_VERSION,
  Transport,
  joinUrl,
} from "./transport.js";
import type { APIKeyInfo, CallOptions, RelyvoConfig } from "./types.js";
import { VideoAPI } from "./video.js";

/** True when the module was loaded somewhere that has a global window. */
function isBrowser(): boolean {
  return typeof (globalThis as { window?: unknown }).window !== "undefined";
}

function normalizeBaseUrl(field: string, value: string | undefined, required: boolean): string {
  const trimmed = (value ?? "").trim().replace(/\/+$/, "");
  if (trimmed === "") {
    if (required) {
      throw new RelyvoConfigError(`relyvo: config.${field} is required`);
    }
    return "";
  }

  let parsed: URL;
  try {
    parsed = new URL(trimmed);
  } catch {
    throw new RelyvoConfigError(`relyvo: config.${field} must be an absolute URL`);
  }

  if (parsed.protocol !== "http:" && parsed.protocol !== "https:") {
    throw new RelyvoConfigError(`relyvo: config.${field} must use http or https`);
  }

  return trimmed;
}

/**
 * Server-side client for the Relyvo platform.
 *
 * It exists to exchange an API key for the short-lived credentials a browser
 * needs. The key never leaves the server: only minted tokens do.
 *
 * Construct it once and share it; the client is stateless and safe to use from
 * concurrent requests.
 */
export class Relyvo {
  /** Mints media credentials. */
  readonly video: VideoAPI;

  /** Manages chat identities and tokens. */
  readonly chat: ChatAPI;

  private readonly transport: Transport;
  private readonly apiUrl: string;

  constructor(config: RelyvoConfig) {
    if (!config.allowBrowser && isBrowser()) {
      throw new RelyvoConfigError(
        "relyvo: refusing to run in a browser because the API key would be exposed to every visitor; " +
          "construct the client on your server and send only the minted token to the browser",
      );
    }

    const apiKey = (config.apiKey ?? "").trim();
    if (apiKey === "") {
      throw new RelyvoConfigError("relyvo: config.apiKey is required");
    }
    if (apiKey.toLowerCase().startsWith("bearer ")) {
      throw new RelyvoConfigError(
        "relyvo: config.apiKey must be the raw rv_<keyID>_<secret> value, without a Bearer prefix",
      );
    }

    this.apiUrl = normalizeBaseUrl("apiUrl", config.apiUrl, true);
    const chatUrl = normalizeBaseUrl("chatUrl", config.chatUrl, false);

    const fetchImpl = config.fetch ?? globalThis.fetch;
    if (typeof fetchImpl !== "function") {
      throw new RelyvoConfigError(
        "relyvo: no fetch implementation found; pass config.fetch on a runtime without a global fetch",
      );
    }

    const maxRetries = config.maxRetries ?? DEFAULT_MAX_RETRIES;
    if (maxRetries < 0) {
      throw new RelyvoConfigError("relyvo: config.maxRetries must not be negative");
    }

    const suffix = (config.userAgent ?? "").trim();
    const userAgent = suffix === "" ? `relyvo-server-sdk-node/${SDK_VERSION}` : `relyvo-server-sdk-node/${SDK_VERSION} ${suffix}`;

    this.transport = new Transport({
      apiKey,
      fetch: fetchImpl,
      timeoutMs: config.timeoutMs ?? DEFAULT_TIMEOUT_MS,
      maxRetries,
      baseDelayMs: config.baseDelayMs ?? DEFAULT_BASE_DELAY_MS,
      userAgent,
    });

    this.video = new VideoAPI(this.transport, this.apiUrl);
    this.chat = new ChatAPI(this.transport, chatUrl === "" ? undefined : chatUrl);
  }

  /**
   * Resolves the key's project.
   *
   * Useful as a startup check: it fails fast on a revoked or mistyped key, before
   * the first user tries to start a call.
   */
  async whoami(options: CallOptions = {}): Promise<APIKeyInfo> {
    const raw = await this.transport.send({
      method: "GET",
      url: joinUrl(this.apiUrl, "/v1/api/whoami"),
      retryable: true,
      signal: options.signal,
    });

    const body = asRecord(raw, "whoami");
    const key = asRecord(body["api_key"], "whoami.api_key");

    const projectId = asText(key, "project_id", "whoami.api_key");
    const scopes = Array.isArray(key["scopes"])
      ? key["scopes"].filter((scope): scope is string => typeof scope === "string")
      : [];

    return {
      id: asText(key, "id", "whoami.api_key"),
      keyId: asOptionalText(key, "key_id") ?? "",
      projectId,
      environment: asOptionalText(key, "environment") ?? "",
      scopes,
    };
  }
}

export { SDK_VERSION };
