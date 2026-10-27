// @vitest-environment happy-dom
import { create } from '@bufbuild/protobuf';
import { createPinia, setActivePinia } from 'pinia';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import { WhoamiResponseSchema } from '#gen/auth/v1/auth_pb';
import { mountPage } from '#test/mount-page';
import { navigateTo, setClient } from '#test/nuxt-globals';
import Onboarding from '~/pages/onboarding.vue';
import Orgs from '~/pages/orgs.vue';

const org = { id: 'V3StGXR8Z5jdHj6BmyT2a', slug: 'acme', name: 'Acme' };
const project = { id: 'x6MH9WPWjktMwCBKHy4pF', orgId: org.id, slug: 'default', name: 'Default' };

function setup(orgs: Array<typeof org & { role?: string }>, orgCreation: boolean) {
  setClient('$authClient', { whoami: vi.fn().mockResolvedValue(create(WhoamiResponseSchema, { orgCreation })), setActiveOrg: vi.fn().mockResolvedValue({}), createOwnOrg: vi.fn().mockResolvedValue({ org }) });
  setClient('$orgClient', { listOrgs: vi.fn().mockResolvedValue({ orgs }) });
  setClient('$projectClient', { listProjects: vi.fn().mockResolvedValue({ projects: [project] }) });
}

beforeEach(() => {
  setActivePinia(createPinia());
});

describe('onboarding page', () => {
  it('asks a new cloud user for their first org and creates it', async () => {
    setup([], true);
    const w = await mountPage(Onboarding);
    expect(navigateTo).not.toHaveBeenCalled();
    expect(w.find('h1').text()).toBe('onboarding.title');
    await w.find('input[type="text"]').setValue('Acme');
    await w.find('form').trigger('submit');
    await vi.waitFor(() => expect(navigateTo).toHaveBeenCalledWith('/o/acme/projects/default'));
  });

  it('sends someone who already has an org to it', async () => {
    setup([org], true);
    await mountPage(Onboarding);
    expect(navigateTo).toHaveBeenCalledWith('/o/acme/projects/default', { replace: true });
  });

  it('sends someone who may not create orgs to the org list', async () => {
    setup([], false);
    await mountPage(Onboarding);
    expect(navigateTo).toHaveBeenCalledWith('/orgs', { replace: true });
  });
});

describe('org list with no orgs', () => {
  it('sends a caller who may create one to onboarding, the one first-org path', async () => {
    setup([], true);
    await mountPage(Orgs);
    expect(navigateTo).toHaveBeenCalledWith('/onboarding', { replace: true });
  });

  it('tells a cloud owner why there is no New organization', async () => {
    setup([{ ...org, role: 'owner' }], true);
    const w = await mountPage(Orgs);
    expect(w.find('[data-testid="org-create-owned"]').text()).toBe('org.create_owned');
    expect(w.text()).not.toContain('org.new');
  });

  it('offers New organization to a cloud member who owns none', async () => {
    setup([{ ...org, role: 'admin' }], true);
    const w = await mountPage(Orgs);
    expect(w.find('[data-testid="org-create-owned"]').exists()).toBe(false);
    expect(w.text()).toContain('org.new');
  });

  it('shows the empty state to a caller who may not', async () => {
    setup([], false);
    const w = await mountPage(Orgs);
    expect(navigateTo).not.toHaveBeenCalled();
    expect(w.text()).toContain('org.none_title');
  });
});
