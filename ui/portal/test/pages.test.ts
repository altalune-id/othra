// @vitest-environment happy-dom
import { create } from '@bufbuild/protobuf';
import { timestampFromDate } from '@bufbuild/protobuf/wkt';
import { createPinia, setActivePinia } from 'pinia';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import { AppResourceSchema, ConnectedAppSchema, ProfileSchema, SessionSchema, SignInMethodKind, SignInMethodLock, SignInMethodSchema, TokenServerSchema } from '#gen/account/v1/account_pb';
import { mountPage } from '#test/mount-page';
import { connectError, router, runtimeConfig, setClient, setKnownKeys, setRoute, useHead } from '#test/nuxt-globals';
import appConfig from '~/app.config';
import App from '~/app.vue';
import Layout from '~/layouts/default.vue';
import Apps from '~/pages/apps.vue';
import Sessions from '~/pages/sessions.vue';
import SignIn from '~/pages/sign-in.vue';
import NewToken from '~/pages/tokens/new.vue';

const at = timestampFromDate(new Date('2026-10-01T00:00:00Z'));
const here = create(SessionSchema, { id: 'Q7mZp2LkV9xRt4NbW8cYd', userAgent: 'Mozilla/5.0 (Macintosh) Chrome/130.0', current: true, authTime: at, lastSeenAt: at });
const other = create(SessionSchema, { id: 'H3nVb8XqT1wLp6RkZ2mJc', userAgent: 'Mozilla/5.0 (iPhone) Safari/605.1', authTime: at, lastSeenAt: at });

function app(name: string, consented: boolean) {
  return create(ConnectedAppSchema, {
    clientId: 'dPm7fmQRJKPVw8PxrgdrN',
    name,
    consented,
    connectedAt: at,
    resources: [
      create(AppResourceSchema, { identifier: '', scopes: ['openid', 'email'] }),
      create(AppResourceSchema, { name: 'Notes', identifier: 'https://mcp.acme.test/mcp', scopes: ['notes:read'], scopeDescriptions: { 'notes:read': 'Read your notes' } }),
    ],
  });
}

let client: Record<string, ReturnType<typeof vi.fn>>;

beforeEach(() => {
  setActivePinia(createPinia());
  runtimeConfig.public = { apiUrl: '', project: 'Acme Store', hostedUrl: '/p/acme', termsUrl: '', privacyUrl: '', logoUrl: '' };
  client = {
    getProfile: vi.fn().mockResolvedValue({ profile: create(ProfileSchema, { id: 'F5pLm2XwQ8kRt3VnZ7bYc', email: 'ana@acme.test', name: 'Ana' }), signOutToken: 'tok', currentSessionId: here.id }),
    listSessions: vi.fn().mockResolvedValue({ sessions: [other, here] }),
    listConnectedApps: vi.fn().mockResolvedValue({ apps: [app('Notes CLI', true)] }),
    getSignInMethods: vi.fn().mockResolvedValue({ methods: [], linkToken: 'link' }),
    listTokenServers: vi.fn().mockResolvedValue({ servers: [create(TokenServerSchema, { id: 'dPm7fmQRJKPVw8PxrgdrN', name: 'Notes', identifier: 'https://mcp.acme.test/mcp', scopes: ['notes:read'] })] }),
    createPersonalToken: vi.fn(),
  };
  setClient('$accountClient', client);
});

describe('portal shell', () => {
  it('sets <html lang> from the locale and keeps the portal\'s own theme key', async () => {
    await mountPage(App, { global: { stubs: { NuxtLayout: { template: '<slot />' }, NuxtPage: true } } });
    const head = useHead.mock.calls[0]![0] as { htmlAttrs: { lang: { value: string } } };
    expect(head.htmlAttrs.lang.value).toBe('en-US');
    expect(appConfig.theme).toEqual({ storageKey: 'othra_portal_theme', followSystem: true });
  });

  it('opens the account menu from a focusable trigger and switches account back to the portal base', async () => {
    const w = await mountPage(Layout);
    const trigger = w.findAll('.dropdown [role="button"][aria-haspopup="true"]').find(b => b.text().includes('menu.account'))!;
    expect(trigger.element.tagName).toBe('DIV');
    expect(trigger.attributes('tabindex')).toBe('0');
    expect(trigger.text()).toContain('ana@acme.test');
    expect(router.resolve).toHaveBeenCalledWith('/');
    const menu = trigger.element.parentElement!;
    expect(menu.querySelector('.dropdown-content a')!.getAttribute('href')).toBe('/p/acme/login?add=1&return_to=%2Fbase%2F');
    const form = menu.querySelector('.dropdown-content form')!;
    expect(form.getAttribute('action')).toBe('/p/acme/login/signout');
    expect((form.querySelector('input[name="user_session"]') as HTMLInputElement).value).toBe(here.id);
  });
});

