import { create } from '@bufbuild/protobuf';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import { ResourceServerSchema, ResourceServerState, VerificationFailureSchema } from '#gen/resourceserver/v1/resourceserver_pb';
import { connectError, deferred, setClient, setKnownKeys } from '#test/nuxt-globals';
import { useResourceServer, useResourceServerCreate, useResourceServerList, useScopeRows } from './useResourceServers';

const projectId = 'x6MH9WPWjktMwCBKHy4pF';
const rsId = 'Rs2n2Xq8RtY5uVw3zA9bC';
const rs = create(ResourceServerSchema, { id: rsId, name: 'Store MCP', identifier: 'https://store.example/mcp', scopes: ['posts:read', 'posts:write'], state: ResourceServerState.PENDING });

describe('useResourceServerList', () => {
  it('loads the servers, and shows a load error', async () => {
    setKnownKeys('apperror.GEN900');
    const listResourceServers = vi.fn().mockResolvedValueOnce({ resourceServers: [rs] }).mockRejectedValueOnce(connectError('GEN900'));
    setClient('$resourceServerClient', { listResourceServers });
    const l = useResourceServerList(projectId);
    await l.load();
    expect(listResourceServers).toHaveBeenCalledWith({ projectId });
    expect(l.servers.value).toHaveLength(1);
    await l.load();
    expect(l.loadError.value?.message).toBe('apperror.GEN900');
  });
});

describe('useScopeRows', () => {
  it('adds, removes and checks rows, keeping at least one and at most 64', () => {
    const s = useScopeRows([]);
    expect(s.rows.value).toEqual(['']);
    s.add();
    s.rows.value = ['openid', 'posts:read'];
    expect(s.check()).toBeNull();
    expect(s.errors.value).toEqual(['apperror.RSV004_reserved', '']);
    s.clearRow(0);
    expect(s.errors.value).toEqual(['', '']);
    s.clearRow(3);
    expect(s.errors.value).toEqual(['', '']);
    s.remove(0);
    expect([s.rows.value, s.errors.value]).toEqual([['posts:read'], ['']]);
    s.remove(0);
    expect(s.rows.value).toEqual(['']);
    s.reset(Array.from({ length: 64 }, (_, i) => `s${i}`));
    expect(s.full.value).toBe(true);
    s.add();
    expect(s.rows.value).toHaveLength(64);
    s.reset([]);
    expect(s.rows.value).toEqual(['']);
  });

  it('places only an RSV004 refusal that names a listed scope', () => {
    const s = useScopeRows(['a:b', '', 'c:d']);
    const list = s.check()!;
    expect(list.scopes).toEqual(['a:b', 'c:d']);
    expect(s.place(connectError('RSV004', { scope: 'c:d' }), 'bad', list)).toBe(true);
    expect(s.errors.value[2]).toBe('bad');
    expect(s.place(connectError('RSV004', { scope: 'x' }), 'bad', list)).toBe(false);
    expect(s.place(connectError('RSV005', { scope: 'a:b' }), 'bad', list)).toBe(false);
  });
});

