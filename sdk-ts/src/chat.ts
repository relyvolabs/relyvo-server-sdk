import { asMetadata, asOptionalText, asRecord, asText, asTimestamp } from "./decode.js";
import { RelyvoConfigError } from "./errors.js";
import { joinUrl, type Transport } from "./transport.js";
import type {
  CallOptions,
  ChatUser,
  ChatUserInput,
  ChatUserResult,
  ChatUserToken,
} from "./types.js";

/**
 * Manages chat identities and tokens.
 *
 * These calls go to the chat service, because that is where chat data lives; the
 * control plane owns admission and project policy, not messages. Every call still
 * authenticates with the Relyvo API key, which chat validates against the control
 * plane.
 */
export class ChatAPI {
  constructor(
    private readonly transport: Transport,
    private readonly chatUrl: string | undefined,
  ) {}

  /**
   * Creates or updates a chat user.
   *
   * Idempotent on (project, externalUserId), so it is safe to call on every
   * sign-in and safe to retry.
   */
  async upsertUser(input: ChatUserInput, options: CallOptions = {}): Promise<ChatUserResult> {
    const externalUserId = input.externalUserId?.trim() ?? "";
    if (externalUserId === "") {
      throw new TypeError("relyvo: externalUserId is required");
    }

    const raw = await this.transport.send({
      method: "POST",
      url: joinUrl(this.requireBase(), "/v1/chat/users"),
      body: {
        external_user_id: externalUserId,
        ...(input.displayName === undefined ? {} : { display_name: input.displayName }),
        ...(input.avatarUrl === undefined ? {} : { avatar_url: input.avatarUrl }),
        ...(input.metadata === undefined ? {} : { metadata: input.metadata }),
      },
      retryable: true,
      signal: options.signal,
    });

    const body = asRecord(raw, "chat.user");
    const data = asRecord(body["data"], "chat.user.data");
    const user = asRecord(data["user"], "chat.user.data.user");

    return {
      user: this.toUser(user),
      created: data["created"] === true,
    };
  }

  /**
   * Mints a short-lived end-user token.
   *
   * Never retried automatically: a retry would mint a second token.
   */
  async createUserToken(userId: string, options: CallOptions = {}): Promise<ChatUserToken> {
    const id = userId?.trim() ?? "";
    if (id === "") {
      throw new TypeError("relyvo: userId is required");
    }

    const raw = await this.transport.send({
      method: "POST",
      url: joinUrl(this.requireBase(), `/v1/chat/users/${encodeURIComponent(id)}/tokens`),
      body: {},
      retryable: false,
      signal: options.signal,
    });

    const body = asRecord(raw, "chat.token");
    const data = asRecord(body["data"], "chat.token.data");

    return {
      token: asText(data, "token", "chat.token.data"),
      tokenType: asOptionalText(data, "token_type") ?? "Bearer",
      scope: asOptionalText(data, "scope") ?? "chat.user",
      expiresAt: asTimestamp(data, "expires_at", "chat.token.data"),
    };
  }

  private requireBase(): string {
    if (this.chatUrl === undefined || this.chatUrl === "") {
      throw new RelyvoConfigError("relyvo: chat is not configured; set RelyvoConfig.chatUrl");
    }
    return this.chatUrl;
  }

  private toUser(source: Record<string, unknown>): ChatUser {
    return {
      id: asText(source, "id", "chat.user"),
      organizationId: asOptionalText(source, "organization_id") ?? "",
      projectId: asOptionalText(source, "project_id") ?? "",
      externalUserId: asText(source, "external_user_id", "chat.user"),
      displayName: asOptionalText(source, "display_name") ?? "",
      avatarUrl: asOptionalText(source, "avatar_url"),
      metadata: asMetadata(source, "metadata"),
      createdAt: asTimestamp(source, "created_at", "chat.user"),
      updatedAt: asTimestamp(source, "updated_at", "chat.user"),
    };
  }
}
