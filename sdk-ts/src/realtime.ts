import { asNumber, asOptionalText, asRecord, asText, asTimestamp, invalidResponse } from "./decode.js";
import { RelyvoConfigError } from "./errors.js";
import { joinUrl, type Transport } from "./transport.js";
import type { CallOptions } from "./types.js";
import type { RealtimeBatchResult, RealtimeClientToken, RealtimeClientTokenInput, RealtimeEventInput, RealtimePublishResult, RealtimeReceiptPage, RealtimeReceiptOptions, RealtimeUnresolvedPage, RealtimeMonitoringOverview, RealtimeWindow } from "./realtime-types.js";

/** Project-key operations on the Realtime data plane. */
export class RealtimeAPI {
  constructor(private readonly transport: Transport, private readonly baseUrl: string | undefined) {}

  private url(path: string): string {
    if (!this.baseUrl) throw new RelyvoConfigError("relyvo: realtime is not configured; set RelyvoConfig.realtimeUrl");
    return joinUrl(this.baseUrl, `/v1/realtime${path}`);
  }

  async createClientToken(input: RealtimeClientTokenInput, options: CallOptions = {}): Promise<RealtimeClientToken> {
    const raw = await this.transport.send({ method: "POST", url: this.url("/client-tokens"), body: {
      user_id: input.userId, channels: input.channels,
      ...(input.displayName === undefined ? {} : { display_name: input.displayName }),
    }, retryable: false, signal: options.signal });
    const body = asRecord(raw, "realtime.clientToken");
    if (!Array.isArray(body.channels)) throw invalidResponse("realtime.clientToken.channels");
    return {
      wsUrl: asText(body, "ws_url", "realtime.clientToken"),
      connectionToken: asText(body, "connection_token", "realtime.clientToken"),
      expiresAt: asTimestamp(body, "expires_at", "realtime.clientToken"),
      channels: body.channels.map((value) => {
        const channel = asRecord(value, "realtime.clientToken.channel");
        const type = asText(channel, "type", "realtime.clientToken.channel");
        if (type !== "public" && type !== "private" && type !== "presence") throw invalidResponse("realtime.clientToken.channel.type");
        return { channel: asText(channel, "channel", "realtime.clientToken.channel"), internalChannel: asText(channel, "internal_channel", "realtime.clientToken.channel"), type, subscriptionToken: asText(channel, "subscription_token", "realtime.clientToken.channel"), expiresAt: asTimestamp(channel, "expires_at", "realtime.clientToken.channel") };
      }),
    };
  }

  /** Acknowledgement is not subscriber delivery; uncertain outcomes must not be blindly retried. */
  async publish(input: RealtimeEventInput, options: CallOptions = {}): Promise<RealtimePublishResult> {
    return decodeResult(await this.transport.send({ method: "POST", url: this.url("/events"), body: input, retryable: false, signal: options.signal }));
  }

  async publishBatch(events: RealtimeEventInput[], options: CallOptions = {}): Promise<RealtimeBatchResult> {
    const body = asRecord(await this.transport.send({ method: "POST", url: this.url("/events/batch"), body: { events }, retryable: false, signal: options.signal }), "realtime.batch");
    if (!Array.isArray(body.results)) throw invalidResponse("realtime.batch.results");
    return { results: body.results.map(decodeResult), published: asNumber(body, "published", "realtime.batch"), failed: asNumber(body, "failed", "realtime.batch"), unknown: asNumber(body, "unknown", "realtime.batch") };
  }

  /** Unresolved attempts may be in flight or already accepted; do not blindly retry. */
  async listUnresolved(input: RealtimeReceiptOptions = {}, options: CallOptions = {}): Promise<RealtimeUnresolvedPage> {
    const query = new URLSearchParams();
    if (input.limit !== undefined) query.set("limit", String(input.limit));
    if (input.cursor !== undefined) query.set("cursor", input.cursor);
    const suffix = query.toString();
    const body = asRecord(await this.transport.send({ method: "GET", url: this.url(`/events/unresolved${suffix ? `?${suffix}` : ""}`), retryable: true, signal: options.signal }), "realtime.unresolved");
    if (!Array.isArray(body.candidates) || typeof body.next_cursor !== "string") throw invalidResponse("realtime.unresolved");
    return { candidates: body.candidates.map((value) => {
      const item = asRecord(value, "realtime.unresolved.candidate");
      return { eventId: asText(item, "event_id", "realtime.unresolved.candidate"), channel: asText(item, "channel", "realtime.unresolved.candidate"), createdAt: asTimestamp(item, "created_at", "realtime.unresolved.candidate"), status: asText(item, "status", "realtime.unresolved.candidate") };
    }), nextCursor: body.next_cursor };
  }

