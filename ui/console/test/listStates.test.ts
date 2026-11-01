// @vitest-environment happy-dom
import { create } from '@bufbuild/protobuf';
import { flushPromises } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import { WhoamiResponseSchema } from '#gen/auth/v1/auth_pb';
import { mountPage } from '#test/mount-page';
import { setClient, setRoute } from '#test/nuxt-globals';
import WebhookList from '~/components/webhook/list/WebhookList.vue';
import ResourceServers from '~/pages/o/[org]/projects/[project]/resource-servers/index.vue';
import { useOrgsStore } from '~/stores/orgs';
import { useProjectsStore } from '~/stores/projects';

const orgId = 'B6rTn1YkM8wQz3XpL5vGd';
const projectId = 'x6MH9WPWjktMwCBKHy4pF';
const retry = (w: Awaited<ReturnType<typeof mountPage>>) => w.findAll('button').find(b => b.text().includes('common.retry'))!;

beforeEach(() => {
  setActivePinia(createPinia());
  useOrgsStore().items = [{ id: orgId, slug: 'acme', name: 'Acme', role: 'owner', permissions: [] }];
  useProjectsStore().items = [{ id: projectId, orgId, slug: 'web', name: 'Web' }];
  setClient('$authClient', { whoami: vi.fn().mockResolvedValue(create(WhoamiResponseSchema, { permissions: ['project:read', 'project:write'] })) });
});

describe('resource servers page', () => {
  it('shows the empty state with New for editors, and Retry after a failed load', async () => {
    setRoute({ path: '/o/acme/projects/web/resource-servers', params: { org: 'acme', project: 'web' } });
    const listResourceServers = vi.fn().mockRejectedValueOnce(new Error('offline')).mockResolvedValue({ resourceServers: [] });
    setClient('$resourceServerClient', { listResourceServers });
    const w = await mountPage(ResourceServers);
    await flushPromises();
    await retry(w).trigger('click');
    await flushPromises();
    expect(listResourceServers).toHaveBeenCalledTimes(2);
    expect(w.get('h2').text()).toBe('rs.empty_title');
    expect(w.text()).toContain('rs.new');
  });
});

describe('webhooks list', () => {
  it('offers Retry after a failed load, then shows the empty state', async () => {
    const listWebhooks = vi.fn().mockRejectedValueOnce(new Error('offline')).mockResolvedValue({ webhooks: [] });
    setClient('$webhookClient', { listWebhooks });
    const w = await mountPage(WebhookList, { pageProps: { orgId, projectId } });
    await flushPromises();
    await retry(w).trigger('click');
    await flushPromises();
    expect(listWebhooks).toHaveBeenCalledTimes(2);
    expect(w.get('h2').text()).toBe('webhooks.empty');
  });
});
