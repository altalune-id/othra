import { create } from '@bufbuild/protobuf';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import { CIMDAdmissionSchema, LinkSchema, ResourceServerSchema, ResourceServerState } from '#gen/resourceserver/v1/resourceserver_pb';
import { connectError, deferred, setClient, setKnownKeys } from '#test/nuxt-globals';
import { admissible, isCimdLink, parseHosts, useCimd } from './useCimd';

const projectId = 'x6MH9WPWjktMwCBKHy4pF';
const rsId = 'Rs2n2Xq8RtY5uVw3zA9bC';
const rs = create(ResourceServerSchema, {
  id: rsId,
  scopes: ['messages:read', 'messages:write'],
  state: ResourceServerState.VERIFIED,
  cimd: create(CIMDAdmissionSchema, { accept: true, hosts: ['claude.ai'], allScopes: false, scopes: ['messages:read'] }),
});
const shadow = create(LinkSchema, { clientId: 'Cc2n2Xq8RtY5uVw3zA9bC', clientName: 'Claude Code', clientMetadataUrl: 'https://claude.ai/oauth/claude-code-client-metadata' });

describe('cimd helpers', () => {
  it('admits only a verified or trusted server', () => {
    expect(admissible({ state: ResourceServerState.VERIFIED, trusted: false })).toBe(true);
    expect(admissible({ state: ResourceServerState.PENDING, trusted: true })).toBe(true);
    expect(admissible({ state: ResourceServerState.PENDING, trusted: false })).toBe(false);
    expect(admissible({ state: ResourceServerState.FAILED, trusted: false })).toBe(false);
  });

  it('reads one host per line, lower-cased, without blanks or repeats', () => {
    expect(parseHosts(' Claude.AI \n\nclaude.ai\nexample.com \n')).toEqual(['claude.ai', 'example.com']);
    expect(parseHosts('')).toEqual([]);
  });

  it('tells a CIMD app link from a registered one', () => {
    expect(isCimdLink(shadow)).toBe(true);
    expect(isCimdLink(create(LinkSchema, { clientId: 'x' }))).toBe(false);
  });
});

