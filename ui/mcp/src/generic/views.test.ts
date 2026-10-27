// @vitest-environment happy-dom
import type { ListDescriptor } from './descriptor';
import { flushPromises, mount } from '@vue/test-utils';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { ref } from 'vue';
import DefaultView from '../views/DefaultView.vue';
import { descriptors } from '../views/descriptors';
import { resolveView } from '../views/index';
import DetailView from './DetailView.vue';
import FieldValue from './FieldValue.vue';
import ListView from './ListView.vue';

const orgs = { orgs: [
  { id: 'V3StGXR8Z5jdHj6BmyT2a', slug: 'acme', name: 'Acme', role: 'owner' },
  { id: 'YeDxg2kMYznPJJG3wVmjj', slug: 'beta', name: 'Beta', role: 'member' },
] };

function withApp(app: Record<string, unknown>, input: Record<string, unknown> = {}) {
  return { global: { provide: { mcpApp: app, mcpInput: ref(input) } }, attachTo: document.body };
}

afterEach(() => {
  document.body.innerHTML = '';
});

describe('resolveView', () => {
  it('gives a described tool its generic view, others their own view or the JSON fallback', () => {
    expect(resolveView('org_list').component).toBe(ListView);
    expect(resolveView('org_list').descriptor).toBe(descriptors.org_list);
    expect(resolveView('project_get').component).toBe(DetailView);
    expect(resolveView('member_invite').descriptor).toBeUndefined();
    expect(resolveView('nope').component).toBe(DefaultView);
  });
});

describe('list view (org_list)', () => {
  it('shows a row per org with its badge and copyable id, and refreshes through the tool', async () => {
    const callServerTool = vi.fn().mockResolvedValue({ structuredContent: { orgs: [orgs.orgs[0]] } });
    const w = mount(ListView, { props: { data: orgs, descriptor: descriptors.org_list as ListDescriptor }, ...withApp({ callServerTool }) });
    expect(w.find('h1').text()).toBe('Organizations');
    expect(w.findAll('tbody tr')).toHaveLength(2);
    expect(w.text()).toContain('Owner');
    expect(w.find('button[aria-label="Copy ID"]').exists()).toBe(true);
    await w.findAll('button').find(b => b.text() === 'Refresh')!.trigger('click');
    await flushPromises();
    expect(callServerTool).toHaveBeenCalledWith({ name: 'org_list', arguments: {} });
    expect(w.findAll('tbody tr')).toHaveLength(1);
  });

  it('shows the refusal when a refresh fails', async () => {
    const callServerTool = vi.fn().mockResolvedValue({ isError: true, content: [{ type: 'text', text: 'denied' }] });
    const w = mount(ListView, { props: { data: orgs, descriptor: descriptors.org_list as ListDescriptor }, ...withApp({ callServerTool }) });
    await w.findAll('button').find(b => b.text() === 'Refresh')!.trigger('click');
    await flushPromises();
    expect(w.text()).toContain('denied');
    expect(w.findAll('tbody tr')).toHaveLength(2);
  });

  it('says so when there are no rows', () => {
    const w = mount(ListView, { props: { data: {}, descriptor: descriptors.org_list as ListDescriptor }, ...withApp({ callServerTool: vi.fn() }) });
    expect(w.text()).toContain('No organizations.');
  });

  it('confirms a row action before calling its tool, then refreshes; a refusal keeps the dialog open', async () => {
    const descriptor: ListDescriptor = {
      ...(descriptors.org_list as ListDescriptor),
      actions: [{
        label: 'Remove',
        tool: 'org_remove',
        args: (row, input) => ({ id: row.id, by: input.by }),
        confirm: { title: () => 'Remove org?', body: row => `Remove ${row.name}.`, button: 'Remove' },
      }],
    };
    const callServerTool = vi.fn()
      .mockResolvedValueOnce({ isError: true, content: [{ type: 'text', text: 'refused' }] })
      .mockResolvedValue({ structuredContent: orgs });
    const w = mount(ListView, { props: { data: orgs, descriptor }, ...withApp({ callServerTool }, { by: 'me' }) });
    await w.findAll('tbody button').find(b => b.text() === 'Remove')!.trigger('click');
    const dialog = () => [...document.querySelectorAll('dialog')].find(d => d.open);
    expect(dialog()?.textContent).toContain('Remove Acme.');
    expect(callServerTool).not.toHaveBeenCalled();
    const confirm = () => [...dialog()!.querySelectorAll('button')].find(b => b.textContent?.trim() === 'Remove')!;
    confirm().click();
    await flushPromises();
    expect(callServerTool).toHaveBeenCalledWith({ name: 'org_remove', arguments: { id: 'V3StGXR8Z5jdHj6BmyT2a', by: 'me' } });
    expect(dialog()?.textContent).toContain('refused');
    confirm().click();
    await flushPromises();
    expect(callServerTool).toHaveBeenLastCalledWith({ name: 'org_list', arguments: {} });
    expect(dialog()).toBeUndefined();
  });
});

