import { Relyvo, toRealtimeWebCredentials } from '@relyvolabs/server-sdk';

// Authenticate the user and authorize these channels before calling this.
export async function createRealtimeCredentials(sdk: Relyvo, userId: string, channels: string[]) {
  const issued = await sdk.realtime.createClientToken({ userId, channels });
  return toRealtimeWebCredentials(issued);
}
