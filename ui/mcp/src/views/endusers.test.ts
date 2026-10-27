// @vitest-environment happy-dom
import type { ListDescriptor } from '../generic/descriptor';
import { flushPromises, mount } from '@vue/test-utils';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { ref } from 'vue';
import ListView from '../generic/ListView.vue';
import { descriptors } from './descriptors';
import EndUserView from './EndUserView.vue';
import { resolveView } from './index';

const ana = { id: 'F5pLm2XwQ8kRt3VnZ7bYc', email: 'ana@acme.test', createdAt: '2026-10-01T00:00:00Z' };
const full = {
  endUser: ana,
  methods: [{ kind: 'SIGN_IN_METHOD_KIND_EMAIL_LINK', linked: true, email: 'ana@acme.test' }, { kind: 'SIGN_IN_METHOD_KIND_GOOGLE' }],
  sessions: [{ id: 'Q7mZp2LkV9xRt4NbW8cYd', method: 'SIGN_IN_METHOD_KIND_EMAIL_LINK', userAgent: 'Chrome on macOS', authTime: '2026-10-27T01:00:00Z' }],
  apps: [{ clientId: 'dPm7fmQRJKPVw8PxrgdrN', name: 'Notes CLI', connectedAt: '2026-10-02T00:00:00Z' }],
  tokens: [
    { id: 'K9tWq4ZbN2xLm7VcR5pHs', name: 'cli', hint: 'ab12', state: 'PERSONAL_TOKEN_STATE_ACTIVE', server: { name: 'Notes MCP' } },
    { id: 'R2tWq4ZbN2xLm7VcR5pHs', name: 'old', hint: 'cd34', state: 'PERSONAL_TOKEN_STATE_REVOKED' },
  ],
};

function mountUser(callServerTool = vi.fn()) {
  return mount(EndUserView, {
    props: { data: full },
    global: { provide: { mcpApp: { callServerTool }, mcpInput: ref({ projectId: 'p1', endUserId: ana.id }) } },
    attachTo: document.body,
  });
}

const dialog = () => [...document.querySelectorAll('dialog')].find(d => d.open);

afterEach(() => {
  document.body.innerHTML = '';
});

