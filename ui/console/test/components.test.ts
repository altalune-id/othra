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
import ClientList from '~/components/client/ClientList.vue';
import MemberList from '~/components/member/MemberList.vue';
import OrgCreateModal from '~/components/org/create/OrgCreateModal.vue';
import OrgFields from '~/components/org/create/OrgFields.vue';
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

describe('org fields', () => {
  it('names each input by its fieldset legend and shows its error', () => {
    const w = mount(OrgFields, { props: { name: '', slug: '', fieldErrors: { slug: 'taken' } } });
    const legends = w.findAll('fieldset > legend.fieldset-legend');
    expect(legends.map(l => l.text())).toEqual(['common.name', 'common.slug']);
    const [name, slug] = w.findAll('input');
    expect(name!.attributes('aria-labelledby')).toBe(legends[0]!.attributes('id'));
    expect(slug!.attributes('aria-labelledby')).toBe(legends[1]!.attributes('id'));
    expect(slug!.attributes('aria-invalid')).toBe('true');
    expect(w.text()).toContain('taken');
  });

  it('is the same block in the modal and on onboarding', () => {
    expect(mount(OrgCreateModal).findComponent(OrgFields).exists()).toBe(true);
  });
});

describe('client list', () => {
  it('marks the console client with the system badge', () => {
    const row = { name: 'App', kind: 'Web app', created: 'Oct 20, 2026', to: '#' };
    const w = mount(ClientList, { props: { rows: [{ ...row, id: 'Lq3vT8nWx2KpZr6YbM9cD', system: true }, { ...row, id: 'dPm7fmQRJKPVw8PxrgdrN' }] } });
    const cells = w.findAll('tbody tr').map(r => r.find('td').text());
    expect(cells).toEqual(['App System', 'App']);
  });
});

describe('member list', () => {
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

  it('keeps the system org read-only, even for owners', async () => {
    useOrgsStore().items = [{ id: orgId, slug: 'othra', name: 'Othra', role: 'owner', permissions: [], system: true }];
    setRoute({ path: '/o/othra/settings', params: { org: 'othra' } });
    setClient('$authClient', { whoami: vi.fn().mockResolvedValue(create(WhoamiResponseSchema, { permissions: ['org:read', 'org:write'] })) });
    const w = await mountPage(OrgSettings);
    expect(w.find('form').exists()).toBe(false);
    expect(w.find('[data-testid="org-settings-readonly"]').exists()).toBe(true);
    expect(w.text()).toContain('org.system_note');
    expect(w.text()).not.toContain('settings.read_only');
  });
});
