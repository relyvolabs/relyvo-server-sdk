import { asRecord, invalidResponse } from "./decode.js";
import { validateChatResponse } from "./chat-validate.js";
import { RelyvoConfigError } from "./errors.js";
import { joinUrl, type Transport, type SendRequest } from "./transport.js";
import type { CallOptions } from "./types.js";
import type { ChatAttachment, ChatChannel, ChatCreateChannelInput, ChatDownload, ChatEditMessageInput, ChatMember, ChatMessage, ChatMessagePageOptions, ChatPage, ChatPageOptions, ChatPrepareUploadInput, ChatPreparedUpload, ChatPresence, ChatReadState, ChatRealtimeCredentials, ChatSendMessageInput, ChatSubscription, ChatUnreadSummary, ChatUpdateChannelInput } from "./chat-types.js";

/** User-scoped operations. actor must be a trusted internal chat user UUID. */
export class ChatScopedAPI {
  constructor(private readonly scopedTransport: Transport, private readonly scopedBase: string | undefined) {}

  private async request<T>(actor: string, method: SendRequest["method"], path: string, body: unknown, options: CallOptions): Promise<T> {
    const id = actor?.trim() ?? "";
    if (!/^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i.test(id)) throw new TypeError("relyvo: chat actor must be an internal chat user UUID");
    if (!this.scopedBase) throw new RelyvoConfigError("relyvo: chat is not configured; set RelyvoConfig.chatUrl");
    const raw = await this.scopedTransport.send({ method, url: joinUrl(this.scopedBase, `/v1/chat${path}`), body, retryable: method === "GET", signal: options.signal, chatActor: id });
    if (method === "DELETE" || (method === "POST" && path.endsWith("/typing"))) return undefined as T;
    const envelope = asRecord(raw, "chat.response");
    const data = asRecord(envelope.data, "chat.response.data");
    validateChatResponse(method, path.split("?")[0]!, data);
    return data as T;
  }
  private page(options: ChatPageOptions, after = false): string {
    const q = new URLSearchParams();
    if (options.limit !== undefined) q.set("limit", String(options.limit));
    if (options.cursor !== undefined) q.set("cursor", options.cursor);
    if (after) q.set("direction", "after");
    const query = q.toString();
    return query ? `?${query}` : "";
  }
  private id(id: string): string { if (!id?.trim()) throw new TypeError("relyvo: identifier is required"); return encodeURIComponent(id); }
  private channel(id: string): string { return `/channels/${this.id(id)}`; }

