// @vitest-environment happy-dom
import type { VueWrapper } from '@vue/test-utils';
import { create } from '@bufbuild/protobuf';
import { flushPromises } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import { WhoamiResponseSchema } from '#gen/auth/v1/auth_pb';
import { ClientKind, ClientSchema } from '#gen/client/v1/client_pb';
import { ProjectSchema } from '#gen/project/v1/project_pb';
import { mountPage } from '#test/mount-page';
import { connectError, setClient, setRoute } from '#test/nuxt-globals';
import ClientForm from '~/components/client/ClientForm.vue';
import ClientKindPicker from '~/components/client/ClientKindPicker.vue';
import ClientNew from '~/pages/o/[org]/projects/[project]/clients/new.vue';
import { useOrgsStore } from '~/stores/orgs';
import { useProjectsStore } from '~/stores/projects';

const orgId = 'B6rTn1YkM8wQz3XpL5vGd';
const projectId = 'x6MH9WPWjktMwCBKHy4pF';
const issuer = 'https://id.acme.test/p/web';
const web = create(ClientSchema, { id: 'dPm7fmQRJKPVw8PxrgdrN', projectId, name: 'Store web', kind: ClientKind.WEB, redirectUris: ['https://store.example/cb'] });
const cli = create(ClientSchema, { id: 'H9MspUj32ux8pfYKYsu1a', projectId, name: 'Store CLI', kind: ClientKind.CLI, redirectUris: ['http://127.0.0.1/callback'] });
let generateClientSecret: ReturnType<typeof vi.fn>;
let createClient: ReturnType<typeof vi.fn>;

beforeEach(() => {
  setActivePinia(createPinia());
  vi.stubGlobal('onBeforeRouteLeave', vi.fn());
  useOrgsStore().items = [{ id: orgId, slug: 'acme', name: 'Acme', role: 'owner', permissions: [] }];
  useProjectsStore().items = [{ id: projectId, orgId, slug: 'web', name: 'Web' }];
  setClient('$authClient', { whoami: vi.fn().mockResolvedValue(create(WhoamiResponseSchema, { permissions: ['project:read', 'project:write'] })) });
  createClient = vi.fn().mockResolvedValue({ client: web });
  generateClientSecret = vi.fn().mockRejectedValueOnce(connectError('GEN001')).mockResolvedValue({ client: web, secret: 'othra_cs_x' });
  setClient('$clientClient', { createClient, generateClientSecret });
  setClient('$projectClient', { getProject: vi.fn().mockResolvedValue({ project: create(ProjectSchema, { id: projectId, orgId, issuerUrl: issuer }) }) });
  setRoute({ path: '/o/acme/projects/web/clients/new', params: { org: 'acme', project: 'web' } });
});

async function created(kind?: 'cli') {
  const w = await mountPage(ClientNew);
  expect(w.find('[data-testid="type-step"]').exists()).toBe(true);
  if (kind) {
    w.findComponent(ClientKindPicker).vm.$emit('update:modelValue', kind);
    await flushPromises();
  }
  await w.findAll('button').find(b => b.text().includes('clients.continue_details'))!.trigger('click');
  await flushPromises();
  const form = w.findComponent(ClientForm);
  expect(form.props('kindChosen')).toBe(true);
  form.vm.$emit('update:fields', { ...form.props('fields'), name: 'Store', redirectUris: [kind ? 'http://127.0.0.1/callback' : 'https://store.example/cb'] });
  await flushPromises();
  form.vm.$emit('submit');
  await flushPromises();
  return w;
}

const button = (w: VueWrapper<any>, label: string) => w.findAll('button').find(b => b.text().includes(label))!;

describe('new client: secret then quickstart', () => {
  it('shows a failed first secret with Generate again instead of moving on silently', async () => {
    const w = await created();
    const step = w.get('[data-testid="secret-step"]');
    expect(step.findAll('[role="alert"]')).toHaveLength(1);
    expect(step.findAll('[role="alert"] [role="alert"]')).toHaveLength(0);
    expect(button(w, 'clients.continue_quickstart').attributes('disabled')).toBeDefined();
    await button(w, 'clients.generate_again').trigger('click');
    await flushPromises();
    expect(generateClientSecret).toHaveBeenCalledTimes(2);
    expect(w.text()).toContain('othra_cs_x');
  });

  it('opens the quickstart with the real values once the secret is stored, the secret in the .env', async () => {
    generateClientSecret.mockReset().mockResolvedValue({ client: web, secret: 'othra_cs_x' });
    const w = await created();
    expect(button(w, 'clients.continue_quickstart').attributes('disabled')).toBeDefined();
    await w.get('input[type="checkbox"]').setValue(true);
    await button(w, 'clients.continue_quickstart').trigger('click');
    await flushPromises();
    const env = w.get('[data-testid="quickstart-env"]').text();
    expect(env).toContain(`OTHRA_ISSUER=${issuer}`);
    expect(env).toContain(`OTHRA_CLIENT_ID=${web.id}`);
    expect(env).toContain('OTHRA_REDIRECT_URI=https://store.example/cb');
    expect(env).toContain('OTHRA_CLIENT_SECRET=othra_cs_x');
    const open = vi.fn();
    vi.stubGlobal('open', open);
    await button(w, 'clients.try_signin').trigger('click');
    await vi.waitFor(() => expect(open).toHaveBeenCalled());
    expect(open.mock.calls[0]![0]).toMatch(new RegExp(`^${issuer}/oauth2/authorize\\?`));
  });

  it('starts on Type, and Change goes back to it from Details', async () => {
    const w = await mountPage(ClientNew);
    await w.findAll('button').find(b => b.text().includes('clients.continue_details'))!.trigger('click');
    await flushPromises();
    expect(w.text()).toContain('clients.type_line');
    await w.findAll('button').find(b => b.text().includes('clients.change_type'))!.trigger('click');
    await flushPromises();
    expect(w.find('[data-testid="type-step"]').exists()).toBe(true);
  });

  it('goes straight to the quickstart for a public client, with no secret anywhere', async () => {
    createClient.mockResolvedValue({ client: cli });
    const w = await created('cli');
    expect(w.find('[data-testid="secret-step"]').exists()).toBe(false);
    expect(generateClientSecret).not.toHaveBeenCalled();
    expect(w.get('[data-testid="quickstart-env"]').text()).not.toContain('SECRET');
  });
});
