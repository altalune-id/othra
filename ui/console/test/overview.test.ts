// @vitest-environment happy-dom
import { create } from '@bufbuild/protobuf';
import { timestampFromDate } from '@bufbuild/protobuf/wkt';
import { flushPromises } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';

import { beforeEach, describe, expect, it, vi } from 'vitest';
import { WhoamiResponseSchema } from '#gen/auth/v1/auth_pb';
import { SigningKeySchema, SigningKeyStatus } from '#gen/keys/v1/keys_pb';
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
  it('draws the design overview: header actions, two columns, the mono active key, the rotation timeline and the setup rows', async () => {
    setup({ issuerHandle: 'web', signinEmailLink: true });
    const day = 86_400_000;
    const at = (ago: number) => timestampFromDate(new Date(Date.now() - ago * day));
    setClient('$signingKeyClient', { listSigningKeys: vi.fn().mockResolvedValue({ keys: [
      create(SigningKeySchema, { publicId: 'k1', kid: '2026-08-aXYZ', alg: 'ES256', status: SigningKeyStatus.ACTIVE, createdAt: at(80), activatedAt: at(78) }),
      create(SigningKeySchema, { publicId: 'k0', kid: '2026-02-bXYZ', alg: 'ES256', status: SigningKeyStatus.RETIRED, createdAt: at(200), retireAfter: at(70) }),
      create(SigningKeySchema, { publicId: 'k2', kid: '2026-11-aXYZ', alg: 'ES256', status: SigningKeyStatus.NEXT, createdAt: at(1), activatesAt: at(-7) }),
    ] }) });
    const w = await mountPage(Overview, { attachTo: document.body });
    await flushPromises();
    const actions = w.get('header').findAll('a, button').map(b => b.text());
    expect(actions).toEqual(expect.arrayContaining(['overview.copy_issuer', 'overview.view_json', 'overview.try_signin']));
    expect(w.get('header a.btn-primary').attributes('href')).toBe(`${issuer}/account/`);
    expect(w.get('.grid').classes()).toEqual(expect.arrayContaining(['grid-cols-1', 'xl:grid-cols-[minmax(0,2fr)_minmax(16rem,1fr)]']));
    const status = w.get('[data-testid="issuer-status"]');
    expect(status.text()).toContain('overview.active_key');
    expect(status.get('.font-mono.text-xs.font-medium').text()).toBe('2026-08-');
    expect(status.text()).toContain('overview.active_key_detail|{"alg":"ES256","n":78}');
    expect(w.get('[data-testid="cache-note"]').classes()).toEqual(expect.arrayContaining(['text-ui', 'text-base-content/70']));
    await w.findAll('header button').find(b => b.text() === 'overview.view_json')!.trigger('click');
    await flushPromises();
    const drawer = document.body.querySelector('body > .drawer-end')!;
    expect((drawer.querySelector('input.drawer-toggle') as HTMLInputElement).checked).toBe(true);
    expect(drawer.querySelector('h2')!.textContent!.trim()).toBe('overview.discovery_doc');
    const json = drawer.querySelector('[data-testid="discovery-json"]') as HTMLElement;
    expect(json.textContent).toContain(issuer);
    expect(json.tabIndex).toBe(0);
    expect(json.getAttribute('aria-label')).toBe('overview.discovery_doc');
    const rotation = w.get('[data-testid="key-rotation"]');
    expect(rotation.findAll('.timeline-box').map(b => b.text())).toEqual(['2026-02-', '2026-08-', '2026-11-']);
    expect(rotation.findAll('.timeline-start').at(-1)!.text()).toMatch(/^overview\.rotation_next\|\{"date":"[^"]+"\}$/);
    expect(rotation.findAll('.timeline-middle').at(-1)!.get('[data-icon]').attributes('data-icon')).toBe('lucide:circle-dashed');
    expect(status.text()).toContain('overview.next_rotation');
    expect(status.text()).not.toContain('overview.next_rotation_none');
    expect(rotation.get('.timeline-middle.text-primary').exists()).toBe(true);
    const row = w.get('[data-testid="setup-checklist"] li');
    expect(row.classes()).toEqual(['flex', 'items-start', 'gap-2', 'px-4', 'py-2', 'text-ui']);
    expect(w.get('[data-testid="setup-checklist"] progress').classes()).toEqual(['progress', 'mt-1', 'h-1']);
    w.unmount();
  });

  it('says Not scheduled for the next rotation when there is no NEXT key', async () => {
    setup({ issuerHandle: 'web' });
    setClient('$signingKeyClient', { listSigningKeys: vi.fn().mockResolvedValue({ keys: [
      create(SigningKeySchema, { publicId: 'k1', kid: '2026-08-aXYZ', alg: 'ES256', status: SigningKeyStatus.ACTIVE, createdAt: timestampFromDate(new Date()) }),
    ] }) });
    const w = await mountPage(Overview);
    await flushPromises();
    expect(w.get('[data-testid="issuer-status"]').text()).toContain('overview.next_rotation_none');
  });

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

  it('shows a failed project load in Setup with Retry, never as steps not done', async () => {
    setup({ issuerHandle: 'web', signinEmailLink: true });
    const getProject = vi.fn().mockRejectedValueOnce(new Error('offline')).mockResolvedValue({ project: create(ProjectSchema, { id: projectId, orgId, issuerUrl: issuer, issuerHandle: 'web', signinEmailLink: true }) });
    setClient('$projectClient', { getProject });
    const w = await mountPage(Overview);
    await flushPromises();
    const checklist = w.get('[data-testid="setup-checklist"]');
    expect(checklist.text()).not.toContain('overview.setup_progress');
    expect(checklist.text()).not.toContain('overview.setup_handle');
    await checklist.findAll('button').find(b => b.text().includes('common.retry'))!.trigger('click');
    await flushPromises();
    expect(getProject).toHaveBeenCalledTimes(2);
    expect(checklist.text()).toContain('overview.setup_progress');
  });

  it('stops saying Loading on the keys row once discovery has failed', async () => {
    setup({}, 0, 'down');
    const w = await mountPage(Overview);
    await flushPromises();
    const status = w.get('[data-testid="issuer-status"]').text();
    expect(status).not.toContain('common.loading');
    expect(status).toContain('—');
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