describe('end user views', () => {
  it('routes end_user_get to its own view', () => {
    expect(resolveView('end_user_get').component).toBe(EndUserView);
    expect(resolveView('end_user_list').component).toBe(ListView);
  });

  it('shows the person and their methods, sessions, apps and tokens', () => {
    const w = mountUser();
    expect(w.find('h1').text()).toBe('ana@acme.test');
    expect(w.findAll('h2').map(h => h.text())).toEqual(['Sign-in methods', 'Sessions', 'Connected apps', 'Personal tokens']);
    expect(w.text()).toContain('Chrome on macOS');
    expect(w.text()).toContain('…ab12');
    expect(w.text()).toContain('Notes MCP');
    const tokenRows = w.findAll('section')[3]!.findAll('tbody tr');
    expect(tokenRows[0]!.find('button').text()).toBe('Revoke');
    expect(tokenRows[1]!.find('button').exists()).toBe(false);
    w.unmount();
  });

  it('names the row on each repeated action button', () => {
    const w = mountUser();
    expect(w.findAll('section')[3]!.find('tbody button').attributes('aria-label')).toBe('Revoke: cli');
    expect(w.findAll('section')[1]!.find('tbody button').attributes('aria-label')).toBe('Sign out: Email link');
    w.unmount();
  });

  it('puts focus on the section heading after a revoke', async () => {
    const callServerTool = vi.fn().mockResolvedValue({ structuredContent: full });
    const w = mountUser(callServerTool);
    await w.findAll('section')[3]!.find('tbody button').trigger('click');
    [...dialog()!.querySelectorAll('button')].find(b => b.textContent?.trim() === 'Revoke')!.click();
    await flushPromises();
    expect(document.activeElement?.textContent?.trim()).toBe('Personal tokens');
    w.unmount();
  });

  it('follows a second result for another person', async () => {
    const w = mountUser();
    await w.setProps({ data: { endUser: { ...ana, email: 'bo@acme.test' } } });
    expect(w.find('h1').text()).toBe('bo@acme.test');
    w.unmount();
  });

  it('signs a session out only after the confirm, then reloads the person', async () => {
    const callServerTool = vi.fn().mockResolvedValueOnce({ structuredContent: {} }).mockResolvedValue({ structuredContent: { ...full, sessions: [] } });
    const w = mountUser(callServerTool);
    await w.findAll('section')[1]!.find('tbody button').trigger('click');
    expect(dialog()?.textContent).toContain('Apps signed in through it lose access');
    expect(callServerTool).not.toHaveBeenCalled();
    [...dialog()!.querySelectorAll('button')].find(b => b.textContent?.trim() === 'Sign out')!.click();
    await flushPromises();
    expect(callServerTool).toHaveBeenNthCalledWith(1, { name: 'end_user_session_revoke', arguments: { projectId: 'p1', endUserId: ana.id, sessionId: 'Q7mZp2LkV9xRt4NbW8cYd' } });
    expect(callServerTool).toHaveBeenNthCalledWith(2, { name: 'end_user_get', arguments: { projectId: 'p1', endUserId: ana.id } });
    expect(w.text()).toContain('No live sessions.');
    w.unmount();
  });

  it('disconnects an app and revokes a token with their own tool and arguments', async () => {
    const callServerTool = vi.fn().mockResolvedValue({ structuredContent: full });
    const w = mountUser(callServerTool);
    await w.findAll('section')[2]!.find('tbody button').trigger('click');
    expect(dialog()?.textContent).toContain('Notes CLI');
    [...dialog()!.querySelectorAll('button')].find(b => b.textContent?.trim() === 'Disconnect')!.click();
    await flushPromises();
    expect(callServerTool).toHaveBeenCalledWith({ name: 'end_user_app_disconnect', arguments: { projectId: 'p1', endUserId: ana.id, clientId: 'dPm7fmQRJKPVw8PxrgdrN' } });
    await w.findAll('section')[3]!.find('tbody button').trigger('click');
    [...dialog()!.querySelectorAll('button')].find(b => b.textContent?.trim() === 'Revoke')!.click();
    await flushPromises();
    expect(callServerTool).toHaveBeenCalledWith({ name: 'end_user_token_revoke', arguments: { projectId: 'p1', endUserId: ana.id, tokenId: 'K9tWq4ZbN2xLm7VcR5pHs' } });
    w.unmount();
  });

  it('keeps the revoke dialog open with the refusal in words, not the JSON envelope', async () => {
    const callServerTool = vi.fn().mockResolvedValue({ isError: true, content: [{ type: 'text', text: '{"code":"ACC004","message":"That token no longer exists","request_id":"r1"}' }] });
    const w = mountUser(callServerTool);
    await w.findAll('section')[3]!.find('tbody button').trigger('click');
    [...dialog()!.querySelectorAll('button')].find(b => b.textContent?.trim() === 'Revoke')!.click();
    await flushPromises();
    expect(dialog()?.textContent).toContain('That token no longer exists. (ACC004)');
    expect(dialog()?.textContent).not.toContain('"code"');
    w.unmount();
  });

  it('shows a refused reload', async () => {
    const callServerTool = vi.fn().mockResolvedValueOnce({ structuredContent: {} }).mockResolvedValue({ isError: true, content: [{ type: 'text', text: 'END001 not found' }] });
    const w = mountUser(callServerTool);
    await w.findAll('section')[1]!.find('tbody button').trigger('click');
    [...dialog()!.querySelectorAll('button')].find(b => b.textContent?.trim() === 'Sign out')!.click();
    await flushPromises();
    expect(w.text()).toContain('END001 not found');
    w.unmount();
  });

  it('loads the next page of end users after the cursor, and stops when there is none', async () => {
    const bo = { id: 'B2pLm2XwQ8kRt3VnZ7bYc', email: 'bo@acme.test' };
    const callServerTool = vi.fn().mockResolvedValue({ structuredContent: { endUsers: [bo] } });
    const w = mount(ListView, { props: { data: { endUsers: [ana], next: ana.id }, descriptor: descriptors.end_user_list as ListDescriptor }, global: { provide: { mcpApp: { callServerTool }, mcpInput: ref({ projectId: 'p1', query: 'acme' }) } } });
    await w.findAll('button').find(b => b.text() === 'Load more')!.trigger('click');
    await flushPromises();
    expect(callServerTool).toHaveBeenCalledWith({ name: 'end_user_list', arguments: { projectId: 'p1', query: 'acme', after: ana.id } });
    expect(w.findAll('tbody tr')).toHaveLength(2);
    expect(w.findAll('button').some(b => b.text() === 'Load more')).toBe(false);
  });

  it('lists end users and says when there are none', () => {
    const w = mount(ListView, { props: { data: { endUsers: [ana] }, descriptor: descriptors.end_user_list as ListDescriptor }, global: { provide: { mcpApp: { callServerTool: vi.fn() }, mcpInput: ref({}) } } });
    expect(w.text()).toContain('ana@acme.test');
    expect(mount(ListView, { props: { data: {}, descriptor: descriptors.end_user_list as ListDescriptor }, global: { provide: { mcpApp: { callServerTool: vi.fn() }, mcpInput: ref({}) } } }).text()).toContain('No end users yet.');
  });

  it('says so when there is no person in the result', () => {
    const w = mount(EndUserView, { props: { data: {} }, global: { provide: { mcpApp: { callServerTool: vi.fn() }, mcpInput: ref({}) } } });
    expect(w.text()).toContain('Nothing to show.');
  });
});
