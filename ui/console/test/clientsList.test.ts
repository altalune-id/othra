// @vitest-environment happy-dom
import { create } from '@bufbuild/protobuf';
import { timestampFromDate } from '@bufbuild/protobuf/wkt';
import { flushPromises } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';
import { describe, expect, it, vi } from 'vitest';

import { WhoamiResponseSchema } from '#gen/auth/v1/auth_pb';
import { ClientKind, ClientSchema } from '#gen/client/v1/client_pb';
import { mountPage } from '#test/mount-page';
import { setClient, setRoute } from '#test/nuxt-globals';
import ClientIndex from '~/pages/o/[org]/projects/[project]/clients/index.vue';
import { useOrgsStore } from '~/stores/orgs';
import { useProjectsStore } from '~/stores/projects';

describe('clients list', () => {
  it('shows when each client last signed someone in, and Never for one that has not', async () => {
    setActivePinia(createPinia());
    const orgId = 'B6rTn1YkM8wQz3XpL5vGd';
    const projectId = 'x6MH9WPWjktMwCBKHy4pF';
    useOrgsStore().items = [{ id: orgId, slug: 'acme', name: 'Acme', role: 'owner', permissions: [] }];
    useProjectsStore().items = [{ id: projectId, orgId, slug: 'web', name: 'Web' }];
    setRoute({ path: '/o/acme/projects/web/clients', params: { org: 'acme', project: 'web' } });
    setClient('$authClient', { whoami: vi.fn().mockResolvedValue(create(WhoamiResponseSchema, { permissions: ['project:read'] })) });
    const used = create(ClientSchema, { id: 'dPm7fmQRJKPVw8PxrgdrN', name: 'Store web', kind: ClientKind.WEB, redirectUris: ['https://store.acme.example/callback'], lastUsedAt: timestampFromDate(new Date(Date.now() - 2 * 86_400_000)) });
    const fresh = create(ClientSchema, { id: 'H9MspUj32ux8pfYKYsu1a', name: 'Store CLI', kind: ClientKind.CLI });
    setClient('$clientClient', { listClients: vi.fn().mockResolvedValue({ clients: [used, fresh] }) });
    const w = await mountPage(ClientIndex);
    await flushPromises();
    const rows = w.findAll('tbody tr').map(r => r.findAll('td').map(td => td.text()));
    expect(rows.map(r => r[4])).toEqual(['2 days ago', 'clients.never_used']);
    expect(rows.map(r => r[1])).toEqual(['clients.type_web', 'clients.type_native']);
    expect(rows.map(r => r[3])).toEqual(['store.acme.example', 'clients.no_redirect']);
    const first = w.get('tbody tr');
    expect(first.get('td:nth-child(2) .badge [data-icon]').attributes('data-icon')).toBe('lucide:app-window');
    expect(first.get('td:nth-child(3)').classes()).toEqual(['font-mono', 'text-xs']);
    expect(first.get('td:nth-child(5) span').classes()).toEqual(['tabular-nums', 'text-base-content/70']);
    expect(first.get('td:nth-child(6) button[popovertarget] .sr-only').text()).toBe('clients.more_for|{"name":"Store web"}');
    expect(w.get('table').classes()).toEqual(['table', 'table-sm', 'text-ui']);
    expect(w.get('thead').text()).toContain('clients.col_last_used');
    expect(w.text()).not.toContain('clients.new');
  });
});
