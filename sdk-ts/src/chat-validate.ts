import { asRecord, invalidResponse } from "./decode.js";
import type { SendRequest } from "./transport.js";
type Kind = "string" | "number" | "boolean" | "array" | "object";
type Fields = Record<string, Kind>;
function validate(value: unknown, fields: Fields): Record<string, unknown> {
  const record = asRecord(value, "chat.response.data");
  for (const [key, kind] of Object.entries(fields)) {
    const v = record[key];
    if (kind === "array" ? !Array.isArray(v) : kind === "object" ? typeof v !== "object" || v === null || Array.isArray(v) : typeof v !== kind) throw invalidResponse(`chat.response.data.${key}`);
  }
  return record;
}
const channel: Fields = { id: "string", organization_id: "string", project_id: "string", type: "string", name: "string", description: "string", created_by: "string", last_seq: "number", max_members: "number", sending_enabled: "boolean", created_at: "string", updated_at: "string" };
const member: Fields = { id: "string", organization_id: "string", project_id: "string", channel_id: "string", user_id: "string", role: "string", joined_at: "string", created_at: "string", updated_at: "string" };
const message: Fields = { id: "string", organization_id: "string", project_id: "string", channel_id: "string", sender_id: "string", channel_seq: "number", type: "string", content: "string", created_at: "string" };
const attachment: Fields = { id: "string", organization_id: "string", project_id: "string", uploader_id: "string", storage_provider: "string", file_name: "string", mime_type: "string", size_bytes: "number", status: "string", created_at: "string", updated_at: "string" };

/** Validate required wire fields without rewriting metadata or timestamps. */
export function validateChatResponse(method: SendRequest["method"], path: string, data: Record<string, unknown>): void {
  if (path === "/channels" && method === "POST") { validate(data, { channel: "object", created: "boolean" }); validate(data.channel, channel); return; }
  const page = path === "/channels" ? channel : path.endsWith("/members") && method === "GET" ? member : path.endsWith("/messages") && method === "GET" ? message : undefined;
  if (page) { validate(data, { items: "array", has_more: "boolean" }); for (const item of data.items as unknown[]) validate(item, page); return; }
  if (path.endsWith("/members") && method === "POST") { validate(data, { member: "object", added: "boolean" }); validate(data.member, member); return; }
  if (path.endsWith("/messages") && method === "POST") { validate(data, { message: "object", created: "boolean" }); validate(data.message, message); return; }
  if (/^\/channels\/[^/]+$/.test(path)) { validate(data, channel); return; }
  if (/^\/messages\/[^/]+$/.test(path)) { validate(data, message); return; }
  if (path.endsWith("/read")) { validate(data, { organization_id: "string", project_id: "string", channel_id: "string", user_id: "string", last_read_seq: "number", last_read_at: "string", updated_at: "string" }); return; }
  if (path.endsWith("/unread")) { validate(data, { channel_id: "string", last_seq: "number", last_read_seq: "number", unread_count: "number", has_unread: "boolean" }); return; }
  if (path.endsWith("/presence")) { validate(data, { channel_id: "string", presence: "array" }); for (const item of data.presence as unknown[]) validate(item, { client_id: "string", user_id: "string" }); return; }
  if (path === "/realtime/token") { validate(data, { token: "string", ws_url: "string", expires_at: "string", channel_prefix: "string", ttl_seconds: "number" }); return; }
  if (path === "/realtime/subscriptions") { validate(data, { subscriptions: "array" }); for (const item of data.subscriptions as unknown[]) validate(item, { channel: "string", kind: "string", token: "string", expires_at: "string" }); return; }
  if (path === "/attachments/upload") { validate(data, { attachment: "object", upload_url: "string", expires_at: "string" }); validate(data.attachment, attachment); return; }
  if (/^\/attachments\/[^/]+\/complete$/.test(path)) { validate(data, attachment); return; }
  if (/^\/attachments\/[^/]+$/.test(path)) { validate(data, { attachment: "object", download_url: "string" }); validate(data.attachment, attachment); }
}
