// @vitest-environment happy-dom
import { create } from '@bufbuild/protobuf';
import { mount } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import { APIKeySchema } from '#gen/apikey/v1/apikey_pb';
import { WhoamiResponseSchema } from '#gen/auth/v1/auth_pb';
import { WebhookSchema } from '#gen/webhook/v1/webhook_pb';
import { mountPage } from '#test/mount-page';
import { setClient, setRoute } from '#test/nuxt-globals';
import APIKeyList from '~/components/apikey/list/APIKeyList.vue';
import MemberList from '~/components/member/MemberList.vue';
import OrgCreateModal from '~/components/org/create/OrgCreateModal.vue';
import WebhookList from '~/components/webhook/list/WebhookList.vue';
import OrgSettings from '~/pages/o/[org]/settings/index.vue';
import { useOrgsStore } from '~/stores/orgs';

const orgId = 'B6rTn1YkM8wQz3XpL5vGd';
const projectId = 'x6MH9WPWjktMwCBKHy4pF';
const openDialog = () => [...document.querySelectorAll('dialog')].find(d => d.open);

beforeEach(() => {
  setActivePinia(createPinia());
});

describe('org create modal copy', () => {
  it('takes its placeholders and the random-slug help from the locales', () => {
    const w = mount(OrgCreateModal);
    const [name, slug] = w.findAll('input');
    expect(name!.attributes('placeholder')).toBe('org.name_placeholder');
    expect(slug!.attributes('placeholder')).toBe('org.slug_placeholder');
    expect(w.text()).toContain('org.slug_random_help');
    expect(w.text()).not.toContain('generated when empty');
  });
});

describe('member list (O8)', () => {
  it('collapses row actions into a menu below sm, named for the member', () => {
    const w = mount(MemberList, { props: { rows: [{ userId: 'F5pLm2XwQ8kRt3VnZ7bYc', email: 'budi@acme.test', name: 'Budi', role: 'member', initials: 'B', self: false, roles: ['member'], removable: true }] } });
    const menu = w.get('.dropdown');
    expect(menu.classes()).toEqual(expect.arrayContaining(['dropdown-left', 'sm:hidden']));
    expect(menu.get('[role="button"] .sr-only').text()).toBe('Actions for Budi');
    expect(menu.find('[data-icon="lucide:ellipsis"]').exists()).toBe(true);
    expect(w.findAll('.max-sm\\:hidden button').map(b => b.text())).toContain('Remove');
  });
});

describe('member-visible write actions', () => {
  it('shows org API key Revoke only to people who manage keys, behind a confirm naming the key', async () => {
    const keys = { listAPIKeys: vi.fn().mockResolvedValue({ keys: [create(APIKeySchema, { id: 'K9tWq4ZbN2xLm7VcR5pHs', name: 'deploy-bot', kind: 'org', secretHint: 'ab12' })] }) };
    setClient('$apiKeyClient', keys);
    const readOnly = await mountPage(APIKeyList, { pageProps: { orgId } });
    expect(readOnly.findAll('tbody button')).toHaveLength(0);
    readOnly.unmount();
    setActivePinia(createPinia());
    const w = await mountPage(APIKeyList, { pageProps: { orgId, canManage: true }, attachTo: document.body });
    await w.get('tbody button').trigger('click');
    expect(openDialog()!.textContent).toContain('apikeys.revoke_confirm');
    expect(openDialog()!.textContent).toContain('apikeys.revoke_body');
  });

  it('asks before deleting a webhook', async () => {
    setClient('$webhookClient', { listWebhooks: vi.fn().mockResolvedValue({ webhooks: [create(WebhookSchema, { id: 'H3nVb8XqT1wLp6RkZ2mJc', projectId, url: 'https://hooks.acme.test/othra', active: true, eventTypes: [] })] }) });
    const w = await mountPage(WebhookList, { pageProps: { orgId, projectId }, attachTo: document.body });
    const remove = w.findAll('button').find(b => b.text().includes('common.delete'))!;
    await remove.trigger('click');
    expect(openDialog()!.textContent).toContain('webhooks.delete_title');
  });

  it('makes org settings read-only for people without org:write', async () => {
    useOrgsStore().items = [{ id: orgId, slug: 'acme', name: 'Acme', role: 'member', permissions: [] }];
    setRoute({ path: '/o/acme/settings', params: { org: 'acme' } });
    setClient('$authClient', { whoami: vi.fn().mockResolvedValue(create(WhoamiResponseSchema, { permissions: ['org:read'] })) });
    const w = await mountPage(OrgSettings);
    expect(w.find('form').exists()).toBe(false);
    expect(w.find('dl').exists()).toBe(true);
    expect(w.text()).toContain('settings.read_only');
  });
});
