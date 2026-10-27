import type { ShownError } from '#base/lib/apperror/message';
import type { ResourceServer } from '#gen/resourceserver/v1/resourceserver_pb';
import { useErrorMessage } from '#base/composables/useErrorMessage';
import { useSecrets } from '../secret/useSecrets';
import { admissible } from './useCimd';

// introspectionEndpoint is the RFC 7662 endpoint a resource server calls under the project's issuer.
export function introspectionEndpoint(issuerUrl: string): string {
  return issuerUrl ? `${issuerUrl.replace(/\/+$/, '')}/oauth2/introspect` : '';
}

// usePersonalTokenAccess turns personal tokens on or off for a resource server and manages the secret it introspects with.
export function usePersonalTokenAccess(projectId: string, resourceServerId: string, current: () => ResourceServer, onServer: (rs: ResourceServer) => void) {
  const { $resourceServerClient: client } = useNuxtApp();
  const errorMessage = useErrorMessage();
  const busy = ref(false);
  const error = ref<ShownError | null>(null);
  const ids = { projectId, resourceServerId };
  const lockReason = computed<'' | 'unverified' | 'no_secret'>(() => {
    const rs = current();
    if (rs.acceptPersonalTokens) {
      return '';
    }
    if (!admissible(rs)) {
      return 'unverified';
    }
    return rs.introspectionSecrets.length === 0 ? 'no_secret' : '';
  });
  const locked = computed(() => lockReason.value !== '');

  async function setAccept(accept: boolean) {
    if (busy.value || (accept && locked.value)) {
      return;
    }
    busy.value = true;
    error.value = null;
    try {
      const res = await client.setPersonalTokenAccess({ ...ids, accept });
      if (res.resourceServer) {
        onServer(res.resourceServer);
      }
    } catch (e) {
      error.value = errorMessage(e, 'error.save_rs');
    } finally {
      busy.value = false;
    }
  }

  const secrets = useSecrets<ResourceServer>(resourceServerId, {
    generate: async () => {
      const res = await client.generateIntrospectionSecret(ids);
      return { owner: res.resourceServer, secret: res.secret };
    },
    rotate: async (grace) => {
      const res = await client.rotateIntrospectionSecret({ ...ids, grace });
      return { owner: res.resourceServer, secret: res.secret };
    },
    revoke: async secretId => ({ owner: (await client.revokeIntrospectionSecret({ ...ids, secretId })).resourceServer }),
  }, onServer);

  return { busy, error, locked, lockReason, setAccept, secrets };
}
