// @vitest-environment happy-dom
import type { Component } from 'vue';
import { create } from '@bufbuild/protobuf';
import { createPinia, setActivePinia } from 'pinia';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { WhoamiResponseSchema } from '#gen/auth/v1/auth_pb';
import { mountPage } from '#test/mount-page';
import { connectError, setClient, setRoute } from '#test/nuxt-globals';
import Branding from '~/pages/o/[org]/projects/[project]/settings/branding.vue';
import Legal from '~/pages/o/[org]/projects/[project]/settings/legal.vue';
import { useOrgsStore } from '~/stores/orgs';
import { useProjectsStore } from '~/stores/projects';

const orgId = 'B6rTn1YkM8wQz3XpL5vGd';
const projectId = 'x6MH9WPWjktMwCBKHy4pF';
const project = {
  getProject: vi.fn(),
  updateProjectBranding: vi.fn(),
  updateProjectTerms: vi.fn(),
};

beforeEach(() => {
  setActivePinia(createPinia());
  setRoute({ params: { org: 'acme', project: 'shop' } });
  useOrgsStore().items = [{ id: orgId, slug: 'acme', name: 'Acme', role: 'owner', permissions: [], system: false }];
  useProjectsStore().items = [{ id: projectId, orgId, slug: 'shop', name: 'Shop', system: false }];
  setClient('$authClient', { whoami: vi.fn().mockResolvedValue(create(WhoamiResponseSchema, { permissions: ['project:read', 'project:write'] })) });
  project.getProject.mockReset().mockRejectedValue(connectError('GEN900'));
  project.updateProjectBranding.mockReset().mockResolvedValue({});
  project.updateProjectTerms.mockReset().mockResolvedValue({});
  setClient('$projectClient', project);
});

describe('settings pages after a failed GetProject', () => {
  it.each<[string, Component, () => unknown]>([
    ['branding', Branding, () => project.updateProjectBranding],
    ['legal', Legal, () => project.updateProjectTerms],
  ])('%s fails with a 502 and never saves blanks', async (_, page, update) => {
    const errors: unknown[] = [];
    const w = await mountPage(page, { global: { config: { errorHandler: e => errors.push(e) } } });
    const form = w.find('form');
    if (form.exists()) {
      await form.trigger('submit');
    }
    expect(update()).not.toHaveBeenCalled();
    expect(errors).toContainEqual(expect.objectContaining({ statusCode: 502 }));
    w.unmount();
  });
});
