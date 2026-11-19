import { describe, expect, it } from "vitest";
import { Relyvo } from "../src/index.js";
import { jsonResponse, mockFetch, sentBody } from "./helpers.js";

const setup = (fetch: typeof globalThis.fetch) => new Relyvo({ apiKey: "rv_key_secret", apiUrl: "https://api.test", chatUrl: "https://chat.test/", fetch, maxRetries: 2, baseDelayMs: 0 });
const at = "2026-01-01T00:00:00Z";
const channel = { id: "c", organization_id: "o", project_id: "p", type: "group", name: "group", description: "", created_by: "u", metadata: null, last_seq: 0, max_members: 50, sending_enabled: true, created_at: at, updated_at: at };
const member = { id: "member", organization_id: "o", project_id: "p", channel_id: "c", user_id: "u", role: "member", joined_at: at, created_at: at, updated_at: at };
const message = { id: "m", organization_id: "o", project_id: "p", channel_id: "c", sender_id: "u", channel_seq: 1, type: "text", content: "hi", metadata: null, created_at: at };
const attachment = { id: "a", organization_id: "o", project_id: "p", uploader_id: "u", storage_provider: "s3", file_name: "a.png", mime_type: "image/png", size_bytes: 4, status: "pending", created_at: at, updated_at: at };
function responseFor(url: string, method: string | undefined): Response {
  const path = new URL(url).pathname.replace("/v1/chat", "");
  let data: unknown;
  if (path === "/channels" && method === "POST") data = { channel, created: true };
  else if ((path === "/channels" || path.endsWith("/members") || path.endsWith("/messages")) && method === "GET") data = { items: [], has_more: false };
  else if (path.endsWith("/members")) data = { member, added: true };
  else if (path.endsWith("/messages")) data = { message, created: true };
  else if (path.startsWith("/messages/")) data = message;
  else if (path.endsWith("/read")) data = { organization_id: "o", project_id: "p", channel_id: "c", user_id: "u", last_read_seq: 1, last_read_at: at, updated_at: at };
  else if (path.endsWith("/unread")) data = { channel_id: "c", last_seq: 1, last_read_seq: 0, unread_count: 1, has_unread: true };
  else if (path.endsWith("/presence")) data = { channel_id: "c", presence: [] };
  else if (path === "/realtime/token") data = { token: "jwt", ws_url: "wss://chat", expires_at: at, ttl_seconds: 60, channel_prefix: "project" };
  else if (path === "/realtime/subscriptions") data = { subscriptions: [] };
  else if (path === "/attachments/upload") data = { attachment, upload_url: "https://upload", headers: {}, expires_at: at };
  else if (path.endsWith("/complete")) data = attachment;
  else if (path.startsWith("/attachments/")) data = { attachment, download_url: "https://download" };
  else data = channel;
  return jsonResponse({ data });
}

