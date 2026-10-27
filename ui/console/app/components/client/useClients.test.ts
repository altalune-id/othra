import { create } from '@bufbuild/protobuf';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import { ClientRegistration, ClientSchema } from '#gen/client/v1/client_pb';
import { connectError, deferred, setClient, setKnownKeys } from '#test/nuxt-globals';
import { useClient, useClientList } from './useClients';

const projectId = 'x6MH9WPWjktMwCBKHy4pF';
const clientId = 'dPm7fmQRJKPVw8PxrgdrN';
const web = create(ClientSchema, { id: clientId, name: 'Store web' });

describe('useClientList', () => {
  it('loads the clients, and shows a load error', async () => {
    setKnownKeys('apperror.GEN900');
    const listClients = vi.fn().mockResolvedValueOnce({ clients: [web] }).mockRejectedValueOnce(connectError('GEN900'));
    setClient('$clientClient', { listClients });
    const l = useClientList(projectId);
    expect(l.loading.value).toBe(true);
    await l.load();
    expect(listClients).toHaveBeenCalledWith({ projectId });
    expect(l.clients.value.map(c => c.name)).toEqual(['Store web']);
    expect(l.loading.value).toBe(false);
    await l.load();
    expect(l.loadError.value?.message).toBe('apperror.GEN900');
  });
});

describe('useClient', () => {
  let getClient: ReturnType<typeof vi.fn>;
  let deleteClient: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    getClient = vi.fn().mockResolvedValue({ client: web });
    deleteClient = vi.fn().mockResolvedValue({});
    setClient('$clientClient', { getClient, deleteClient });
  });

  it('loads one client and replaces it after a save', async () => {
    const c = useClient(projectId, clientId);
    await c.load();
    expect(getClient).toHaveBeenCalledWith({ projectId, clientId });
    expect(c.client.value?.name).toBe('Store web');
    c.replace(create(ClientSchema, { id: clientId, name: 'Renamed' }));
    expect(c.client.value?.name).toBe('Renamed');
  });

  it('tells not found apart from other load errors', async () => {
    setKnownKeys('apperror.CLT001');
    getClient.mockRejectedValueOnce(connectError('CLT001')).mockRejectedValueOnce(connectError('GEN900')).mockResolvedValueOnce({});
    const c = useClient(projectId, clientId);
    await c.load();
    expect(c.notFound.value).toBe(true);
    await c.load();
    expect(c.notFound.value).toBe(false);
    expect(c.loadError.value).not.toBeNull();
    await c.load();
    expect(c.notFound.value).toBe(true);
  });

  it('deletes after a confirm, and keeps the confirm open on a refusal', async () => {
    setKnownKeys('apperror.GEN900');
    deleteClient.mockRejectedValueOnce(connectError('GEN900'));
    const c = useClient(projectId, clientId);
    expect(await c.confirmDelete()).toBe(false);
    c.askDelete();
    expect(await c.confirmDelete()).toBe(false);
    expect(c.error.value?.message).toBe('apperror.GEN900');
    expect(c.deleting.value).toBe(true);
    expect(await c.confirmDelete()).toBe(true);
    expect(deleteClient).toHaveBeenCalledWith({ projectId, clientId });
    expect(c.deleting.value).toBe(false);
    c.askDelete();
    c.cancelDelete();
    expect(c.deleting.value).toBe(false);
  });

  it('ignores a second delete and a cancel while one runs', async () => {
    const pending = deferred<object>();
    deleteClient.mockReturnValue(pending.promise);
    const c = useClient(projectId, clientId);
    c.askDelete();
    const first = c.confirmDelete();
    expect(await c.confirmDelete()).toBe(false);
    c.cancelDelete();
    expect(c.deleting.value).toBe(true);
    pending.resolve({});
    expect(await first).toBe(true);
    expect(deleteClient).toHaveBeenCalledTimes(1);
  });
});

describe('useClientList and apps without registration (M6)', () => {
  it('lists registered clients only', async () => {
    setClient('$clientClient', { listClients: vi.fn().mockResolvedValue({ clients: [web, create(ClientSchema, { id: 'c', name: 'Claude Code', registration: ClientRegistration.CIMD })] }) });
    const l = useClientList(projectId);
    await l.load();
    expect(l.clients.value.map(c => c.name)).toEqual(['Store web']);
  });
});
