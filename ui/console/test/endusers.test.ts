// @vitest-environment happy-dom
import { create } from '@bufbuild/protobuf';
import { timestampFromDate } from '@bufbuild/protobuf/wkt';
import { createPinia, setActivePinia } from 'pinia';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import { ConnectedAppSchema, PersonalTokenSchema, PersonalTokenState, SessionSchema, SignInMethodKind, SignInMethodSchema } from '#gen/account/v1/account_pb';
import { WhoamiResponseSchema } from '#gen/auth/v1/auth_pb';
import { EndUserSchema } from '#gen/enduser/v1/enduser_pb';
import { mountPage } from '#test/mount-page';
import { connectError, setClient, setKnownKeys, setRoute, useHead } from '#test/nuxt-globals';
import App from '~/app.vue';
import ProjectSidebar from '~/components/project/ProjectSidebar.vue';
import EndUserDetail from '~/pages/o/[org]/projects/[project]/end-users/[user].vue';
import EndUserList from '~/pages/o/[org]/projects/[project]/end-users/index.vue';
import { useOrgsStore } from '~/stores/orgs';
import { useProjectsStore } from '~/stores/projects';

const projectId = 'x6MH9WPWjktMwCBKHy4pF';
const endUserId = 'F5pLm2XwQ8kRt3VnZ7bYc';
const at = timestampFromDate(new Date('2026-10-01T00:00:00Z'));
const ana = create(EndUserSchema, { id: endUserId, email: 'ana@acme.test', name: 'Ana', createdAt: at });

let endUsers: Record<string, ReturnType<typeof vi.fn>>;

function signedIn(permissions: string[]) {
  setClient('$authClient', { whoami: vi.fn().mockResolvedValue(create(WhoamiResponseSchema, { permissions })) });
}

beforeEach(() => {
  setActivePinia(createPinia());
  useOrgsStore().items = [{ id: 'B6rTn1YkM8wQz3XpL5vGd', slug: 'acme', name: 'Acme', role: 'owner', permissions: [] }];
  useProjectsStore().items = [{ id: projectId, orgId: 'B6rTn1YkM8wQz3XpL5vGd', slug: 'store', name: 'Store' }];
  setRoute({ path: '/o/acme/projects/store/end-users', params: { org: 'acme', project: 'store', user: endUserId } });
  signedIn(['end_user:read', 'end_user:revoke']);
  endUsers = {
    listEndUsers: vi.fn().mockResolvedValue({ endUsers: [ana], next: '' }),
    getEndUser: vi.fn().mockResolvedValue({
      endUser: ana,
      methods: [
        create(SignInMethodSchema, { kind: SignInMethodKind.EMAIL_LINK, linked: true }),
        create(SignInMethodSchema, { kind: SignInMethodKind.GOOGLE, linked: false }),
      ],
      sessions: [create(SessionSchema, { id: 'Q7mZp2LkV9xRt4NbW8cYd', userAgent: 'Mozilla/5.0 (Macintosh) Chrome/130.0', authTime: at, lastSeenAt: at })],
      apps: [create(ConnectedAppSchema, { clientId: 'dPm7fmQRJKPVw8PxrgdrN', name: 'Notes CLI', connectedAt: at, resources: [] })],
      tokens: [create(PersonalTokenSchema, { id: 'K9tWq4ZbN2xLm7VcR5pHs', name: 'cli', hint: 'ab12', state: PersonalTokenState.ACTIVE })],
    }),
    revokeEndUserToken: vi.fn().mockResolvedValue({}),
  };
  setClient('$endUserClient', endUsers);
});

describe('console shell', () => {
  it('sets <html lang> from the locale', async () => {
    await mountPage(App, { global: { stubs: { NuxtLayout: { template: '<slot />' }, NuxtPage: true } } });
    const head = useHead.mock.calls[0]![0] as { htmlAttrs: { lang: { value: string } } };
    expect(head.htmlAttrs.lang.value).toBe('en-US');
  });
});

describe('project sidebar', () => {
  it('shows End users only to people who may read them', async () => {
    const labels = async () => {
      const w = await mountPage(ProjectSidebar);
      const shown = w.findAll('nav button').map(b => b.text());
      w.unmount();
      return shown;
    };
    expect(await labels()).toContain('nav.end_users');
    setActivePinia(createPinia());
    useOrgsStore().items = [{ id: 'B6rTn1YkM8wQz3XpL5vGd', slug: 'acme', name: 'Acme', role: 'member', permissions: [] }];
    signedIn(['project:read']);
    const { useAuthStore } = await import('~/stores/auth');
    await useAuthStore().load();
    expect(await labels()).not.toContain('nav.end_users');
  });
});