describe("Chat user-scoped project credentials", () => {
  it("sends explicit actor alongside the project key and maps all actual routes", async () => {
    const mock = mockFetch((call) => call.init.method === "DELETE" || call.url.endsWith("/typing") ? new Response(null, { status: call.init.method === "DELETE" ? 204 : 202 }) : responseFor(call.url, call.init.method));
    const chat = setup(mock.fetch).chat;
    const actor = "11111111-1111-4111-8111-111111111111";
    await chat.createChannel(actor, { type: "group", name: "team", memberIds: ["u"], otherUserId: "peer" });
    await chat.listChannels(actor, { limit: 5, cursor: "a&b" });
    await chat.getChannel(actor, "c/a");
    await chat.updateChannel(actor, "c", { name: "new" });
    await chat.deleteChannel(actor, "c");
    await chat.addMember(actor, "c", "u", "admin");
    await chat.listMembers(actor, "c");
    await chat.removeMember(actor, "c", "u");
    await chat.sendMessage(actor, "c", { clientMessageId: "key", content: "hello", replyToMessageId: "m", attachmentIds: ["a"], metadata: { raw_key: 1 } });
    await chat.listMessages(actor, "c", { after: true, cursor: "opaque+", limit: 2 });
    await chat.getMessage(actor, "m");
    await chat.editMessage(actor, "m", { content: "edit" });
    await chat.deleteMessage(actor, "m");
    await chat.markRead(actor, "c", "m");
    await chat.unread(actor, "c");
    await chat.typing(actor, "c");
    await chat.presence(actor, "c");
    await chat.createRealtimeToken(actor);
    await chat.createSubscriptionTokens(actor, ["c"]);
    await chat.prepareUpload(actor, { fileName: "a.png", mimeType: "image/png", sizeBytes: 4 });
    await chat.completeUpload(actor, "a");
    await chat.getAttachmentDownload(actor, "a");
    expect(mock.calls.map(({ url, init }) => `${init.method} ${url.replace("https://chat.test/v1/chat", "")}`)).toEqual([
      "POST /channels", "GET /channels?limit=5&cursor=a%26b", "GET /channels/c%2Fa", "PATCH /channels/c", "DELETE /channels/c", "POST /channels/c/members", "GET /channels/c/members", "DELETE /channels/c/members/u", "POST /channels/c/messages", "GET /channels/c/messages?limit=2&cursor=opaque%2B&direction=after", "GET /messages/m", "PATCH /messages/m", "DELETE /messages/m", "POST /channels/c/read", "GET /channels/c/unread", "POST /channels/c/typing", "GET /channels/c/presence", "POST /realtime/token", "POST /realtime/subscriptions", "POST /attachments/upload", "POST /attachments/a/complete", "GET /attachments/a",
    ]);
    for (let i = 0; i < mock.calls.length; i++) {
      expect(mock.headers(i).Authorization).toBe("Bearer rv_key_secret");
      expect(mock.headers(i)["X-Relyvo-Chat-User-ID"]).toBe(actor);
    }
    expect(sentBody(mock.calls[0])).toEqual({ type: "group", name: "team", member_ids: ["u"], other_user_id: "peer" });
    expect(sentBody(mock.calls[5])).toEqual({ user_id: "u", role: "admin" });
    expect(sentBody(mock.calls[8])).toEqual({ client_message_id: "key", content: "hello", reply_to_message_id: "m", attachment_ids: ["a"], metadata: { raw_key: 1 } });
    expect(sentBody(mock.calls[13])).toEqual({ message_id: "m" });
    expect(sentBody(mock.calls[18])).toEqual({ channel_ids: ["c"] });
    expect(sentBody(mock.calls[19])).toEqual({ file_name: "a.png", mime_type: "image/png", size_bytes: 4 });
  });
  it("retains actual response data and pagination rather than wrapping singleton resources", async () => {
    const data = [channel, { items: [message], has_more: true, next_cursor: "next" }, { message, created: false }, { token: "jwt", ws_url: "wss://chat", expires_at: at, ttl_seconds: 60, channel_prefix: "project" }, { attachment, upload_url: "https://upload", headers: { "Content-Type": "image/png" }, expires_at: at }];
    const mock = mockFetch((_call, index) => jsonResponse({ data: data[index], request_id: "req" }));
    const chat = setup(mock.fetch).chat;
    expect(await chat.getChannel("11111111-1111-4111-8111-111111111111", "c")).toEqual(data[0]);
    expect(await chat.listMessages("11111111-1111-4111-8111-111111111111", "c")).toEqual(data[1]);
    expect(await chat.sendMessage("11111111-1111-4111-8111-111111111111", "c", { content: "x" })).toEqual(data[2]);
    expect(await chat.createRealtimeToken("11111111-1111-4111-8111-111111111111")).toEqual(data[3]);
    expect(await chat.prepareUpload("11111111-1111-4111-8111-111111111111", { fileName: "a", mimeType: "image/png", sizeBytes: 1 })).toEqual(data[4]);
  });
  it("retries only GET, never writes even with an idempotency key", async () => {
    const mock = mockFetch(() => jsonResponse({ error: { code: "unavailable", message: "down" } }, 503));
    const chat = setup(mock.fetch).chat;
    await expect(chat.getChannel("11111111-1111-4111-8111-111111111111", "c")).rejects.toMatchObject({ status: 503 });
    expect(mock.calls).toHaveLength(3);
    await expect(chat.sendMessage("11111111-1111-4111-8111-111111111111", "c", { clientMessageId: "key", content: "x" })).rejects.toMatchObject({ status: 503 });
    await expect(chat.createSubscriptionTokens("11111111-1111-4111-8111-111111111111", ["c"])).rejects.toMatchObject({ status: 503 });
    await expect(chat.deleteChannel("11111111-1111-4111-8111-111111111111", "c")).rejects.toMatchObject({ status: 503 });
    expect(mock.calls).toHaveLength(6);
  });
  it("rejects malformed endpoint-specific response bodies", async () => {
    const mock = mockFetch(() => jsonResponse({ data: { items: [{}], has_more: true } }));
    await expect(setup(mock.fetch).chat.listMessages("11111111-1111-4111-8111-111111111111", "c")).rejects.toMatchObject({ code: "invalid_response" });
  });
  it("requires an internal UUID actor and propagates cancellation", async () => {
    const mock = mockFetch(({ init }) => new Promise((_resolve, reject) => init.signal?.addEventListener("abort", () => reject(new Error("aborted")), { once: true })));
    const chat = setup(mock.fetch).chat;
    await expect(chat.getChannel(" ", "c")).rejects.toThrow("internal chat user UUID");
    const controller = new AbortController();
    const pending = chat.sendMessage("11111111-1111-4111-8111-111111111111", "c", { content: "x" }, { signal: controller.signal });
    controller.abort();
    await expect(pending).rejects.toMatchObject({ code: "aborted" });
    expect(mock.calls).toHaveLength(1);
  });
});