describe('sessions page', () => {
  it('shows a Retry and no list under a load error', async () => {
    setKnownKeys('apperror.GEN900');
    client.listSessions!.mockRejectedValueOnce(connectError('GEN900'));
    const w = await mountPage(Sessions);
    expect(w.get('[role="alert"]').text()).toContain('apperror.GEN900');
    expect(w.find('[data-testid="sessions"]').exists()).toBe(false);
    client.listSessions!.mockResolvedValueOnce({ sessions: [here] });
    await w.get('button.btn-sm').trigger('click');
    await new Promise(r => setTimeout(r));
    expect(w.find('[data-testid="sessions"]').exists()).toBe(true);
  });

  it('lists this device first without a sign-out, and asks before signing out every other session', async () => {
    const w = await mountPage(Sessions, { attachTo: document.body });
    const rows = w.findAll('[data-testid="sessions"] li');
    expect(rows[0]!.text()).toContain('sessions.this_device');
    expect(rows[0]!.find('button').exists()).toBe(false);
    const others = w.findAll('button').find(b => b.text().includes('sessions.sign_out_others'))!;
    expect(others.classes()).toEqual(expect.arrayContaining(['btn-error', 'btn-soft']));
    await others.trigger('click');
    const open = [...document.querySelectorAll('dialog')].find(d => d.open)!;
    expect(open.textContent).toContain('sessions.others_body');
    w.unmount();
  });
});

describe('connected apps page', () => {
  it('shows the sign-in line alongside the server lines, and words the disconnect by consent', async () => {
    client.listConnectedApps!.mockResolvedValueOnce({ apps: [app('Notes CLI', true), { ...app('Sign-in only', false), clientId: 'B6rTn1YkM8wQz3XpL5vGd', resources: [] }] });
    const w = await mountPage(Apps, { attachTo: document.body });
    const [first, second] = w.findAll('[data-testid="app"]');
    const lines = first!.findAll('li').map(l => l.text());
    expect(lines[0]).toBe('apps.sees_email');
    expect(lines.some(l => l.includes('Read your notes'))).toBe(true);
    const dialogText = () => [...document.querySelectorAll('dialog')].find(d => d.open)?.textContent ?? '';
    await first!.get('button').trigger('click');
    expect(dialogText()).toContain('apps.disconnect_body|');
    await second!.get('button').trigger('click');
    expect(dialogText()).toContain('apps.disconnect_body_signin|');
    w.unmount();
  });

  it('shows no empty state under a load error', async () => {
    setKnownKeys('apperror.GEN900');
    client.listConnectedApps!.mockRejectedValueOnce(connectError('GEN900'));
    const w = await mountPage(Apps);
    expect(w.text()).not.toContain('apps.empty');
  });
});

describe('sign-in methods page', () => {
  it('locks the only method: the disconnect is aria-disabled and opens nothing', async () => {
    client.getSignInMethods!.mockResolvedValueOnce({
      linkToken: 'link',
      methods: [
        create(SignInMethodSchema, { kind: SignInMethodKind.GOOGLE, linked: true, identityId: 'J2xKv7NqP4wRt9LmZ6cBh', email: 'ana@gmail.com', linkedAt: at, lock: SignInMethodLock.LAST_METHOD }),
      ],
    });
    const w = await mountPage(SignIn, { attachTo: document.body });
    const row = w.get('[data-testid="methods"] li');
    expect(row.text()).toContain('ana@gmail.com');
    expect(row.text()).toContain('methods.only_method');
    const button = row.get('button');
    expect(button.attributes('aria-disabled')).toBe('true');
    await button.trigger('click');
    expect([...document.querySelectorAll('dialog')].some(d => d.open)).toBe(false);
    w.unmount();
  });

  it('shows the connected notice once and drops ?linked=1 after mount', async () => {
    setRoute({ path: '/sign-in', query: { linked: '1' } });
    const w = await mountPage(SignIn);
    expect(w.get('.alert').text()).toBe('methods.connected');
    expect(router.replace).toHaveBeenCalledWith({ query: {} });
  });
});

describe('new token page', () => {
  it('sends a fresh sign-in back to this page under the portal base on ACC009', async () => {
    setKnownKeys('apperror.ACC009');
    setRoute({ path: '/tokens/new' });
    client.createPersonalToken!.mockRejectedValueOnce(connectError('ACC009'));
    const w = await mountPage(NewToken);
    await w.get('input:not([type])').setValue('cli');
    await w.get('input[type="checkbox"]').setValue(true);
    await w.get('form').trigger('submit');
    await new Promise(r => setTimeout(r));
    const link = w.findAll('a').find(a => a.text() === 'tokens.reauth')!;
    expect(link.attributes('href')).toBe('/p/acme/login?prompt=login&return_to=%2Fbase%2Ftokens%2Fnew');
  });

  it('reveals the token once with the MCP snippet, then forgets it', async () => {
    client.createPersonalToken!.mockResolvedValueOnce({ secret: 'othra_pt_secret' });
    const w = await mountPage(NewToken);
    await w.get('input:not([type])').setValue('cli');
    await w.get('input[type="checkbox"]').setValue(true);
    await w.get('form').trigger('submit');
    await new Promise(r => setTimeout(r));
    const reveal = w.get('[data-testid="reveal"]');
    expect(reveal.text()).toContain('https://mcp.acme.test/mcp');
    await reveal.findAll('button').at(-1)!.trigger('click');
    await new Promise(r => setTimeout(r));
    expect(w.find('[data-testid="reveal"]').exists()).toBe(false);
  });
});
