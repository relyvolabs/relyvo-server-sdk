/**
 * The smallest useful Relyvo server integration: resolve the project, mint a video
 * token, and mint a chat token for the same person.
 *
 * Run it with a real key (build the SDK first):
 *
 *   RELYVO_API_KEY=rv_… RELYVO_API_URL=https://relyvo.example.com/api \
 *   RELYVO_CHAT_URL=https://chat-api.example.com pnpm start
 *
 * Tokens are masked by default. Pass --show-token only for a local hand-off.
 */
import { Relyvo, isRelyvoError } from "@relyvolabs/server-sdk";

const showToken = process.argv.includes("--show-token");

function present(token: string): string {
  if (showToken) {
    return token;
  }
  return token.length <= 12 ? "…" : `${token.slice(0, 12)}…`;
}

function describe(error: unknown): string {
  if (isRelyvoError(error)) {
    const suffix = error.requestId === undefined ? "" : `, request ${error.requestId}`;
    return `${error.code} (status ${error.status}${suffix})`;
  }
  return error instanceof Error ? error.message : String(error);
}

async function main(): Promise<void> {
  const apiKey = process.env.RELYVO_API_KEY?.trim() ?? "";
  if (apiKey === "") {
    throw new Error("RELYVO_API_KEY is required");
  }

  const relyvo = new Relyvo({
    apiKey,
    apiUrl: process.env.RELYVO_API_URL?.trim() ?? "http://localhost:8080",
    chatUrl: process.env.RELYVO_CHAT_URL?.trim(),
  });

  const roomName = process.env.RELYVO_ROOM ?? "support-42";
  const participantIdentity = process.env.RELYVO_IDENTITY ?? "user_7";
  const displayName = process.env.RELYVO_NAME ?? "Ada";

  const info = await relyvo.whoami();
  console.log(`project   ${info.projectId} (${info.environment})`);

  try {
    const media = await relyvo.video.config();
    console.log(`media     ${media.serverUrl} (token ttl ${media.tokenTtlSeconds}s)`);
  } catch (error) {
    console.log(`media     unavailable: ${describe(error)}`);
  }

  const video = await relyvo.video.createToken({
    roomName,
    participantIdentity,
    displayName,
    mode: "video",
  });

  console.log("video", {
    room: video.roomName,
    identity: video.participantIdentity,
    mode: video.mode,
    expiresAt: video.expiresAt.toISOString(),
    serverUrl: video.serverUrl,
    meterCode: video.meterCode,
    token: present(video.token),
  });

  if ((process.env.RELYVO_CHAT_URL ?? "").trim() === "") {
    console.log("chat      skipped: RELYVO_CHAT_URL is not set");
    return;
  }

  const { user, created } = await relyvo.chat.upsertUser({
    externalUserId: participantIdentity,
    displayName,
  });
  console.log(`chat user ${user.id} (created=${created})`);

  const token = await relyvo.chat.createUserToken(user.id);
  console.log("chat", {
    userId: user.id,
    scope: token.scope,
    expiresAt: token.expiresAt.toISOString(),
    token: present(token.token),
  });
}

main().catch((error: unknown) => {
  console.error(`error: ${describe(error)}`);
  process.exitCode = 1;
});