describe('end users list', () => {
  it('shows a Retry and no table under a load error', async () => {
    setKnownKeys('apperror.GEN900');
    endUsers.listEndUsers!.mockRejectedValueOnce(connectError('GEN900'));
    const w = await mountPage(EndUserList);
    expect(w.get('[role="alert"]').text()).toContain('apperror.GEN900');
    expect(w.find('[data-testid="end-users"]').exists()).toBe(false);
    expect(w.text()).not.toContain('endusers.empty');
  });

  it('keeps the empty message on the shown search while the box changes, with daisyUI 5 row hover', async () => {
    endUsers.listEndUsers!.mockResolvedValueOnce({ endUsers: [], next: '' });
    const w = await mountPage(EndUserList);
    expect(w.text()).toContain('endusers.empty');
    await w.get('input[type="search"]').setValue('budi');
    expect(w.text()).toContain('endusers.empty');
    endUsers.listEndUsers!.mockResolvedValueOnce({ endUsers: [ana], next: '' });
    await w.get('form').trigger('submit');
    await new Promise(r => setTimeout(r));
    expect(w.get('[data-testid="end-users"] tbody tr').classes()).toContain('row-hover');
  });
});

describe('end user detail', () => {
  it('shows every section through the shared rows, with the admin\'s sign-in line and copy', async () => {
    const w = await mountPage(EndUserDetail);
    expect(w.text()).toContain('endusers.detail_subtitle_revoke');
    const methods = w.findAll('[data-testid="end-user-methods"] li').map(li => li.text());
    expect(methods[0]).toContain('ana@acme.test');
    expect(methods[1]).toContain('methods.not_connected');
    expect(w.get('[data-testid="end-user-sessions"]').text()).toContain('Chrome');
    expect(w.get('[data-testid="end-user-apps"]').text()).toContain('endusers.signs_in');
    expect(w.text()).toContain('endusers.methods_help');
  });

  it('hides every section under a load error and offers Retry', async () => {
    setKnownKeys('apperror.GEN900');
    endUsers.getEndUser!.mockRejectedValueOnce(connectError('GEN900'));
    const w = await mountPage(EndUserDetail);
    expect(w.findAll('section')).toHaveLength(0);
    expect(w.text()).not.toContain('endusers.sessions_empty');
    await w.get('button.btn-sm').trigger('click');
    await new Promise(r => setTimeout(r));
    expect(w.findAll('section').length).toBeGreaterThan(0);
  });

  it('turns a missing end user into a 404', async () => {
    setKnownKeys('apperror.EUS001');
    endUsers.getEndUser!.mockRejectedValueOnce(connectError('EUS001'));
    const errors: unknown[] = [];
    const w = await mountPage(EndUserDetail, { global: { config: { errorHandler: e => errors.push(e) } } });
    expect(errors).toContainEqual(expect.objectContaining({ statusCode: 404 }));
    w.unmount();
  });

  it('offers revokes only to people who may revoke', async () => {
    signedIn(['end_user:read']);
    const w = await mountPage(EndUserDetail);
    expect(w.text()).toContain('endusers.detail_subtitle_read');
    for (const list of ['end-user-sessions', 'end-user-apps', 'end-user-tokens']) {
      expect(w.get(`[data-testid="${list}"]`).find('button').exists(), list).toBe(false);
    }
  });

  it('revokes a token behind a confirm and dates the revoke at once', async () => {
    const w = await mountPage(EndUserDetail, { attachTo: document.body });
    await w.get('[data-testid="end-user-tokens"] button').trigger('click');
    const dialog = [...document.querySelectorAll('dialog')].find(d => d.open)!;
    expect(dialog.textContent).toContain('tokens.revoke_body');
    (dialog.querySelectorAll('.modal-action button')[1] as HTMLButtonElement).click();
    await new Promise(r => setTimeout(r));
    expect(endUsers.revokeEndUserToken).toHaveBeenCalledWith({ projectId, endUserId, tokenId: 'K9tWq4ZbN2xLm7VcR5pHs' });
    const row = w.get('[data-testid="end-user-tokens"] li').text();
    expect(row).toContain('tokens.status_revoked');
    expect(row).toMatch(/tokens\.revoked_on\|\{"date":"[A-Z][a-z]{2} \d{1,2}, \d{4}"\}/);
    w.unmount();
  });
});