describe('useResourceServerCreate', () => {
  let createResourceServer: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    createResourceServer = vi.fn().mockResolvedValue({ resourceServer: rs });
    setClient('$resourceServerClient', { createResourceServer });
  });

  it('creates a server with its trimmed fields', async () => {
    const c = useResourceServerCreate(projectId);
    c.name.value = ' Store MCP ';
    c.identifier.value = ' https://store.example/mcp ';
    c.scopes.rows.value = ['posts:read', '', 'posts:write'];
    expect(await c.submit()).toBe(rs);
    expect(createResourceServer).toHaveBeenCalledWith({ projectId, name: 'Store MCP', identifier: 'https://store.example/mcp', scopes: ['posts:read', 'posts:write'], scopeDescriptions: {}, trusted: false });
  });

  it('offers trust only where it is available, and requires it for an http identifier', async () => {
    const listResourceServers = vi.fn().mockResolvedValueOnce({ resourceServers: [], trustAvailable: true }).mockRejectedValueOnce(connectError('GEN900'));
    setClient('$resourceServerClient', { createResourceServer, listResourceServers });
    const c = useResourceServerCreate(projectId);
    c.identifier.value = 'http://127.0.0.1:8080/mcp';
    expect([c.trustRequired.value, c.trusted.value]).toEqual([true, false]);
    await c.loadTrust();
    expect(listResourceServers).toHaveBeenCalledWith({ projectId });
    expect([c.trustAvailable.value, c.trusted.value]).toEqual([true, true]);
    c.name.value = 'Local tools';
    c.scopes.rows.value = ['tools:run'];
    await c.submit();
    expect(createResourceServer).toHaveBeenLastCalledWith({ projectId, name: 'Local tools', identifier: 'http://127.0.0.1:8080/mcp', scopes: ['tools:run'], scopeDescriptions: {}, trusted: true });
    c.identifier.value = 'https://store.example/mcp';
    expect([c.trustRequired.value, c.trusted.value]).toEqual([false, false]);
    c.trustPicked.value = true;
    expect(c.trusted.value).toBe(true);
    await c.loadTrust();
    expect([c.trustAvailable.value, c.trusted.value]).toEqual([false, false]);
  });

  it('shows the console project\'s refusal as PRJ011', async () => {
    setKnownKeys('apperror.PRJ011');
    createResourceServer.mockRejectedValueOnce(connectError('PRJ011'));
    const c = useResourceServerCreate(projectId);
    c.name.value = 'Local tools';
    c.identifier.value = 'http://127.0.0.1:8080/mcp';
    c.scopes.rows.value = ['tools:run'];
    await c.submit();
    expect(c.error.value?.message).toBe('apperror.PRJ011');
  });

  it('puts a trust refusal under the identifier', async () => {
    setKnownKeys('apperror.RSV002_needs_trust', 'apperror.RSV007');
    createResourceServer.mockRejectedValueOnce(connectError('RSV002', { reason: 'needs_trust' })).mockRejectedValueOnce(connectError('RSV007'));
    const c = useResourceServerCreate(projectId);
    c.name.value = 'Local tools';
    c.identifier.value = 'http://127.0.0.1:8080/mcp';
    c.scopes.rows.value = ['tools:run'];
    await c.submit();
    expect(c.errors.value.identifier).toMatch(/^apperror\.RSV002_needs_trust/);
    await c.submit();
    expect(c.errors.value.identifier).toBe('apperror.RSV007');
    expect(c.error.value).toBeNull();
  });

  it('refuses a bad identifier, an empty name and no scopes before sending', async () => {
    const c = useResourceServerCreate(projectId);
    c.identifier.value = 'https://Store.Example/mcp';
    expect(await c.submit()).toBeNull();
    expect(createResourceServer).not.toHaveBeenCalled();
    expect(c.errors.value.identifier).toBe('apperror.RSV002_not_canonical|{"canonical":"https://store.example/mcp"}');
    expect(c.errors.value.name).toBe('validation.required');
    expect(c.scopes.errors.value).toEqual(['rs.scopes_required']);
    c.clearIdentifier();
    expect(c.errors.value.identifier).toBe('');
    c.identifier.value = 'ftp://store.example/mcp';
    await c.submit();
    expect(c.errors.value.identifier).toBe('apperror.RSV002_https_required');
    c.identifier.value = '';
    await c.submit();
    expect(c.errors.value.identifier).toBe('validation.required');
  });

  it('places a taken identifier under the field and a bad scope under its row', async () => {
    setKnownKeys('apperror.RSV003', 'apperror.RSV004_reserved', 'apperror.GEN900');
    createResourceServer
      .mockRejectedValueOnce(connectError('RSV003'))
      .mockRejectedValueOnce(connectError('RSV004', { scope: 'posts:write', reason: 'reserved' }))
      .mockRejectedValueOnce(connectError('GEN900'));
    const c = useResourceServerCreate(projectId);
    c.name.value = 'Store MCP';
    c.identifier.value = 'https://store.example/mcp';
    c.scopes.rows.value = ['posts:read', 'posts:write'];
    await c.submit();
    expect(c.errors.value.identifier).toBe('apperror.RSV003');
    await c.submit();
    expect(c.scopes.errors.value[1]).toMatch(/^apperror\.RSV004_reserved/);
    await c.submit();
    expect(c.error.value?.message).toBe('apperror.GEN900');
  });

  it('ignores a second submit while one runs', async () => {
    const pending = deferred<{ resourceServer: typeof rs }>();
    createResourceServer.mockReturnValue(pending.promise);
    const c = useResourceServerCreate(projectId);
    c.name.value = 'Store MCP';
    c.identifier.value = 'https://store.example/mcp';
    c.scopes.rows.value = ['posts:read'];
    const first = c.submit();
    expect(await c.submit()).toBeNull();
    pending.resolve({ resourceServer: rs });
    await first;
    expect(createResourceServer).toHaveBeenCalledTimes(1);
  });
});