  /** Project-scoped monitoring; gateway-wide counters may be unavailable. */
  async monitoringOverview(window?: RealtimeWindow, options: CallOptions = {}): Promise<RealtimeMonitoringOverview> {
    const query = window === undefined ? "" : `?${new URLSearchParams({ window })}`;
    const body = asRecord(await this.transport.send({ method: "GET", url: this.url(`/monitoring/overview${query}`), retryable: true, signal: options.signal }), "realtime.monitoring");
    if (body.top_channels !== null && !Array.isArray(body.top_channels)) throw invalidResponse("realtime.monitoring.top_channels");
    const topChannels = Array.isArray(body.top_channels) ? body.top_channels.map((value) => {
      const item = asRecord(value, "realtime.monitoring.channel");
      return { channel: asText(item, "channel", "realtime.monitoring.channel"), events: asNumber(item, "events", "realtime.monitoring.channel") };
    }) : null;
    const gateway = body.gateway === undefined ? undefined : asRecord(body.gateway, "realtime.monitoring.gateway");
    return { projectId: asText(body, "project_id", "realtime.monitoring"), window: asText(body, "window", "realtime.monitoring"), published: asNumber(body, "published", "realtime.monitoring"), failed: asNumber(body, "failed", "realtime.monitoring"), payloadBytes: asNumber(body, "payload_bytes", "realtime.monitoring"), topChannels, pendingUsage: asNumber(body, "pending_usage", "realtime.monitoring"), generatedAt: asTimestamp(body, "generated_at", "realtime.monitoring"), gateway: gateway ? { connections: asNumber(gateway, "connections", "realtime.monitoring.gateway"), subscriptions: asNumber(gateway, "subscriptions", "realtime.monitoring.gateway") } : undefined, gatewayError: asOptionalText(body, "gateway_error") };
  }

  /** Best-effort published receipts; absence is not proof of failure or delivery. */
  async listReceipts(input: RealtimeReceiptOptions = {}, options: CallOptions = {}): Promise<RealtimeReceiptPage> {
    const query = new URLSearchParams();
    if (input.limit !== undefined) query.set("limit", String(input.limit));
    if (input.cursor !== undefined) query.set("cursor", input.cursor);
    const suffix = query.toString();
    const body = asRecord(await this.transport.send({ method: "GET", url: this.url(`/events/receipts${suffix ? `?${suffix}` : ""}`), retryable: true, signal: options.signal }), "realtime.receipts");
    if (!Array.isArray(body.events) || typeof body.next_cursor !== "string") throw invalidResponse("realtime.receipts");
    return { events: body.events.map((value) => {
      const item = asRecord(value, "realtime.receipt");
      return { eventId: asText(item, "event_id", "realtime.receipt"), channel: asText(item, "channel", "realtime.receipt"), event: asText(item, "event", "realtime.receipt"), payloadBytes: asNumber(item, "payload_bytes", "realtime.receipt"), requestId: asOptionalText(item, "request_id"), createdAt: asTimestamp(item, "created_at", "realtime.receipt") };
    }), nextCursor: body.next_cursor };
  }
}

function decodeResult(value: unknown): RealtimePublishResult {
  const body = asRecord(value, "realtime.publish");
  for (const key of ["accepted", "published"] as const) {
    if (body[key] !== undefined && typeof body[key] !== "boolean") throw invalidResponse(`realtime.publish.${key}`);
  }
  if (typeof body.channel !== "string" || typeof body.event !== "string") throw invalidResponse("realtime.publish.channel/event");
  return { eventId: asOptionalText(body, "event_id"), channel: body.channel, event: body.event, accepted: body.accepted as boolean | undefined, published: body.published as boolean | undefined, publishedAt: body.published_at === undefined ? undefined : asTimestamp(body, "published_at", "realtime.publish"), outcome: asOptionalText(body, "outcome"), error: asOptionalText(body, "error"), errorCode: asOptionalText(body, "error_code") };
}
