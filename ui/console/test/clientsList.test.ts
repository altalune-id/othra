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
    const used = create(ClientSchema, { id: 'dPm7fmQRJKPVw8PxrgdrN', name: 'Store web', kind: ClientKind.WEB, lastUsedAt: timestampFromDate(new Date(Date.now() - 2 * 86_400_000)) });
    const fresh = create(ClientSchema, { id: 'H9MspUj32ux8pfYKYsu1a', name: 'Store CLI', kind: ClientKind.CLI });
    setClient('$clientClient', { listClients: vi.fn().mockResolvedValue({ clients: [used, fresh] }) });
    const w = await mountPage(ClientIndex);
    await flushPromises();
    const cells = w.findAll('tbody tr').map(r => r.findAll('td').at(-1)!.text());
    expect(cells).toEqual(['2 days ago', 'clients.never_used']);
    expect(w.get('thead').text()).toContain('clients.col_last_used');
    expect(w.text()).not.toContain('clients.new');
  });
});
