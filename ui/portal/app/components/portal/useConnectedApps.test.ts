import { create } from '@bufbuild/protobuf';
import { timestampFromDate } from '@bufbuild/protobuf/wkt';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import { AppResourceSchema, ConnectedAppSchema } from '#gen/account/v1/account_pb';
import { connectError, setClient, setKnownKeys } from '#test/nuxt-globals';
import { useConnectedApps } from './useConnectedApps';

const at = new Date('2026-10-01T00:00:00Z');
const claude = create(ConnectedAppSchema, {
  clientId: 'dPm7fmQRJKPVw8PxrgdrN',
  name: 'Claude Code',
  metadataHost: 'claude.ai',
  consented: true,
  connectedAt: timestampFromDate(at),
  resources: [create(AppResourceSchema, { name: '', identifier: 'https://mcp.acme.test/mcp', scopes: ['notes:read', 'notes:write'], scopeDescriptions: { 'notes:read': 'Read your notes' } })],
});

describe('useConnectedApps', () => {
  let client: Record<string, ReturnType<typeof vi.fn>>;

  beforeEach(() => {
    client = { listConnectedApps: vi.fn().mockResolvedValue({ apps: [claude] }), disconnectApp: vi.fn().mockResolvedValue({}) };
    setClient('$accountClient', client);
  });

  it('loads and disconnects an app', async () => {
    const a = useConnectedApps();
    await a.load();
    expect(a.apps.value).toHaveLength(1);
    a.disconnect.ask(a.apps.value[0]!);
    await a.disconnect.confirm();
    expect(client.disconnectApp).toHaveBeenCalledWith({ clientId: 'dPm7fmQRJKPVw8PxrgdrN' });
    expect(a.apps.value).toEqual([]);
  });

  it('shows a load error', async () => {
    setKnownKeys('apperror.GEN900');
    client.listConnectedApps!.mockRejectedValueOnce(connectError('GEN900'));
    const a = useConnectedApps();
    await a.load();
    expect(a.loadError.value?.message).toBe('apperror.GEN900');
  });
});