describe('useCimd', () => {
  let setResourceServerAdmission: ReturnType<typeof vi.fn>;
  let deleteClient: ReturnType<typeof vi.fn>;
  let onServer: ReturnType<typeof vi.fn<(rs: unknown) => void>>;
  let onRevoked: ReturnType<typeof vi.fn<(id: string) => void>>;

  beforeEach(() => {
    setResourceServerAdmission = vi.fn(async (req: { accept: boolean, hosts: string[], allScopes: boolean, scopes: string[] }) => ({
      resourceServer: { ...rs, cimd: create(CIMDAdmissionSchema, { accept: req.accept, hosts: req.hosts, allScopes: req.allScopes, scopes: req.scopes }) },
    }));
    deleteClient = vi.fn().mockResolvedValue({});
    onServer = vi.fn();
    onRevoked = vi.fn();
    setClient('$resourceServerClient', { setResourceServerAdmission });
    setClient('$clientClient', { deleteClient });
  });

  it('starts from the server and saves the admission', async () => {
    const c = useCimd(projectId, rsId, onServer, onRevoked);
    c.reset(rs);
    expect([c.accept.value, c.hosts.value, c.allScopes.value, c.picked.value, c.catalog.value]).toEqual([true, 'claude.ai', false, ['messages:read'], ['messages:read', 'messages:write']]);
    c.hosts.value = 'claude.ai\nExample.com';
    c.picked.value = ['messages:write', 'gone:scope', 'messages:read'];
    await c.save();
    expect(setResourceServerAdmission).toHaveBeenCalledWith({
      projectId,
      resourceServerId: rsId,
      accept: true,
      hosts: ['claude.ai', 'example.com'],
      allScopes: false,
      scopes: ['messages:read', 'messages:write'],
    });
    expect(onServer).toHaveBeenCalledTimes(1);
    expect(c.saved.value).toBe(true);
    expect(c.hosts.value).toBe('claude.ai\nexample.com');
  });

  it('sends no pinned scopes with all scopes, and turns admission off', async () => {
    const c = useCimd(projectId, rsId, onServer, onRevoked);
    c.reset(rs);
    c.allScopes.value = true;
    c.accept.value = false;
    await c.save();
    expect(setResourceServerAdmission.mock.calls[0]![0]).toMatchObject({ accept: false, allScopes: true, scopes: [] });
  });

  it('starts off for a server without admission', () => {
    const c = useCimd(projectId, rsId, onServer, onRevoked);
    c.reset(create(ResourceServerSchema, { scopes: ['a'] }));
    expect([c.accept.value, c.hosts.value, c.allScopes.value, c.picked.value]).toEqual([false, '', true, []]);
  });

  it('refuses "only these" with nothing picked before sending', async () => {
    const c = useCimd(projectId, rsId, onServer, onRevoked);
    c.reset(rs);
    c.picked.value = [];
    await c.save();
    expect(setResourceServerAdmission).not.toHaveBeenCalled();
    expect(c.error.value?.message).toBe('apperror.RSV009_empty');
  });

  it('puts a bad host under the hosts field and other refusals on the card', async () => {
    setKnownKeys('apperror.RSV010', 'apperror.RSV011');
    setResourceServerAdmission.mockRejectedValueOnce(connectError('RSV010', { host: '10.0.0.1', reason: 'ip_literal' })).mockRejectedValueOnce(connectError('RSV011'));
    const c = useCimd(projectId, rsId, onServer, onRevoked);
    c.reset(rs);
    await c.save();
    expect(c.hostsError.value).toMatch(/^apperror\.RSV010/);
    expect(c.error.value).toBeNull();
    await c.save();
    expect(c.hostsError.value).toBe('');
    expect(c.error.value?.message).toBe('apperror.RSV011');
    expect(c.saved.value).toBe(false);
  });

  it('checks the request against its proto rules before sending', async () => {
    const c = useCimd(projectId, rsId, onServer, onRevoked);
    c.reset(rs);
    c.hosts.value = Array.from({ length: 65 }, (_, i) => `h${i}.example`).join('\n');
    await c.save();
    expect(setResourceServerAdmission).not.toHaveBeenCalled();
    expect(c.hostsError.value).toBe('validation.too_many|{"max":"64"}');
  });

  it('ignores a second save while one runs, and a reply without a server', async () => {
    const pending = deferred<object>();
    setResourceServerAdmission.mockReturnValue(pending.promise);
    const c = useCimd(projectId, rsId, onServer, onRevoked);
    c.reset(rs);
    const first = c.save();
    await c.save();
    pending.resolve({});
    await first;
    expect(setResourceServerAdmission).toHaveBeenCalledTimes(1);
    expect(onServer).not.toHaveBeenCalled();
  });

  it('revokes a CIMD app after a confirm, and keeps the confirm open on a refusal', async () => {
    setKnownKeys('apperror.GEN900');
    deleteClient.mockRejectedValueOnce(connectError('GEN900'));
    const c = useCimd(projectId, rsId, onServer, onRevoked);
    await c.confirmRevoke();
    c.askRevoke(shadow);
    await c.confirmRevoke();
    expect(c.revokeError.value?.message).toBe('apperror.GEN900');
    expect(c.revoking.value).not.toBeNull();
    await c.confirmRevoke();
    expect(deleteClient).toHaveBeenCalledWith({ projectId, clientId: shadow.clientId });
    expect(onRevoked).toHaveBeenCalledWith(shadow.clientId);
    expect(c.revoking.value).toBeNull();
    c.askRevoke(shadow);
    c.cancelRevoke();
    expect(c.revoking.value).toBeNull();
  });

  it('ignores a second revoke and a cancel while one runs', async () => {
    const pending = deferred<object>();
    deleteClient.mockReturnValue(pending.promise);
    const c = useCimd(projectId, rsId, onServer, onRevoked);
    c.askRevoke(shadow);
    const first = c.confirmRevoke();
    await c.confirmRevoke();
    c.cancelRevoke();
    expect(c.revoking.value).not.toBeNull();
    pending.resolve({});
    await first;
    expect(deleteClient).toHaveBeenCalledTimes(1);
  });
});