  createChannel(actor: string, input: ChatCreateChannelInput, options: CallOptions = {}): Promise<{ channel: ChatChannel; created: boolean }> {
    return this.request(actor, "POST", "/channels", { type: input.type, name: input.name, description: input.description, metadata: input.metadata, member_ids: input.memberIds, other_user_id: input.otherUserId }, options);
  }
  listChannels(actor: string, page: ChatPageOptions = {}, options: CallOptions = {}): Promise<ChatPage<ChatChannel>> { return this.request(actor, "GET", `/channels${this.page(page)}`, undefined, options); }
  getChannel(actor: string, channelId: string, options: CallOptions = {}): Promise<ChatChannel> { return this.request(actor, "GET", this.channel(channelId), undefined, options); }
  updateChannel(actor: string, channelId: string, input: ChatUpdateChannelInput, options: CallOptions = {}): Promise<ChatChannel> { return this.request(actor, "PATCH", this.channel(channelId), input, options); }
  deleteChannel(actor: string, channelId: string, options: CallOptions = {}): Promise<void> { return this.request(actor, "DELETE", this.channel(channelId), undefined, options); }
  addMember(actor: string, channelId: string, userId: string, role?: "admin" | "member", options: CallOptions = {}): Promise<{ member: ChatMember; added: boolean }> { return this.request(actor, "POST", `${this.channel(channelId)}/members`, { user_id: userId, role }, options); }
  listMembers(actor: string, channelId: string, page: ChatPageOptions = {}, options: CallOptions = {}): Promise<ChatPage<ChatMember>> { return this.request(actor, "GET", `${this.channel(channelId)}/members${this.page(page)}`, undefined, options); }
  removeMember(actor: string, channelId: string, userId: string, options: CallOptions = {}): Promise<void> { return this.request(actor, "DELETE", `${this.channel(channelId)}/members/${this.id(userId)}`, undefined, options); }
  sendMessage(actor: string, channelId: string, input: ChatSendMessageInput, options: CallOptions = {}): Promise<{ message: ChatMessage; created: boolean }> {
    return this.request(actor, "POST", `${this.channel(channelId)}/messages`, { client_message_id: input.clientMessageId, type: input.type, content: input.content, metadata: input.metadata, reply_to_message_id: input.replyToMessageId, attachment_ids: input.attachmentIds }, options);
  }
  listMessages(actor: string, channelId: string, page: ChatMessagePageOptions = {}, options: CallOptions = {}): Promise<ChatPage<ChatMessage>> { return this.request(actor, "GET", `${this.channel(channelId)}/messages${this.page(page, page.after)}`, undefined, options); }
  getMessage(actor: string, messageId: string, options: CallOptions = {}): Promise<ChatMessage> { return this.request(actor, "GET", `/messages/${this.id(messageId)}`, undefined, options); }
  editMessage(actor: string, messageId: string, input: ChatEditMessageInput, options: CallOptions = {}): Promise<ChatMessage> { return this.request(actor, "PATCH", `/messages/${this.id(messageId)}`, input, options); }
  deleteMessage(actor: string, messageId: string, options: CallOptions = {}): Promise<void> { return this.request(actor, "DELETE", `/messages/${this.id(messageId)}`, undefined, options); }
  markRead(actor: string, channelId: string, messageId: string, options: CallOptions = {}): Promise<ChatReadState> { return this.request(actor, "POST", `${this.channel(channelId)}/read`, { message_id: messageId }, options); }
  unread(actor: string, channelId: string, options: CallOptions = {}): Promise<ChatUnreadSummary> { return this.request(actor, "GET", `${this.channel(channelId)}/unread`, undefined, options); }
  typing(actor: string, channelId: string, options: CallOptions = {}): Promise<void> { return this.request(actor, "POST", `${this.channel(channelId)}/typing`, undefined, options); }
  presence(actor: string, channelId: string, options: CallOptions = {}): Promise<ChatPresence> { return this.request(actor, "GET", `${this.channel(channelId)}/presence`, undefined, options); }
  createRealtimeToken(actor: string, options: CallOptions = {}): Promise<ChatRealtimeCredentials> { return this.request(actor, "POST", "/realtime/token", undefined, options); }
  async createSubscriptionTokens(actor: string, channelIds: string[], options: CallOptions = {}): Promise<ChatSubscription[]> {
    const data = await this.request<{ subscriptions: ChatSubscription[] }>(actor, "POST", "/realtime/subscriptions", { channel_ids: channelIds }, options);
    if (!Array.isArray(data.subscriptions)) throw invalidResponse("chat.subscriptions");
    return data.subscriptions;
  }
  prepareUpload(actor: string, input: ChatPrepareUploadInput, options: CallOptions = {}): Promise<ChatPreparedUpload> { return this.request(actor, "POST", "/attachments/upload", { file_name: input.fileName, mime_type: input.mimeType, size_bytes: input.sizeBytes }, options); }
  completeUpload(actor: string, attachmentId: string, options: CallOptions = {}): Promise<ChatAttachment> { return this.request(actor, "POST", `/attachments/${this.id(attachmentId)}/complete`, undefined, options); }
  getAttachmentDownload(actor: string, attachmentId: string, options: CallOptions = {}): Promise<ChatDownload> { return this.request(actor, "GET", `/attachments/${this.id(attachmentId)}`, undefined, options); }
}
