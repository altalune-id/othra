// @vitest-environment happy-dom
import { create } from '@bufbuild/protobuf';
import { createPinia, setActivePinia } from 'pinia';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import { WhoamiResponseSchema } from '#gen/auth/v1/auth_pb';
import { ClientKind, ClientRegistration, ClientSchema } from '#gen/client/v1/client_pb';
import { ProjectSchema } from '#gen/project/v1/project_pb';
import { mountPage } from '#test/mount-page';
import { navigateTo, setClient, setRoute } from '#test/nuxt-globals';
import ClientDetail from '~/pages/o/[org]/projects/[project]/clients/[client].vue';
import ClientIndex from '~/pages/o/[org]/projects/[project]/clients/index.vue';
import ClientNew from '~/pages/o/[org]/projects/[project]/clients/new.vue';
import Overview from '~/pages/o/[org]/projects/[project]/index.vue';
import ResourceServerIndex from '~/pages/o/[org]/projects/[project]/resource-servers/index.vue';
import ResourceServerNew from '~/pages/o/[org]/projects/[project]/resource-servers/new.vue';
import SettingsGeneral from '~/pages/o/[org]/projects/[project]/settings/index.vue';
import SettingsIssuer from '~/pages/o/[org]/projects/[project]/settings/issuer.vue';
import SignIn from '~/pages/o/[org]/projects/[project]/settings/sign-in.vue';
import { useOrgsStore } from '~/stores/orgs';
import { useProjectsStore } from '~/stores/projects';

const orgId = 'B6rTn1YkM8wQz3XpL5vGd';
const projectId = 'x6MH9WPWjktMwCBKHy4pF';
const consoleClient = create(ClientSchema, {
  id: 'Lq3vT8nWx2KpZr6YbM9cD',
  projectId,
  name: 'Othra console',
  kind: ClientKind.WEB,
  registration: ClientRegistration.SYSTEM,
  redirectUris: ['http://localhost:8080/oauth/callback'],
  allowedScopes: ['openid', 'profile', 'email'],
});

beforeEach(() => {
  setActivePinia(createPinia());
  vi.stubGlobal('onBeforeRouteLeave', vi.fn());
  useOrgsStore().items = [{ id: orgId, slug: 'othra', name: 'Othra', role: 'owner', permissions: [], system: true }];
  useProjectsStore().items = [{ id: projectId, orgId, slug: 'console', name: 'Console', system: true }];
  setClient('$authClient', { whoami: vi.fn().mockResolvedValue(create(WhoamiResponseSchema, { permissions: ['project:read', 'project:write'] })) });
  setClient('$clientClient', { listClients: vi.fn().mockResolvedValue({ clients: [consoleClient] }), getClient: vi.fn().mockResolvedValue({ client: consoleClient }) });
  setClient('$projectClient', { getProject: vi.fn().mockResolvedValue({ project: create(ProjectSchema, { id: projectId, orgId, system: true, signinEmailLink: true }) }) });
  setClient('$upstreamClient', { listUpstreams: vi.fn().mockResolvedValue({ upstreams: [] }) });
  setClient('$resourceServerClient', { listResourceServers: vi.fn().mockResolvedValue({ resourceServers: [] }), listLinks: vi.fn().mockResolvedValue({ links: [] }) });
});

describe('the console project, for an editor', () => {
  it('lists its clients with the system note and no New', async () => {
    setRoute({ path: '/o/othra/projects/console/clients', params: { org: 'othra', project: 'console' } });
    const w = await mountPage(ClientIndex);
    expect(w.text()).toContain('clients.system_note');
    expect(w.text()).not.toContain('clients.new');
    expect(w.text()).toContain('common.system');
  });

  it('shows the console client read-only: the note, no Delete, no settings form, no secrets', async () => {
    setRoute({ path: `/o/othra/projects/console/clients/${consoleClient.id}`, params: { org: 'othra', project: 'console', client: consoleClient.id } });
    const w = await mountPage(ClientDetail);
    expect(w.text()).toContain('clients.system_client_note');
    expect(w.findAll('button').filter(b => b.text().includes('common.delete') && !b.element.closest('dialog'))).toEqual([]);
    expect(w.find('[data-testid="client-settings-readonly"]').exists()).toBe(true);
    expect(w.find('[data-testid="client-settings"] form').exists()).toBe(false);
    expect(w.find('[data-testid="secrets-card"]').exists()).toBe(false);
  });
});

describe('the console project, for an editor, elsewhere', () => {
  it.each([
    ['clients', ClientNew],
    ['resource-servers', ResourceServerNew],
  ])('sends %s/new back to the list', async (section, page) => {
    setRoute({ path: `/o/othra/projects/console/${section}/new`, params: { org: 'othra', project: 'console' } });
    await mountPage(page);
    expect(navigateTo).toHaveBeenCalledWith(`/o/othra/projects/console/${section}`, { replace: true });
  });

  it('keeps the email link locked on, with the reason', async () => {
    setRoute({ path: '/o/othra/projects/console/settings/sign-in', params: { org: 'othra', project: 'console' } });
    const w = await mountPage(SignIn);
    const toggle = w.find('[data-testid="email-link-card"] input[type="checkbox"]');
    expect(toggle.attributes('aria-disabled')).toBe('true');
    expect((toggle.element as HTMLInputElement).checked).toBe(true);
    expect(w.text()).toContain('signin.email_locked_system');
  });
});

describe('the console project pages', () => {
  const pages = [
    ['overview', '', Overview, 'project.system_note'],
    ['general settings', '/settings', SettingsGeneral, 'project.system_general'],
    ['issuer settings', '/settings/issuer', SettingsIssuer, 'project.system_issuer'],
    ['resource servers', '/resource-servers', ResourceServerIndex, 'rs.system_empty'],
  ] as const;

  function open(path: string) {
    setRoute({ path: `/o/othra/projects/console${path}`, params: { org: 'othra', project: 'console' } });
  }

  it.each(pages)('%s: tells an editor what Othra manages, with no edit form', async (_name, path, page, note) => {
    open(path);
    const w = await mountPage(page);
    expect(w.text()).toContain(note);
    expect(w.findAll('form').filter(f => !f.element.closest('dialog'))).toEqual([]);
  });

  it.each(pages)('%s: shows a viewer the plain read-only note, not what they could change', async (_name, path, page, note) => {
    setClient('$authClient', { whoami: vi.fn().mockResolvedValue(create(WhoamiResponseSchema, { permissions: ['project:read'] })) });
    open(path);
    const w = await mountPage(page);
    expect(w.text()).not.toContain(note);
    expect(w.text()).not.toContain('project.system_note');
    expect(w.text()).toContain('project.read_only');
  });
});
