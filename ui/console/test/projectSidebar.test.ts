// @vitest-environment happy-dom
import { mount, RouterLinkStub } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';
import { beforeEach, describe, expect, it } from 'vitest';

import { setRoute } from '#test/nuxt-globals';
import AppSidebar from '~/components/layout/AppSidebar.vue';
import ProjectSidebar from '~/components/project/ProjectSidebar.vue';
import { useAuthStore } from '~/stores/auth';
import { useOrgsStore } from '~/stores/orgs';
import { useProjectsStore } from '~/stores/projects';

const orgId = 'B6rTn1YkM8wQz3XpL5vGd';
const stubs = { OrgSelector: true, ProjectSelector: true, SidebarUser: true };

beforeEach(() => {
  setActivePinia(createPinia());
  useOrgsStore().items = [{ id: orgId, slug: 'acme', name: 'Acme', role: 'admin', permissions: [] }];
  useProjectsStore().items = [{ id: 'x6MH9WPWjktMwCBKHy4pF', orgId, slug: 'web', name: 'Web' }];
});

const sections = (w: ReturnType<typeof mount>) => w.findAll('.menu-title').map(g => g.text());
const links = (w: ReturnType<typeof mount>) => w.findAllComponents(RouterLinkStub).map(a => [a.text(), a.props('to')]);

describe('project sidebar', () => {
  it('groups every project page under one sidebar with real links, keeping the existing routes', () => {
    setRoute({ path: '/o/acme/projects/web/settings/keys', params: { org: 'acme', project: 'web' } });
    useAuthStore().me = { permissions: ['end_user:read'] } as never;
    const w = mount(ProjectSidebar, { global: { stubs } });
    expect(sections(w)).toEqual(['nav.group_applications', 'nav.group_people', 'nav.group_signin', 'nav.group_security', 'nav.group_developers']);
    const base = '/o/acme/projects/web';
    expect(links(w)).toEqual([
      ['nav.org_overview', '/o/acme'],
      ['nav.projects', '/o/acme/projects'],
      ['members.title', '/o/acme/settings/members'],
      ['apikeys.title', '/o/acme/settings/api-keys'],
      ['nav.org_settings', '/o/acme/settings'],
      ['nav.overview', base],
      ['nav.clients', `${base}/clients`],
      ['nav.resource_servers', `${base}/resource-servers`],
      ['nav.end_users', `${base}/end-users`],
      ['nav.methods', `${base}/settings/sign-in`],
      ['branding.title', `${base}/settings/branding`],
      ['nav.legal', `${base}/settings/legal`],
      ['keys.title', `${base}/settings/keys`],
      ['issuer.title', `${base}/settings/issuer`],
      ['webhooks.title', `${base}/webhooks`],
    ]);
    expect(w.get('[aria-current="page"]').text()).toBe('keys.title');
    expect(w.get('[data-testid="org-role"]').text()).toBe('members.role_admin');
  });

  it('leaves People out for someone who cannot read end users', () => {
    setRoute({ path: '/o/acme/projects/web', params: { org: 'acme', project: 'web' } });
    useAuthStore().me = { permissions: ['project:read'] } as never;
    expect(sections(mount(ProjectSidebar, { global: { stubs } }))).not.toContain('nav.group_people');
  });
});

describe('org sidebar', () => {
  it('shows the org pages as one flat group in the design order, with the role', () => {
    setRoute({ path: '/o/acme/settings/members', params: { org: 'acme' } });
    const w = mount(AppSidebar, { global: { stubs } });
    expect(links(w).map(([, href]) => href)).toEqual(['/o/acme', '/o/acme/projects', '/o/acme/settings/members', '/o/acme/settings/api-keys', '/o/acme/settings']);
    expect(sections(w)).toEqual([]);
    expect(w.get('[aria-current="page"]').text()).toBe('members.title');
    expect(w.get('[data-testid="org-role"]').exists()).toBe(true);
    const row = w.get('[data-testid="org-role"]').element.parentElement!;
    expect(row.className).toBe('flex items-center gap-1');
    expect(row.firstElementChild!.className).toBe('min-w-0 flex-1');
  });
});
