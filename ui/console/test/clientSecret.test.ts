// @vitest-environment happy-dom
import { create } from '@bufbuild/protobuf';
import { flushPromises } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import { WhoamiResponseSchema } from '#gen/auth/v1/auth_pb';
import { ClientKind, ClientSchema } from '#gen/client/v1/client_pb';
import { ProjectSchema } from '#gen/project/v1/project_pb';
import { mountPage } from '#test/mount-page';
import { connectError, navigateTo, setClient, setRoute } from '#test/nuxt-globals';
import ClientForm from '~/components/client/ClientForm.vue';
import ClientDetail from '~/pages/o/[org]/projects/[project]/clients/[client].vue';
import ClientNew from '~/pages/o/[org]/projects/[project]/clients/new.vue';
import { useOrgsStore } from '~/stores/orgs';
import { useProjectsStore } from '~/stores/projects';

const orgId = 'B6rTn1YkM8wQz3XpL5vGd';
const projectId = 'x6MH9WPWjktMwCBKHy4pF';
const web = create(ClientSchema, { id: 'dPm7fmQRJKPVw8PxrgdrN', projectId, name: 'Store web', kind: ClientKind.WEB, redirectUris: ['https://store.example/cb'] });
const base = '/o/acme/projects/web/clients';

beforeEach(() => {
  setActivePinia(createPinia());
  vi.stubGlobal('onBeforeRouteLeave', vi.fn());
  useOrgsStore().items = [{ id: orgId, slug: 'acme', name: 'Acme', role: 'owner', permissions: [] }];
  useProjectsStore().items = [{ id: projectId, orgId, slug: 'web', name: 'Web' }];
  setClient('$authClient', { whoami: vi.fn().mockResolvedValue(create(WhoamiResponseSchema, { permissions: ['project:read', 'project:write'] })) });
  setClient('$clientClient', {
    createClient: vi.fn().mockResolvedValue({ client: web }),
    generateClientSecret: vi.fn().mockRejectedValue(connectError('GEN001')),
    getClient: vi.fn().mockResolvedValue({ client: web }),
  });
  setClient('$projectClient', { getProject: vi.fn().mockResolvedValue({ project: create(ProjectSchema, { id: projectId, orgId }) }) });
  setClient('$resourceServerClient', { listResourceServers: vi.fn().mockResolvedValue({ resourceServers: [] }), listLinks: vi.fn().mockResolvedValue({ links: [] }) });
});

describe('a new client whose first secret fails', () => {
  it('lands on the client with the failure flagged, not silently', async () => {
    setRoute({ path: `${base}/new`, params: { org: 'acme', project: 'web' } });
    const w = await mountPage(ClientNew);
    const form = w.findComponent(ClientForm);
    form.vm.$emit('update:fields', { ...form.props('fields'), name: 'Store web', redirectUris: ['https://store.example/cb'] });
    await flushPromises();
    form.vm.$emit('submit');
    await flushPromises();
    expect(navigateTo).toHaveBeenCalledWith({ path: `${base}/${web.id}`, query: { focus: 'secrets', secret: 'failed' } });
  });

  it('says so on the Secrets card', async () => {
    setRoute({ path: `${base}/${web.id}`, params: { org: 'acme', project: 'web', client: web.id }, query: { focus: 'secrets', secret: 'failed' } });
    const w = await mountPage(ClientDetail);
    await flushPromises();
    expect(w.get('[data-testid="secrets-card"]').text()).toContain('clients.secret_not_generated');
  });
});
