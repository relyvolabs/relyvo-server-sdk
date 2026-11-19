import { Relyvo } from '@relyvolabs/server-sdk';

// Run this function only after authenticating and authorizing the participant.
export async function issueCallToken(sdk: Relyvo, roomName: string, participantIdentity: string) {
  return sdk.call.createToken({ roomName, participantIdentity, mode: 'video' });
}