describe('useResourceServer', () => {
  let api: Record<string, ReturnType<typeof vi.fn>>;

  beforeEach(() => {
    api = {
      getResourceServer: vi.fn().mockResolvedValue({ resourceServer: rs }),
      updateResourceServer: vi.fn(async (req: { name: string, scopes: string[] }) => ({ resourceServer: { ...rs, name: req.name, scopes: req.scopes } })),
      verifyResourceServer: vi.fn().mockResolvedValue({ resourceServer: { ...rs, state: ResourceServerState.VERIFIED }, passed: true }),
      setResourceServerTrust: vi.fn(async (req: { trusted: boolean }) => ({ resourceServer: { ...rs, trusted: req.trusted } })),
      deleteResourceServer: vi.fn().mockResolvedValue({}),
    };
    setClient('$resourceServerClient', api);
  });

  async function loaded() {
    const r = useResourceServer(projectId, rsId);
    await r.load();
    return r;
  }

  it('loads the server into the name and scope drafts', async () => {
    const r = await loaded();
    expect(api.getResourceServer).toHaveBeenCalledWith({ projectId, resourceServerId: rsId });
    expect([r.name.value, r.scopes.rows.value]).toEqual(['Store MCP', ['posts:read', 'posts:write']]);
  });

  it('tells not found apart from other load errors', async () => {
    api.getResourceServer!.mockRejectedValueOnce(connectError('RSV001')).mockRejectedValueOnce(connectError('GEN900')).mockResolvedValueOnce({});
    const r = useResourceServer(projectId, rsId);
    await r.load();
    expect(r.notFound.value).toBe(true);
    await r.load();
    expect([r.notFound.value, !!r.loadError.value]).toEqual([false, true]);
    await r.load();
    expect(r.notFound.value).toBe(true);
    expect(r.server.value).toBeNull();
  });

  it('saves the name and keeps the unsaved scope rows, and the other way round', async () => {
    const r = await loaded();
    r.name.value = 'Renamed';
    r.scopes.rows.value = ['posts:read', 'draft:x'];
    await r.save('name');
    expect(api.updateResourceServer).toHaveBeenLastCalledWith({ projectId, resourceServerId: rsId, name: 'Renamed', scopes: ['posts:read', 'posts:write'], scopeDescriptions: {} });
    expect([r.saved.value, r.server.value?.name, r.scopes.rows.value]).toEqual(['name', 'Renamed', ['posts:read', 'draft:x']]);
    r.name.value = 'Draft name';
    await r.save('scopes');
    expect(api.updateResourceServer).toHaveBeenLastCalledWith({ projectId, resourceServerId: rsId, name: 'Renamed', scopes: ['posts:read', 'draft:x'], scopeDescriptions: {} });
    expect([r.saved.value, r.name.value]).toEqual(['scopes', 'Draft name']);
  });

  it('does not save bad scopes or a blank name', async () => {
    const r = await loaded();
    r.scopes.rows.value = ['openid'];
    await r.save('scopes');
    r.name.value = ' ';
    await r.save('name');
    expect(api.updateResourceServer).not.toHaveBeenCalled();
    expect(r.nameError.value).toBe('validation.required');
    expect(r.saveError.value).toBeNull();
  });

  it('shows a pinned-scope refusal on the scopes card and a bad scope on its row', async () => {
    setKnownKeys('apperror.RSV005', 'apperror.RSV004_format', 'apperror.GEN900');
    api.updateResourceServer!
      .mockRejectedValueOnce(connectError('RSV005', { scope: 'posts:write', clients: 'Store web' }))
      .mockRejectedValueOnce(connectError('RSV004', { scope: 'posts:read', reason: 'format' }))
      .mockRejectedValueOnce(connectError('GEN900'));
    const r = await loaded();
    r.scopes.rows.value = ['posts:read'];
    await r.save('scopes');
    expect(r.saveError.value?.part).toBe('scopes');
    expect(r.saveError.value?.error.message).toMatch(/^apperror\.RSV005/);
    await r.save('scopes');
    expect(r.scopes.errors.value[0]).toMatch(/^apperror\.RSV004_format/);
    await r.save('name');
    expect(r.saveError.value).toEqual({ part: 'name', error: { message: 'apperror.GEN900', reference: 'error.ref|{"id":"req-1"}' } });
  });

  it('ignores a save before the load and while one runs', async () => {
    const r = useResourceServer(projectId, rsId);
    await r.save('name');
    await r.load();
    const pending = deferred<{ resourceServer: typeof rs }>();
    api.updateResourceServer!.mockReturnValue(pending.promise);
    const first = r.save('name');
    await r.save('scopes');
    pending.resolve({ resourceServer: rs });
    await first;
    expect(api.updateResourceServer).toHaveBeenCalledTimes(1);
  });

  it('verifies now, and a failed check reports why without changing the state', async () => {
    const r = await loaded();
    await r.verify();
    expect(r.verified.value).toEqual({ passed: true, failure: null });
    expect(r.server.value?.state).toBe(ResourceServerState.VERIFIED);
    const failure = create(VerificationFailureSchema, { reason: 'not_found' });
    api.verifyResourceServer!.mockResolvedValueOnce({ resourceServer: rs, passed: false, error: failure });
    await r.verify();
    expect(r.verified.value).toEqual({ passed: false, failure });
    expect(r.server.value?.state).toBe(ResourceServerState.PENDING);
  });

  it('shows a too-soon refusal and ignores a second verify while one runs', async () => {
    setKnownKeys('apperror.RSV008');
    api.verifyResourceServer!.mockRejectedValueOnce(connectError('RSV008', { time: '14:05' }));
    const r = await loaded();
    await r.verify();
    expect(r.verifyError.value?.message).toMatch(/^apperror\.RSV008/);
    const pending = deferred<object>();
    api.verifyResourceServer!.mockReturnValue(pending.promise);
    const first = r.verify();
    await r.verify();
    pending.resolve({ passed: true });
    await first;
    expect(api.verifyResourceServer).toHaveBeenCalledTimes(2);
  });

  it('asks before trusting, and stops trusting at once', async () => {
    const r = await loaded();
    await r.toggleTrust();
    expect(r.trustAsking.value).toBe(true);
    expect(api.setResourceServerTrust).not.toHaveBeenCalled();
    await r.setTrust(true);
    expect(api.setResourceServerTrust).toHaveBeenLastCalledWith({ projectId, resourceServerId: rsId, trusted: true });
    expect([r.trustAsking.value, r.server.value?.trusted]).toEqual([false, true]);
    await r.toggleTrust();
    expect(api.setResourceServerTrust).toHaveBeenLastCalledWith({ projectId, resourceServerId: rsId, trusted: false });
    expect(r.server.value?.trusted).toBe(false);
  });

  it('keeps the trust confirm open with the reason, and cancels it', async () => {
    setKnownKeys('apperror.RSV007');
    api.setResourceServerTrust!.mockRejectedValueOnce(connectError('RSV007'));
    const r = await loaded();
    await r.toggleTrust();
    await r.setTrust(true);
    expect(r.trustError.value?.message).toBe('apperror.RSV007');
    expect(r.trustAsking.value).toBe(true);
    r.cancelTrust();
    expect([r.trustAsking.value, r.trustError.value]).toEqual([false, null]);
  });

  it('ignores trust changes and cancels while one runs', async () => {
    const pending = deferred<object>();
    api.setResourceServerTrust!.mockReturnValue(pending.promise);
    const r = await loaded();
    await r.toggleTrust();
    const first = r.setTrust(true);
    await r.setTrust(true);
    r.cancelTrust();
    expect(r.trustAsking.value).toBe(true);
    pending.resolve({});
    await first;
    expect(api.setResourceServerTrust).toHaveBeenCalledTimes(1);
  });

  it('deletes after a confirm and shows a linked refusal in the dialog', async () => {
    setKnownKeys('apperror.RSV006');
    api.deleteResourceServer!.mockRejectedValueOnce(connectError('RSV006', { clients: 'Store web' }));
    const r = await loaded();
    expect(await r.confirmDelete()).toBe(false);
    r.askDelete();
    expect(await r.confirmDelete()).toBe(false);
    expect(r.deleteError.value?.message).toMatch(/^apperror\.RSV006/);
    expect(await r.confirmDelete()).toBe(true);
    expect(r.deleting.value).toBe(false);
    r.askDelete();
    r.cancelDelete();
    expect([r.deleting.value, r.deleteError.value]).toEqual([false, null]);
  });

  it('ignores a second delete and a cancel while one runs', async () => {
    const pending = deferred<object>();
    api.deleteResourceServer!.mockReturnValue(pending.promise);
    const r = await loaded();
    r.askDelete();
    const first = r.confirmDelete();
    expect(await r.confirmDelete()).toBe(false);
    r.cancelDelete();
    expect(r.deleting.value).toBe(true);
    pending.resolve({});
    expect(await first).toBe(true);
  });
});

