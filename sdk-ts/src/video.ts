import { asNumber, asRecord, asText, asTimestamp } from "./decode.js";
import { joinUrl, type Transport } from "./transport.js";
import type { CallOptions, VideoConfig, VideoMode, VideoToken, VideoTokenInput } from "./types.js";

/**
 * Mints media credentials for the project the API key belongs to.
 *
 * The credential is join-only: there is no grant or project field, because the
 * project comes from the key and the grant is chosen by the service.
 */
export class VideoAPI {
  constructor(
    private readonly transport: Transport,
    private readonly apiUrl: string,
  ) {}

  /** Public signalling URL and participant token lifetime. */
  async config(options: CallOptions = {}): Promise<VideoConfig> {
    const raw = await this.transport.send({
      method: "GET",
      url: joinUrl(this.apiUrl, "/v1/api/video/config"),
      retryable: true,
      signal: options.signal,
    });

    const body = asRecord(raw, "video.config");
    const livekit = asRecord(body["livekit"], "video.config.livekit");

    return {
      serverUrl: asText(livekit, "server_url", "video.config.livekit"),
      tokenTtlSeconds: asNumber(livekit, "token_ttl_seconds", "video.config.livekit"),
    };
  }

  /**
   * Mints a join-only participant token.
   *
   * Never retried automatically: a retry would mint a second credential.
   */
  async createToken(input: VideoTokenInput, options: CallOptions = {}): Promise<VideoToken> {
    const roomName = input.roomName?.trim() ?? "";
    const participantIdentity = input.participantIdentity?.trim() ?? "";
    if (roomName === "") {
      throw new TypeError("relyvo: roomName is required");
    }
    if (participantIdentity === "") {
      throw new TypeError("relyvo: participantIdentity is required");
    }

    const mode: VideoMode = input.mode ?? "video";

    const raw = await this.transport.send({
      method: "POST",
      url: joinUrl(this.apiUrl, "/v1/api/video/tokens"),
      body: {
        room_name: roomName,
        participant_identity: participantIdentity,
        ...(input.displayName === undefined ? {} : { display_name: input.displayName }),
        mode,
      },
      retryable: false,
      signal: options.signal,
    });

    const body = asRecord(raw, "video.token");

    return {
      serverUrl: asText(body, "server_url", "video.token"),
      token: asText(body, "token", "video.token"),
      roomName: asText(body, "room_name", "video.token"),
      participantIdentity: asText(body, "participant_identity", "video.token"),
      expiresAt: asTimestamp(body, "expires_at", "video.token"),
      mode: asText(body, "mode", "video.token") as VideoMode,
      meterCode: asText(body, "meter_code", "video.token"),
    };
  }
}
