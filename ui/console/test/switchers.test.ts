// @vitest-environment happy-dom
import { mount, RouterLinkStub } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import { ProjectEnvironment } from '#gen/project/v1/project_pb';
import { fakeStorage, setRoute } from '#test/nuxt-globals';
import OrgSelector from '~/components/org/OrgSelector.vue';
import ProjectSelector from '~/components/project/ProjectSelector.vue';
import { useAuthStore } from '~/stores/auth';
import { useOrgsStore } from '~/stores/orgs';
import { useProjectsStore } from '~/stores/projects';
import { useRecentStore } from '~/stores/recent';

const orgId = 'B6rTn1YkM8wQz3XpL5vGd';
const names = (w: ReturnType<typeof mount>) => w.findAll('ul.menu li > button').map(b => b.text());

beforeEach(() => {
  setActivePinia(createPinia());
  vi.stubGlobal('localStorage', fakeStorage());
  useAuthStore().me = { permissions: ['project:read'] } as never;
  useOrgsStore().items = [
    { id: orgId, slug: 'acme', name: 'Acme', role: 'admin', permissions: [] },
    { id: 'Z1', slug: 'north', name: 'Northwind Labs', role: 'member', permissions: [] },
    { id: 'Z2', slug: 'globex', name: 'Globex', role: 'member', permissions: [] },
  ];
  useProjectsStore().items = [
    { id: 'P1', orgId, slug: 'acme-prod', name: 'Acme production', environment: ProjectEnvironment.PRODUCTION },
    { id: 'P2', orgId, slug: 'acme-labs', name: 'Acme labs', environment: ProjectEnvironment.LABS },
    { id: 'P3', orgId, slug: 'acme-misc', name: 'Misc' },
  ] as never;
});

describe('org switcher', () => {
  it('names the current org after Othra, lists orgs last opened first and filters them by the search', async () => {
    setRoute({ path: '/o/acme', params: { org: 'acme' } });
    useRecentStore().visit('globex');
    useRecentStore().visit('acme');
    const w = mount(OrgSelector);
    expect(w.get('button[popovertarget]').text()).toContain('Othra/Acme');
    expect(w.get('.menu-title').text()).toBe('nav.last_opened');
    expect(w.get('button[popovertarget]').classes()).toEqual(expect.arrayContaining(['btn', 'btn-ghost', 'btn-sm', 'w-full', 'justify-between', 'text-ui']));
    expect(w.get('[popover]').classes()).toEqual(expect.arrayContaining(['dropdown', 'card', 'card-sm', 'z-1', 'w-64', 'border', 'border-base-300', 'bg-base-100', 'shadow-sm']));
    expect(w.get('[popover]').classes()).not.toContain('p-2');
    expect(w.get('.card-body').classes()).toEqual(['card-body', 'gap-2', 'p-2']);
    expect(names(w)).toEqual(['Acme', 'Globex', 'Northwind Labs']);
    expect(w.get('.menu-active').text()).toBe('Acme');
    await w.get('input[type="search"]').setValue('north');
    expect(names(w)).toEqual(['Northwind Labs']);
  });
});

describe('project switcher', () => {
  it('shows the handle in mono with its environment badge, lists handles last opened first and links the settings', () => {
    setRoute({ path: '/o/acme/projects/acme-prod', params: { org: 'acme', project: 'acme-prod' } });
    useRecentStore().visit('acme', 'acme-labs');
    const w = mount(ProjectSelector, { global: { stubs: { NuxtLink: RouterLinkStub } } });
    const trigger = w.get('button[popovertarget]');
    expect(trigger.get('.font-mono').text()).toBe('acme-prod');
    expect(trigger.get('.badge').text()).toBe('environment.production');
    expect(names(w)).toEqual(['acme-labsenvironment.labs', 'acme-prodenvironment.production', 'acme-misc']);
    expect(w.getComponent(RouterLinkStub).props('to')).toBe('/o/acme/projects/acme-prod/settings');
  });
});
