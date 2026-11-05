/**
 * Official server SDK for the Relyvo platform.
 *
 * Exchange an API key for the short-lived credentials a browser needs: a LiveKit
 * participant token and a chat end-user token. The key is a server credential and
 * is never sent to a client.
 *
 * @example
 * ```ts
 * const relyvo = new Relyvo({ apiKey: process.env.RELYVO_API_KEY!, apiUrl: "https://relyvo.example.com/api" });
 * const video = await relyvo.video.createToken({ roomName: "support-42", participantIdentity: "user_7" });
 * ```
 */

export { Relyvo, SDK_VERSION } from "./client.js";
export {
  RelyvoConfigError,
  RelyvoError,
  isRelyvoConfigError,
  isRelyvoError,
} from "./errors.js";

export type {
  APIKeyInfo,
  CallOptions,
  ChatUser,
  ChatUserInput,
  ChatUserResult,
  ChatUserToken,
  RelyvoConfig,
  VideoConfig,
  VideoMode,
  VideoToken,
  VideoTokenInput,
} from "./types.js";
