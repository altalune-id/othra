// @vitest-environment happy-dom
import type { VueWrapper } from '@vue/test-utils';
import { create } from '@bufbuild/protobuf';
import { flushPromises, mount } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import AppModal from '#base/components/ui/AppModal.vue';
import { APIKeySchema } from '#gen/apikey/v1/apikey_pb';
import { WebhookSchema } from '#gen/webhook/v1/webhook_pb';
import { mountPage } from '#test/mount-page';
import { setClient } from '#test/nuxt-globals';
import APIKeyCreateModal from '~/components/apikey/create/APIKeyCreateModal.vue';
import InviteModal from '~/components/member/invite/InviteModal.vue';
import WebhookCreateModal from '~/components/webhook/create/WebhookCreateModal.vue';
import WebhookList from '~/components/webhook/list/WebhookList.vue';

const orgId = 'V3StGXR8Z5jdHj6BmyT2a';
const projectId = 'x6MH9WPWjktMwCBKHy4pF';
const hook = create(WebhookSchema, { id: 'UxVy4UK3vmrHzPZ273hCA', orgId, projectId, url: 'https://a.test', eventTypes: ['invite.created'], active: true });

beforeEach(() => {
  setActivePinia(createPinia());
});

function openModal(w: VueWrapper<any>) {
  const modal = w.findAllComponents(AppModal).find(m => m.get('dialog').element.open);
  if (!modal) {
    throw new Error('no dialog is open');
  }
  return modal;
}

async function click(w: VueWrapper<any>, label: string) {
  const button = w.findAll('button').find(b => b.text().includes(label));
  if (!button) {
    throw new Error(`no ${label} button`);
  }
  await button.trigger('click');
  await flushPromises();
}

async function expectEscapeKeeps(w: VueWrapper<any>, secret: string) {
  const modal = openModal(w);
  expect(modal.html()).toContain(secret);
  await modal.get('dialog').trigger('cancel');
  await flushPromises();
  expect(modal.emitted('close')).toBeUndefined();
  expect(modal.get('dialog').element.open).toBe(true);
  expect(modal.html()).toContain(secret);
  expect(modal.find('form.modal-backdrop').exists()).toBe(false);
}

async function expectEscapeCloses(w: VueWrapper<any>) {
  const modal = openModal(w);
  await modal.get('dialog').trigger('cancel');
  await flushPromises();
  expect(modal.emitted('close')).toHaveLength(1);
}

describe('api key create dialog', () => {
  const secret = 'othra_ak_secret';

  async function opened() {
    const createAPIKey = vi.fn().mockResolvedValue({ key: create(APIKeySchema, { id: 'hX4tBn9kWq2Lz7PvR3mYc', name: 'deploy' }), plaintext: secret });
    setClient('$apiKeyClient', { createAPIKey });
    const w = mount(APIKeyCreateModal, { props: { orgId }, attachTo: document.body });
    await click(w, 'apikeys.new');
    return w;
  }

  it('closes on Escape while the form shows', async () => {
    await expectEscapeCloses(await opened());
  });

  it('holds on Escape while the key shows, and closes on Done', async () => {
    const w = await opened();
    await w.get('input[type="text"]').setValue('deploy');
    await w.get('form').trigger('submit');
    await flushPromises();
    await expectEscapeKeeps(w, secret);
    const dialog = openModal(w).get('dialog');
    await click(w, 'apikeys.done');
    expect(dialog.element.open).toBe(false);
  });
});

describe('webhook create dialog', () => {
  const secret = 'whsec_create';

  async function opened() {
    setClient('$webhookClient', { createWebhook: vi.fn().mockResolvedValue({ webhook: hook, secret }), pingWebhook: vi.fn() });
    const w = mount(WebhookCreateModal, { props: { orgId, projectId }, attachTo: document.body });
    await click(w, 'webhooks.new');
    return w;
  }

  it('closes on Escape while the form shows', async () => {
    await expectEscapeCloses(await opened());
  });

  it('holds on Escape while the signing secret shows, and closes on Done', async () => {
    const w = await opened();
    await w.get('input[type="url"]').setValue('https://a.test');
    await w.get('input[type="checkbox"]').setValue(true);
    await w.get('form').trigger('submit');
    await flushPromises();
    await expectEscapeKeeps(w, secret);
    const dialog = openModal(w).get('dialog');
    await click(w, 'common.done');
    expect(dialog.element.open).toBe(false);
  });
});

describe('webhook rotate dialog', () => {
  const secret = 'whsec_rotated';

  it('holds on Escape while the rotated secret shows, and closes on Done', async () => {
    setClient('$webhookClient', { listWebhooks: vi.fn().mockResolvedValue({ webhooks: [hook] }), rotateWebhookSecret: vi.fn().mockResolvedValue({ secret }) });
    const w = await mountPage(WebhookList, { pageProps: { orgId, projectId }, attachTo: document.body, global: { stubs: { NuxtLink: true } } });
    await click(w, 'webhooks.rotate');
    await expectEscapeKeeps(w, secret);
    const dialog = openModal(w).get('dialog');
    await click(w, 'common.done');
    expect(dialog.element.open).toBe(false);
  });
});

describe('invite dialog', () => {
  const link = 'https://id.acme.test/invite/secret-link';

  it('closes on Escape while the form shows', async () => {
    await expectEscapeCloses(mount(InviteModal, { props: { open: true, roles: ['member'] }, attachTo: document.body }));
  });

  it('holds on Escape while the link shows, and asks to close on Done', async () => {
    const w = mount(InviteModal, { props: { open: true, roles: ['member'], sentLink: link }, attachTo: document.body });
    await expectEscapeKeeps(w, link);
    await click(w, 'Done');
    expect(w.emitted('close')).toHaveLength(1);
  });
});
