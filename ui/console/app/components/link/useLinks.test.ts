import { create } from '@bufbuild/protobuf';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import { ClientRegistration, ClientSchema } from '#gen/client/v1/client_pb';
import { LinkSchema, ResourceServerSchema, ResourceServerState } from '#gen/resourceserver/v1/resourceserver_pb';
import { connectError, deferred, setClient, setKnownKeys } from '#test/nuxt-globals';
import { linkUnverified, useLinks } from './useLinks';

const projectId = 'x6MH9WPWjktMwCBKHy4pF';
const clientId = 'dPm7fmQRJKPVw8PxrgdrN';
const apiId = 'Rs2n2Xq8RtY5uVw3zA9bC';
const mcpId = 'Ms3p4Zs2TvA7wXy5bC2dE';
const api = create(ResourceServerSchema, { id: apiId, name: 'Store API', identifier: 'https://store.example/api', scopes: ['orders:read', 'orders:write'], state: ResourceServerState.VERIFIED });
const mcp = create(ResourceServerSchema, { id: mcpId, name: 'Store MCP', identifier: 'https://store.example/mcp', scopes: ['mcp:tools'], state: ResourceServerState.PENDING });
const linked = create(LinkSchema, { clientId, clientName: 'Store web', resourceServerId: apiId, resourceServerName: 'Store API', scopes: ['orders:read'] });

describe('linkUnverified', () => {
  it('flags a server that is neither verified nor trusted', () => {
    expect(linkUnverified(ResourceServerState.PENDING, false)).toBe(true);
    expect(linkUnverified(ResourceServerState.FAILED, false)).toBe(true);
    expect(linkUnverified(ResourceServerState.PENDING, true)).toBe(false);
    expect(linkUnverified(ResourceServerState.VERIFIED, false)).toBe(false);
  });
});

describe('useLinks on a client', () => {
  let rs: Record<string, ReturnType<typeof vi.fn>>;

  beforeEach(() => {
    rs = {
      listLinks: vi.fn().mockResolvedValue({ links: [linked] }),
      listResourceServers: vi.fn().mockResolvedValue({ resourceServers: [api, mcp] }),
      linkClient: vi.fn(async (req: { resourceServerId: string, scopes: string[] }) => ({ link: create(LinkSchema, { clientId, resourceServerId: req.resourceServerId, scopes: req.scopes }) })),
      updateLink: vi.fn(async (req: { resourceServerId: string, scopes: string[] }) => ({ link: create(LinkSchema, { clientId, resourceServerId: req.resourceServerId, scopes: req.scopes }) })),
      unlinkClient: vi.fn().mockResolvedValue({}),
    };
    setClient('$resourceServerClient', rs);
    setClient('$clientClient', {});
  });

  async function loaded() {
    const l = useLinks(projectId, { side: 'client', id: clientId });
    await l.load();
    return l;
  }

  it('loads the links and offers only the servers not linked yet', async () => {
    const l = await loaded();
    expect(rs.listLinks).toHaveBeenCalledWith({ projectId, target: { case: 'clientId', value: clientId } });
    expect(l.links.value).toHaveLength(1);
    expect(l.available.value.map(x => [x.id, x.unverified])).toEqual([[mcpId, true]]);
    expect(l.loading.value).toBe(false);
  });

  it('shows a load error', async () => {
    setKnownKeys('apperror.GEN900');
    rs.listLinks!.mockRejectedValue(connectError('GEN900'));
    const l = await loaded();
    expect(l.loadError.value?.message).toBe('apperror.GEN900');
  });

  it('links a new server with the picked scopes in its order', async () => {
    const l = await loaded();
    l.askLink();
    expect([l.editing.value, l.targetId.value, l.scopeOptions.value]).toEqual(['new', mcpId, ['mcp:tools']]);
    await l.confirmEdit();
    expect(l.pickError.value).toBe(true);
    expect(rs.linkClient).not.toHaveBeenCalled();
    l.pickTarget(apiId);
    l.picked.value = ['orders:write', 'orders:read', 'stale'];
    await l.confirmEdit();
    expect(rs.linkClient).toHaveBeenCalledWith({ projectId, clientId, resourceServerId: apiId, scopes: ['orders:read', 'orders:write'] });
    expect(l.editing.value).toBe('');
    expect(l.links.value.map(x => x.scopes)).toEqual([['orders:read', 'orders:write']]);
  });

  it('edits a link with its pinned scopes checked', async () => {
    const l = await loaded();
    l.askEdit(l.links.value[0]!);
    expect([l.editing.value, l.targetId.value, l.picked.value]).toEqual(['edit', apiId, ['orders:read']]);
    l.picked.value = ['orders:write'];
    await l.confirmEdit();
    expect(rs.updateLink).toHaveBeenCalledWith({ projectId, clientId, resourceServerId: apiId, scopes: ['orders:write'] });
  });

  it('keeps the dialog open with the reason, and closes it on cancel', async () => {
    setKnownKeys('apperror.RSV009_not_in_catalog');
    rs.updateLink!.mockRejectedValue(connectError('RSV009', { reason: 'not_in_catalog', scope: 'orders:read' }));
    const l = await loaded();
    await l.confirmEdit();
    l.askEdit(l.links.value[0]!);
    await l.confirmEdit();
    expect(l.error.value?.message).toMatch(/^apperror\.RSV009_not_in_catalog/);
    expect(l.editing.value).toBe('edit');
    l.cancelEdit();
    expect([l.editing.value, l.error.value]).toEqual(['', null]);
  });

  it('unlinks after a confirm, and keeps the confirm open on a refusal', async () => {
    setKnownKeys('apperror.GEN900');
    rs.unlinkClient!.mockRejectedValueOnce(connectError('GEN900'));
    const l = await loaded();
    await l.confirmUnlink();
    l.askUnlink(l.links.value[0]!);
    await l.confirmUnlink();
    expect(l.error.value?.message).toBe('apperror.GEN900');
    await l.confirmUnlink();
    expect(rs.unlinkClient).toHaveBeenCalledWith({ projectId, clientId, resourceServerId: apiId });
    expect([l.links.value, l.unlinking.value]).toEqual([[], null]);
    l.askUnlink(linked);
    l.cancelUnlink();
    expect(l.unlinking.value).toBeNull();
  });

  it('ignores actions and cancels while one runs, and a reply without a link', async () => {
    const pending = deferred<object>();
    rs.linkClient!.mockReturnValue(pending.promise);
    const l = await loaded();
    l.askLink();
    l.picked.value = ['mcp:tools'];
    const first = l.confirmEdit();
    await l.confirmEdit();
    l.cancelEdit();
    l.askUnlink(linked);
    await l.confirmUnlink();
    l.cancelUnlink();
    expect(l.unlinking.value).not.toBeNull();
    pending.resolve({});
    await first;
    expect(rs.linkClient).toHaveBeenCalledTimes(1);
    expect(rs.unlinkClient).not.toHaveBeenCalled();
    expect(l.links.value).toHaveLength(1);
  });

  it('offers nothing and refuses to link when every server is linked', async () => {
    rs.listResourceServers!.mockResolvedValue({ resourceServers: [api] });
    const l = await loaded();
    l.askLink();
    expect([l.targetId.value, l.target.value, l.scopeOptions.value]).toEqual(['', null, []]);
    await l.confirmEdit();
    expect(rs.linkClient).not.toHaveBeenCalled();
  });
});

