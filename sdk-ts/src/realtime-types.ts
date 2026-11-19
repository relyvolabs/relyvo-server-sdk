export interface RealtimeClientTokenInput { userId: string; displayName?: string; channels: string[] }
export interface RealtimeChannelToken { channel: string; internalChannel: string; type: "public" | "private" | "presence"; subscriptionToken: string; expiresAt: Date }
export interface RealtimeClientToken { wsUrl: string; connectionToken: string; expiresAt: Date; channels: RealtimeChannelToken[] }
export interface RealtimeEventInput { channel: string; event: string; data: unknown }
/** Unknown outcomes omit accepted/published; a failed batch item can have no event ID. */
export interface RealtimePublishResult { eventId?: string; channel: string; event: string; accepted?: boolean; published?: boolean; publishedAt?: Date; outcome?: string; error?: string; errorCode?: string }
export interface RealtimeBatchResult { results: RealtimePublishResult[]; published: number; failed: number; unknown: number }
export interface RealtimeUnresolvedCandidate { eventId: string; channel: string; createdAt: Date; status: string }
export interface RealtimeUnresolvedPage { candidates: RealtimeUnresolvedCandidate[]; nextCursor: string }
export type RealtimeWindow = "15m" | "1h" | "24h" | "7d";
export interface RealtimeMonitoringOverview { projectId: string; window: string; published: number; failed: number; payloadBytes: number; topChannels: { channel: string; events: number }[] | null; pendingUsage: number; generatedAt: Date; gateway?: { connections: number; subscriptions: number }; gatewayError?: string }
export interface RealtimeReceiptOptions { limit?: number; cursor?: string }
export interface RealtimeReceipt { eventId: string; channel: string; event: string; payloadBytes: number; requestId?: string; createdAt: Date }
export interface RealtimeReceiptPage { events: RealtimeReceipt[]; /** Empty when there is no next page. */ nextCursor: string }
