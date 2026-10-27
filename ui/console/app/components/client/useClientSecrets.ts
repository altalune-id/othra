import type { Client } from '#gen/client/v1/client_pb';
import { useSecrets } from '../secret/useSecrets';

// useClientSecrets generates, rotates and revokes a web client's secrets.
export function useClientSecrets(projectId: string, clientId: string, onClient: (client: Client) => void) {
  const { $clientClient: client } = useNuxtApp();
  return useSecrets<Client>(clientId, {
    generate: async () => {
      const res = await client.generateClientSecret({ projectId, clientId });
      return { owner: res.client, secret: res.secret };
    },
    rotate: async (grace) => {
      const res = await client.rotateClientSecret({ projectId, clientId, grace });
      return { owner: res.client, secret: res.secret };
    },
    revoke: async secretId => ({ owner: (await client.revokeClientSecret({ projectId, clientId, secretId })).client }),
  }, onClient);
}