describe('useLinks on a resource server', () => {
  it('lists clients as targets and pins the server\'s own scopes', async () => {
    const listClients = vi.fn().mockResolvedValue({ clients: [create(ClientSchema, { id: clientId, name: 'Store web' }), create(ClientSchema, { id: 'H9MspUj32ux8pfYKYsu2a', name: 'Store CLI' })] });
    const linkClient = vi.fn().mockResolvedValue({ link: linked });
    setClient('$clientClient', { listClients });
    setClient('$resourceServerClient', { listLinks: vi.fn().mockResolvedValue({ links: [linked] }), linkClient });
    const l = useLinks(projectId, { side: 'resourceServer', id: apiId, scopes: ['orders:read'] });
    await l.load();
    expect(l.available.value.map(x => x.name)).toEqual(['Store CLI']);
    expect(l.scopeOptions.value).toEqual(['orders:read']);
    l.setOwnerScopes(['orders:read', 'orders:write']);
    l.askLink();
    expect(l.scopeOptions.value).toEqual(['orders:read', 'orders:write']);
    l.picked.value = ['orders:write'];
    await l.confirmEdit();
    expect(linkClient).toHaveBeenCalledWith({ projectId, clientId: 'H9MspUj32ux8pfYKYsu2a', resourceServerId: apiId, scopes: ['orders:write'] });
  });

  it('has no scopes to offer before the server is known', async () => {
    setClient('$clientClient', { listClients: vi.fn().mockResolvedValue({ clients: [] }) });
    setClient('$resourceServerClient', { listLinks: vi.fn().mockResolvedValue({ links: [] }) });
    const l = useLinks(projectId, { side: 'resourceServer', id: apiId });
    expect(l.scopeOptions.value).toEqual([]);
  });
});

describe('useLinks and apps without registration (M6)', () => {
  it('drops a revoked app from the list and never offers a CIMD client as a link target', async () => {
    const cimd = create(ClientSchema, { id: 'Cc2n2Xq8RtY5uVw3zA9bC', name: 'Claude Code', registration: ClientRegistration.CIMD });
    const web = create(ClientSchema, { id: clientId, name: 'Store web', registration: ClientRegistration.REGISTERED });
    setClient('$clientClient', { listClients: vi.fn().mockResolvedValue({ clients: [cimd, web] }) });
    setClient('$resourceServerClient', { listLinks: vi.fn().mockResolvedValue({ links: [create(LinkSchema, { clientId: cimd.id, resourceServerId: apiId, clientMetadataUrl: 'https://claude.ai/m' })] }) });
    const l = useLinks(projectId, { side: 'resourceServer', id: apiId });
    await l.load();
    expect(l.available.value.map(x => x.name)).toEqual(['Store web']);
    l.drop(cimd.id);
    expect(l.links.value).toEqual([]);
  });
});
