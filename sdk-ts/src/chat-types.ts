/** User-scoped Chat models mirror the service JSON (snake_case and ISO timestamps). */
export interface ChatPage<T> { items: T[]; next_cursor?: string; has_more: boolean }
export interface ChatPageOptions { limit?: number; cursor?: string }
export interface ChatMessagePageOptions extends ChatPageOptions { after?: boolean }
export interface ChatChannel {
  id: string; organization_id: string; project_id: string; type: string; name: string; description: string;
  created_by: string; metadata: Record<string, unknown> | null; last_message_id?: string; last_seq: number;
  max_members: number; external_id?: string; source_product?: string; sending_enabled: boolean;
  created_at: string; updated_at: string; deleted_at?: string; membership_role?: string;
}
export interface ChatMember {
  id: string; organization_id: string; project_id: string; channel_id: string; user_id: string; role: string;
  joined_at: string; left_at?: string; muted_until?: string; created_at: string; updated_at: string;
  user?: ChatUserWire;
}
export interface ChatUserWire { id: string; organization_id: string; project_id: string; external_user_id: string; display_name: string; avatar_url?: string; metadata: Record<string, unknown> | null; created_at: string; updated_at: string; deleted_at?: string }
export interface ChatMessage {
  id: string; organization_id: string; project_id: string; channel_id: string; sender_id: string; channel_seq: number;
  client_message_id?: string; type: string; content: string; metadata: Record<string, unknown> | null;
  reply_to_message_id?: string; created_at: string; edited_at?: string; deleted_at?: string; attachment_ids?: string[];
}
export interface ChatReadState { organization_id: string; project_id: string; channel_id: string; user_id: string; last_read_message_id?: string; last_read_seq: number; last_read_at: string; updated_at: string }
export interface ChatUnreadSummary { channel_id: string; last_seq: number; last_read_seq: number; unread_count: number; has_unread: boolean }
export interface ChatAttachment { id: string; organization_id: string; project_id: string; uploader_id: string; message_id?: string; storage_provider: string; file_name: string; mime_type: string; size_bytes: number; status: string; created_at: string; updated_at: string; expires_at?: string }
export interface ChatPreparedUpload { attachment: ChatAttachment; upload_url: string; headers: Record<string, string> | null; expires_at: string }
export interface ChatDownload { attachment: ChatAttachment; download_url: string }
export interface ChatRealtimeCredentials { token: string; ws_url: string; expires_at: string; channel_prefix: string; ttl_seconds: number }
export interface ChatSubscription { channel: string; kind: string; token: string; expires_at: string }
export interface ChatPresence { channel_id: string; presence: { client_id: string; user_id: string; info?: unknown }[] }
export interface ChatCreateChannelInput { type: "direct" | "group" | "public"; name?: string; description?: string; metadata?: Record<string, unknown>; memberIds?: string[]; otherUserId?: string }
export interface ChatUpdateChannelInput { name?: string; description?: string; metadata?: Record<string, unknown> }
export interface ChatSendMessageInput { clientMessageId?: string; type?: "text" | "attachment"; content?: string; metadata?: Record<string, unknown>; replyToMessageId?: string; attachmentIds?: string[] }
export interface ChatEditMessageInput { content: string; metadata?: Record<string, unknown> }
export interface ChatPrepareUploadInput { fileName: string; mimeType: string; sizeBytes: number }
