// @vitest-environment happy-dom
import type { Component } from 'vue';
import { create } from '@bufbuild/protobuf';
import { mount } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { defineComponent } from 'vue';

import { WhoamiResponseSchema } from '#gen/auth/v1/auth_pb';
import { ProjectSchema } from '#gen/project/v1/project_pb';
import { mountPage } from '#test/mount-page';
import { nuxtError, setClient, setRoute } from '#test/nuxt-globals';
import AppNavbar from '~/components/layout/AppNavbar.vue';
import OrgShell from '~/pages/o/[org].vue';
import ClientDetail from '~/pages/o/[org]/projects/[project]/clients/[client].vue';
import ResourceServerDetail from '~/pages/o/[org]/projects/[project]/resource-servers/[rs].vue';
import { useOrgsStore } from '~/stores/orgs';
import { useProjectsStore } from '~/stores/projects';

const orgId = 'B6rTn1YkM8wQz3XpL5vGd';
const projectId = 'x6MH9WPWjktMwCBKHy4pF';
const shellStubs = { ProjectSidebar: true, AppSidebar: true, ConsoleTopBar: true };

function thrownBy(page: Component, stubs: Record<string, unknown> = {}) {
  return new Promise((resolve) => {
    mount(defineComponent({
      components: { page },
      errorCaptured: (e) => {
        resolve(e);
        return false;
      },
      template: '<Suspense><page /></Suspense>',
    }), { global: { stubs } });
  });
}

beforeEach(() => {
  setActivePinia(createPinia());
  useOrgsStore().items = [{ id: orgId, slug: 'acme', name: 'Acme', role: 'owner', permissions: [], system: false }];
  useOrgsStore().loaded = true;
  vi.stubGlobal('definePageMeta', vi.fn());
  vi.stubGlobal('onBeforeRouteLeave', vi.fn());
  setClient('$authClient', { whoami: vi.fn().mockResolvedValue(create(WhoamiResponseSchema, { permissions: ['project:read'] })) });
  setRoute({ path: '/o/acme/projects/shop', params: { org: 'acme', project: 'shop' } });
});

describe('errors inside an org', () => {
  it('keep the sidebar and top bar, with the error page in place of the page', async () => {
    setClient('$projectClient', { listProjects: vi.fn().mockResolvedValue({ projects: [] }) });
    const Missing = defineComponent({
      setup() {
        throw Object.assign(new Error('Client not found'), { statusCode: 404, statusMessage: 'Client not found' });
      },
      render: () => null,
    });
    const w = await mountPage(OrgShell, { global: { stubs: { ...shellStubs, NuxtPage: Missing, ProjectSidebar: { template: '<nav data-testid="sidebar" />' } } } });
    expect(w.find('[data-testid="sidebar"]').exists()).toBe(true);
    expect(w.find('console-top-bar-stub').exists()).toBe(true);
    expect(w.get('h1').text()).toBe('error_page.not_found_title');
    expect(w.text()).toContain('Client not found');
    expect(w.findAll('main')).toHaveLength(1);
  });

  it('say a failed project load is a failure with Retry, not "not found"', async () => {
    setClient('$projectClient', { listProjects: vi.fn().mockRejectedValue(new Error('offline')) });
    expect(await thrownBy(OrgShell, { ...shellStubs, NuxtPage: true })).toMatchObject({ statusCode: 502, statusMessage: 'error.load_projects' });
  });
});

describe('a failed detail load', () => {
  beforeEach(() => {
    useProjectsStore().items = [{ id: projectId, orgId, slug: 'shop', name: 'Shop', system: false }];
    setClient('$projectClient', { getProject: vi.fn().mockResolvedValue({ project: create(ProjectSchema, { id: projectId, orgId }) }) });
  });

  it('is a failure on the client page, not "client not found"', async () => {
    setRoute({ path: '/o/acme/projects/shop/clients/c1', params: { org: 'acme', project: 'shop', client: 'c1' } });
    setClient('$clientClient', { getClient: vi.fn().mockRejectedValue(new Error('offline')) });
    expect(await thrownBy(ClientDetail)).toMatchObject({ statusCode: 502, statusMessage: 'error.load_clients' });
  });

  it('is a failure on the resource-server page, not "server not found"', async () => {
    setRoute({ path: '/o/acme/projects/shop/resource-servers/r1', params: { org: 'acme', project: 'shop', rs: 'r1' } });
    setClient('$resourceServerClient', { getResourceServer: vi.fn().mockRejectedValue(new Error('offline')), listLinks: vi.fn().mockResolvedValue({ links: [] }) });
    expect(await thrownBy(ResourceServerDetail)).toMatchObject({ statusCode: 502, statusMessage: 'error.load_rs' });
  });
});

describe('the slim navbar', () => {
  it('shows on an org route when the org itself failed, so the error page keeps a way out', () => {
    setRoute({ path: '/o/acme', params: { org: 'acme' } });
    const stubs = { ThemeFamilySwitcher: true, ThemeSwitcher: true, UserMenu: true };
    expect(mount(AppNavbar, { global: { stubs } }).html()).toBe('<!--v-if-->');
    nuxtError.value = { statusCode: 502 };
    expect(mount(AppNavbar, { global: { stubs } }).find('a').exists()).toBe(true);
  });
});
