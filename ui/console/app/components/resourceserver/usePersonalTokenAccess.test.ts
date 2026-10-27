import { create } from '@bufbuild/protobuf';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { ref } from 'vue';

import { ClientSecretSchema, SecretGrace } from '#gen/client/v1/client_pb';
import { ResourceServerSchema, ResourceServerState } from '#gen/resourceserver/v1/resourceserver_pb';
import { connectError, setClient, setKnownKeys } from '#test/nuxt-globals';
import { introspectionEndpoint, usePersonalTokenAccess } from './usePersonalTokenAccess';

const projectId = 'x6MH9WPWjktMwCBKHy4pF';
const resourceServerId = 'dPm7fmQRJKPVw8PxrgdrN';
const ids = { projectId, resourceServerId };

describe('introspectionEndpoint', () => {
  it('hangs /oauth2/introspect under the issuer', () => {
    expect(introspectionEndpoint('https://id.acme.test/p/acme/')).toBe('https://id.acme.test/p/acme/oauth2/introspect');
    expect(introspectionEndpoint('')).toBe('');
  });
});

describe('usePersonalTokenAccess', () => {
  let client: Record<string, ReturnType<typeof vi.fn>>;
  let rs = create(ResourceServerSchema, { id: resourceServerId, trusted: true });
  const withSecret = create(ResourceServerSchema, { id: resourceServerId, trusted: true, introspectionSecrets: [create(ClientSecretSchema, { id: 's1', hint: 'ab' })] });

  beforeEach(() => {
    rs = create(ResourceServerSchema, { id: resourceServerId, trusted: true });
    client = {
      setPersonalTokenAccess: vi.fn(async (req: { accept: boolean }) => ({ resourceServer: create(ResourceServerSchema, { ...withSecret, acceptPersonalTokens: req.accept }) })),
      generateIntrospectionSecret: vi.fn().mockResolvedValue({ resourceServer: withSecret, secret: 'shh' }),
      rotateIntrospectionSecret: vi.fn().mockResolvedValue({ resourceServer: withSecret, secret: 'new' }),
      revokeIntrospectionSecret: vi.fn().mockResolvedValue({ resourceServer: rs }),
    };
    setClient('$resourceServerClient', client);
  });

  it('locks accepting until a secret exists, then accepts', async () => {
    const server = ref(rs);
    const a = usePersonalTokenAccess(projectId, resourceServerId, () => server.value, (next) => {
      server.value = next;
    });
    expect(a.locked.value).toBe(true);
    await a.setAccept(true);
    expect(client.setPersonalTokenAccess).not.toHaveBeenCalled();
    await a.secrets.generate();
    expect(client.generateIntrospectionSecret).toHaveBeenCalledWith(ids);
    expect(a.secrets.revealed.value).toEqual({ ownerId: resourceServerId, secret: 'shh' });
    expect(a.locked.value).toBe(false);
    await a.setAccept(true);
    expect(client.setPersonalTokenAccess).toHaveBeenCalledWith({ ...ids, accept: true });
    expect(server.value.acceptPersonalTokens).toBe(true);
  });

  it('rotates and revokes through the resource server calls, and shows RSV012', async () => {
    setKnownKeys('apperror.RSV012');
    rs = withSecret;
    const a = usePersonalTokenAccess(projectId, resourceServerId, () => rs, (next) => {
      rs = next;
    });
    a.secrets.askRotate();
    a.secrets.grace.value = SecretGrace.NOW;
    await a.secrets.confirmRotate();
    expect(client.rotateIntrospectionSecret).toHaveBeenCalledWith({ ...ids, grace: SecretGrace.NOW });
    a.secrets.askRevoke({ id: 's1', hint: 'ab' });
    await a.secrets.confirmRevoke();
    expect(client.revokeIntrospectionSecret).toHaveBeenCalledWith({ ...ids, secretId: 's1' });
    client.setPersonalTokenAccess!.mockRejectedValueOnce(connectError('RSV012'));
    rs = withSecret;
    await a.setAccept(true);
    expect(a.error.value?.message).toBe('apperror.RSV012');
    client.setPersonalTokenAccess!.mockResolvedValueOnce({});
    await a.setAccept(false);
    expect(a.busy.value).toBe(false);
  });

  it('locks accepting until the server is verified, then until it has a secret', () => {
    const server = ref(create(ResourceServerSchema, { id: resourceServerId, state: ResourceServerState.PENDING }));
    const a = usePersonalTokenAccess(projectId, resourceServerId, () => server.value, () => {});
    expect(a.lockReason.value).toBe('unverified');
    server.value = create(ResourceServerSchema, { id: resourceServerId, state: ResourceServerState.VERIFIED });
    expect(a.lockReason.value).toBe('no_secret');
    server.value = create(ResourceServerSchema, { id: resourceServerId, state: ResourceServerState.PENDING, acceptPersonalTokens: true });
    expect(a.locked.value, 'already on: it can always be turned off').toBe(false);
  });
});
