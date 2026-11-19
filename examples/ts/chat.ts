import { Relyvo } from '@relyvolabs/server-sdk';

// Authenticate the application user before calling this function. Use the
// internal Chat user.id returned by your authorized lookup/upsert as actor.
export async function createChatChannel(sdk: Relyvo, actorId: string, name: string) {
  return sdk.chat.createChannel(actorId, { type: 'group', name });
}
