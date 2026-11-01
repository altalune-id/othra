// @vitest-environment happy-dom
import { create } from '@bufbuild/protobuf';
import { flushPromises } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import { WhoamiResponseSchema } from '#gen/auth/v1/auth_pb';
import { ProjectSchema } from '#gen/project/v1/project_pb';
import { mountPage } from '#test/mount-page';
import { deferred, setClient, setRoute } from '#test/nuxt-globals';
import Overview from '~/pages/o/[org]/projects/[project]/index.vue';
import { useOrgsStore } from '~/stores/orgs';
import { useProjectsStore } from '~/stores/projects';

const orgId = 'B6rTn1YkM8wQz3XpL5vGd';
const projectId = 'x6MH9WPWjktMwCBKHy4pF';
const issuer = 'https://id.acme.test/p/web';

function setup(project: Partial<{ issuerHandle: string, signinEmailLink: boolean, termsUrl: string, privacyUrl: string }>, clients = 0, discovery: 'ok' | 'down' = 'ok') {
  setActivePinia(createPinia());
  useOrgsStore().items = [{ id: orgId, slug: 'acme', name: 'Acme', role: 'owner', permissions: [] }];
  useProjectsStore().items = [{ id: projectId, orgId, slug: 'web', name: 'Web' }];
  setRoute({ path: '/o/acme/projects/web', params: { org: 'acme', project: 'web' } });
  setClient('$authClient', { whoami: vi.fn().mockResolvedValue(create(WhoamiResponseSchema, { permissions: ['project:read'] })) });
  setClient('$projectClient', { getProject: vi.fn().mockResolvedValue({ project: create(ProjectSchema, { id: projectId, orgId, issuerUrl: issuer, ...project }) }) });
  setClient('$clientClient', { listClients: vi.fn().mockResolvedValue({ clients: Array.from({ length: clients }, () => ({})) }) });
  setClient('$upstreamClient', { listUpstreams: vi.fn().mockResolvedValue({ upstreams: [] }) });
  const fetcher = discovery === 'ok'
    ? vi.fn().mockResolvedValueOnce({ ok: true, json: async () => ({ issuer, jwks_uri: `${issuer}/oauth2/jwks.json` }) }).mockResolvedValueOnce({ ok: true, json: async () => ({ keys: [{}] }) })
    : vi.fn().mockRejectedValue(new Error('offline'));
  vi.stubGlobal('fetch', fetcher);
  return fetcher;
}

beforeEach(() => {
  vi.unstubAllGlobals();
});

describe('project overview', () => {
  it('titles the page with the issuer and shows live discovery and the published keys', async () => {
    const fetcher = setup({ issuerHandle: 'web', signinEmailLink: true });
    const w = await mountPage(Overview, { attachTo: document.body });
    await flushPromises();
    expect(w.get('h1').text()).toBe(issuer);
    expect(fetcher).toHaveBeenCalledWith(`${issuer}/.well-known/openid-configuration`);
    const status = w.get('[data-testid="issuer-status"]').text();
    expect(status).toContain('overview.discovery_serving');
    expect(status).toContain('overview.keys_published|{"n":1}');
    expect(status).toContain('overview.method_email');
  });

  it('shows Not reachable with Retry when discovery fails, and only that row', async () => {
    setup({}, 0, 'down');
    const w = await mountPage(Overview);
    await flushPromises();
    const status = w.get('[data-testid="issuer-status"]');
    expect(status.text()).toContain('overview.discovery_unreachable');
    expect(status.findAll('button').some(b => b.text().includes('common.retry'))).toBe(true);
    expect(w.find('[data-testid="setup-checklist"]').exists()).toBe(true);
  });

  it('shows the page before the counts arrive, with skeletons in Setup', async () => {
    setup({ issuerHandle: 'web' });
    const pending = deferred<{ clients: unknown[] }>();
    setClient('$clientClient', { listClients: vi.fn().mockReturnValue(pending.promise) });
    const w = await mountPage(Overview);
    const checklist = w.get('[data-testid="setup-checklist"]');
    expect(checklist.find('[data-testid="setup-loading"]').exists()).toBe(true);
    expect(checklist.find('progress').exists()).toBe(false);
    pending.resolve({ clients: [{}] });
    await flushPromises();
    expect(checklist.find('[data-testid="setup-loading"]').exists()).toBe(false);
    expect(checklist.text()).toContain('overview.setup_progress|{"done":2,"total":5}');
  });

  it('never counts a failed read as zero: Setup shows Retry, Methods says it could not load', async () => {
    setup({ issuerHandle: 'web', signinEmailLink: true });
    const listClients = vi.fn().mockRejectedValueOnce(new Error('offline')).mockResolvedValue({ clients: [{}] });
    setClient('$clientClient', { listClients });
    const w = await mountPage(Overview);
    await flushPromises();
    const checklist = w.get('[data-testid="setup-checklist"]');
    expect(checklist.text()).not.toContain('overview.setup_progress');
    expect(checklist.text()).not.toContain('overview.setup_client');
    expect(w.get('[data-testid="issuer-status"]').text()).toContain('overview.methods_failed');
    await checklist.findAll('button').find(b => b.text().includes('common.retry'))!.trigger('click');
    await flushPromises();
    expect(listClients).toHaveBeenCalledTimes(2);
    expect(checklist.text()).toContain('overview.setup_progress|{"done":3,"total":5}');
    expect(w.get('[data-testid="issuer-status"]').text()).toContain('overview.method_email');
  });

  it('counts the setup steps from real data', async () => {
    setup({ issuerHandle: 'web', signinEmailLink: true, termsUrl: 'https://t', privacyUrl: 'https://p' }, 2);
    const w = await mountPage(Overview);
    await flushPromises();
    const setup5 = w.get('[data-testid="setup-checklist"]');
    expect(setup5.text()).toContain('overview.setup_progress|{"done":4,"total":5}');
    expect(setup5.findAll('a, .btn').filter(b => b.text().includes('overview.setup_go'))).toHaveLength(1);
  });
});