describe('detail view (project_get)', () => {
  const project = { project: { id: 'x6MH9WPWjktMwCBKHy4pF', slug: 'web', name: 'Web', issuerUrl: 'https://id.acme.test/p/acme-web', createdAt: '2026-10-27T03:04:05Z' } };

  it('shows the fields it has, hides the empty optional ones, and copies the issuer', () => {
    const w = mount(DetailView, { props: { data: project, descriptor: descriptors.project_get as never }, ...withApp({ callServerTool: vi.fn() }) });
    expect(w.find('h1').text()).toBe('Project');
    const labels = w.findAll('dt').map(d => d.text());
    expect(labels).toContain('Issuer');
    expect(labels).not.toContain('Support email');
    expect(labels).not.toContain('Kind');
    expect(w.text()).toContain('2026-10-27 03:04 UTC');
    expect(w.find('button[aria-label="Copy Issuer"]').exists()).toBe(true);
    expect(w.find('[role="status"]').exists()).toBe(false);
  });

  it('marks a result as done, adds the note, and says so when there is nothing', () => {
    const descriptor = { kind: 'result' as const, title: 'Created', object: 'project', fields: [{ key: 'name', label: 'Name' }], note: () => 'Next, add a client.' };
    const w = mount(DetailView, { props: { data: project, descriptor }, ...withApp({ callServerTool: vi.fn() }) });
    expect(w.find('[role="status"]').text()).toContain('Done');
    expect(w.find('[role="note"]').text()).toBe('Next, add a client.');
    expect(mount(DetailView, { props: { data: {}, descriptor }, ...withApp({}) }).text()).toContain('Nothing to show.');
  });
});

describe('field value', () => {
  it('opens a link through the host, and shows the URL to copy when the host refuses', async () => {
    const openLink = vi.fn().mockResolvedValueOnce({}).mockResolvedValueOnce({ isError: true });
    const field = { key: 'consoleUrl', label: 'Manage in the console', kind: 'link' as const };
    const w = mount(FieldValue, { props: { field, value: 'https://id.acme.test/console/x' }, ...withApp({ callServerTool: vi.fn(), openLink }) });
    await w.get('button').trigger('click');
    await flushPromises();
    expect(openLink).toHaveBeenCalledWith({ url: 'https://id.acme.test/console/x' });
    expect(w.find('code').exists()).toBe(false);
    await w.get('button').trigger('click');
    await flushPromises();
    expect(w.find('code').text()).toBe('https://id.acme.test/console/x');
  });

  it('renders badges with their tone, secrets in a secret field, and mono text', () => {
    const badge = mount(FieldValue, { props: { field: { key: 's', label: 'S', kind: 'badge', badges: { OK: { label: 'Fine', tone: 'success' } } }, value: 'OK' } });
    expect(badge.get('.badge').classes()).toContain('badge-success');
    const neutral = mount(FieldValue, { props: { field: { key: 's', label: 'S', kind: 'badge', badges: { WEB: { label: 'Web app', tone: 'neutral' } } }, value: 'WEB' } });
    expect(neutral.get('.badge').classes()).toEqual(['badge', 'badge-sm', 'badge-outline']);
    const ghost = mount(FieldValue, { props: { field: { key: 's', label: 'S', kind: 'badge' }, value: 'RAW' } });
    expect(ghost.get('.badge').classes()).toContain('badge-ghost');
    expect(ghost.get('.badge').classes()).not.toContain('badge-soft');
    const secret = mount(FieldValue, { props: { field: { key: 'k', label: 'Secret', kind: 'secret' }, value: 's3cret' } });
    expect(secret.text()).toContain('s3cret');
    expect(mount(FieldValue, { props: { field: { key: 'm', label: 'M', kind: 'mono' }, value: 'abc' } }).get('code').text()).toBe('abc');
    expect(mount(FieldValue, { props: { field: { key: 't', label: 'T' }, value: '' } }).text()).toBe('—');
  });
});