describe('scope descriptions', () => {
  it('keeps descriptions aligned with their rows through add, remove and reset', () => {
    const s = useScopeRows(['a:read', 'b:write'], { 'b:write': 'Write b' });
    expect(s.descriptions.value).toEqual(['', 'Write b']);
    s.add();
    expect(s.descriptions.value).toEqual(['', 'Write b', '']);
    s.remove(0);
    expect([s.rows.value, s.descriptions.value]).toEqual([['b:write', ''], ['Write b', '']]);
    s.remove(0);
    s.remove(0);
    expect([s.rows.value, s.descriptions.value]).toEqual([[''], ['']]);
    s.reset(['c:x'], { 'c:x': 'See c', 'gone': 'dropped' });
    expect(s.descriptions.value).toEqual(['See c']);
  });

  it('sends trimmed descriptions keyed by scope, drops empty ones, and refuses one over 120 characters', () => {
    const s = useScopeRows(['a:read', '', 'b:write', 'c:x'], { 'a:read': '  Read a  ', 'c:x': '' });
    s.descriptions.value[2] = 'Write b';
    const list = s.check()!;
    expect(s.describe(list)).toEqual({ 'a:read': 'Read a', 'b:write': 'Write b' });
    s.descriptions.value[3] = 'x'.repeat(121);
    expect(s.check()).toBeNull();
    expect(s.errors.value[3]).toBe('apperror.RSV004_description|{"scope":"c:x"}');
  });

  it('creates and updates with the descriptions, and keeps the stored ones on a name save', async () => {
    const createResourceServer = vi.fn().mockResolvedValue({ resourceServer: rs });
    setClient('$resourceServerClient', { createResourceServer });
    const c = useResourceServerCreate(projectId);
    c.name.value = 'Store MCP';
    c.identifier.value = 'https://store.example/mcp';
    c.scopes.rows.value = ['posts:read'];
    c.scopes.descriptions.value = ['Read your posts'];
    await c.submit();
    expect(createResourceServer.mock.calls[0]![0].scopeDescriptions).toEqual({ 'posts:read': 'Read your posts' });

    const described = { ...rs, scopeDescriptions: { 'posts:read': 'Read your posts' } };
    const updateResourceServer = vi.fn(async (req: { name: string, scopes: string[], scopeDescriptions: Record<string, string> }) => ({ resourceServer: { ...described, name: req.name, scopes: req.scopes, scopeDescriptions: req.scopeDescriptions } }));
    setClient('$resourceServerClient', { getResourceServer: vi.fn().mockResolvedValue({ resourceServer: described }), updateResourceServer });
    const r = useResourceServer(projectId, rsId);
    await r.load();
    expect(r.scopes.descriptions.value).toEqual(['Read your posts', '']);
    r.scopes.descriptions.value = ['Draft', ''];
    r.name.value = 'Renamed';
    await r.save('name');
    expect(updateResourceServer.mock.calls[0]![0].scopeDescriptions).toEqual({ 'posts:read': 'Read your posts' });
    expect(r.scopes.descriptions.value).toEqual(['Draft', '']);
    await r.save('scopes');
    expect(updateResourceServer.mock.calls[1]![0].scopeDescriptions).toEqual({ 'posts:read': 'Draft' });
  });
});
