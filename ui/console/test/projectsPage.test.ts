// @vitest-environment happy-dom
import { create } from '@bufbuild/protobuf';
import { createPinia, setActivePinia } from 'pinia';
import { describe, expect, it, vi } from 'vitest';

import { WhoamiResponseSchema } from '#gen/auth/v1/auth_pb';
import { ProjectEnvironment } from '#gen/project/v1/project_pb';
import { mountPage } from '#test/mount-page';
import { setClient, setRoute } from '#test/nuxt-globals';
import Projects from '~/pages/o/[org]/projects/index.vue';
import { useOrgsStore } from '~/stores/orgs';
import { useProjectsStore } from '~/stores/projects';

describe('projects page', () => {
  it('lists projects in the design table: link name, mono slug, environment badge, mono id and a named row menu', async () => {
    setActivePinia(createPinia());
    const orgId = 'B6rTn1YkM8wQz3XpL5vGd';
    useOrgsStore().items = [{ id: orgId, slug: 'acme', name: 'Acme', role: 'owner', permissions: [] }];
    useProjectsStore().items = [{ id: 'x6MH9WPWjktMwCBKHy4pF', orgId, slug: 'acme-prod', name: 'Acme production', environment: ProjectEnvironment.PRODUCTION }] as never;
    setRoute({ path: '/o/acme/projects', params: { org: 'acme' } });
    setClient('$authClient', { whoami: vi.fn().mockResolvedValue(create(WhoamiResponseSchema, { permissions: ['project:*'] })) });
    const w = await mountPage(Projects);
    expect(w.get('h1').text()).toBe('project.title');
    expect(w.get('header button').classes()).toEqual(expect.arrayContaining(['btn', 'btn-primary', 'btn-sm', 'text-ui']));
    const cells = w.findAll('tbody td');
    expect(cells[0]!.get('a').classes()).toEqual(expect.arrayContaining(['link', 'link-hover', 'font-medium']));
    expect(cells[1]!.classes()).toEqual(['font-mono', 'text-xs']);
    expect(cells[2]!.get('.badge').text()).toBe('environment.production');
    expect(cells[3]!.get('span').attributes('title')).toBe('x6MH9WPWjktMwCBKHy4pF');
    expect(cells[4]!.get('button[popovertarget] .sr-only').text()).toBe('project.more_for|{"name":"Acme production"}');
    expect(cells[4]!.findAll('li').map(li => li.text())).toEqual(['common.open', 'common.edit', 'project.copy_id']);
  });
});
