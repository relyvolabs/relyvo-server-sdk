import type { RealtimeClientToken } from "./realtime-types.js";

/** Wire shape consumed by the Relyvo web SDK getCredentials callback. */
export interface RealtimeWebCredentials {
  ws_url: string;
  connection_token: string;
  expires_at: string;
  channels: { channel: string; internal_channel: string; type: "public" | "private" | "presence"; subscription_token: string; expires_at: string }[];
}

/** Return only these short-lived credentials to the authenticated browser, never the project key. */
export function toRealtimeWebCredentials(token: RealtimeClientToken): RealtimeWebCredentials {
  return { ws_url: token.wsUrl, connection_token: token.connectionToken, expires_at: token.expiresAt.toISOString(), channels: token.channels.map((channel) => ({ channel: channel.channel, internal_channel: channel.internalChannel, type: channel.type, subscription_token: channel.subscriptionToken, expires_at: channel.expiresAt.toISOString() })) };
}
