export type VideoMode = "video" | "audio";

/** Options accepted by every call. */
export interface CallOptions {
  /** Cancels the call, including a pending retry. */
  signal?: AbortSignal;
}

export interface RelyvoConfig {
  /** Project API key, `rv_<keyID>_<secret>`. Server-side only. */
  apiKey: string;

  /**
   * Origin where `/v1/api/...` resolves. Through a dashboard deployment that is
   * `https://<host>/api`; against the API directly it is `http://localhost:8080`.
   */
  apiUrl: string;

  /** Chat service origin. Required only for chat methods. */
  chatUrl?: string;

  /** Custom fetch, for a proxy, instrumentation or a test double. */
  fetch?: typeof globalThis.fetch;

  /** Bounds one attempt. Defaults to 10s. */
  timeoutMs?: number;

  /** Additional attempts for idempotent calls. Defaults to 2; 0 disables retries. */
  maxRetries?: number;

  /** First backoff step. Defaults to 200ms. */
  baseDelayMs?: number;

  /** Suffix appended to the SDK User-Agent. */
  userAgent?: string;

  /**
   * Permits construction in a browser. Off by default: the constructor receives
   * the API key, and a bundled key is a leaked key.
   */
  allowBrowser?: boolean;
}

export interface VideoTokenInput {
  /** 1-128 chars: letters, digits, dot, dash, colon or underscore. */
  roomName: string;
  /** 1-128 chars: letters, digits, dot, dash, colon, at sign or underscore. */
  participantIdentity: string;
  /** Shown to other participants. At most 128 chars. */
  displayName?: string;
  /** Defaults to `video`. */
  mode?: VideoMode;
}

export interface VideoToken {
  /** Browser-facing signalling URL. */
  serverUrl: string;
  /** Short-lived JWT. Send it to the browser. */
  token: string;
  roomName: string;
  participantIdentity: string;
  expiresAt: Date;
  mode: VideoMode;
  /** The meter that will be billed for this session. */
  meterCode: string;
}

export interface VideoConfig {
  serverUrl: string;
  tokenTtlSeconds: number;
}

export interface ChatUserInput {
  /**
   * Your own user id. It is the idempotency key: the same value always resolves to
   * the same chat user.
   */
  externalUserId: string;
  displayName?: string;
  avatarUrl?: string;
  metadata?: Record<string, unknown>;
}

export interface ChatUser {
  id: string;
  organizationId: string;
  projectId: string;
  externalUserId: string;
  displayName: string;
  avatarUrl?: string;
  metadata: Record<string, unknown> | null;
  createdAt: Date;
  updatedAt: Date;
}

export interface ChatUserResult {
  user: ChatUser;
  /** Whether this call created the user. */
  created: boolean;
}

export interface ChatUserToken {
  /** Scoped end-user JWT. Send it to the browser. */
  token: string;
  tokenType: string;
  scope: string;
  expiresAt: Date;
}

export interface APIKeyInfo {
  id: string;
  keyId: string;
  projectId: string;
  environment: string;
  scopes: string[];
}
