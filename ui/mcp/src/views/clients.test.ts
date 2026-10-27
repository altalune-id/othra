// @vitest-environment happy-dom
import type { DetailDescriptor, ListDescriptor } from '../generic/descriptor';
import { mount } from '@vue/test-utils';
import { describe, expect, it, vi } from 'vitest';
import { ref } from 'vue';
import DetailView from '../generic/DetailView.vue';
import ListView from '../generic/ListView.vue';
import { descriptors, secretHints } from './descriptors';

const consoleUrl = 'http://localhost:5150/console/o/acme/projects/web/clients/dPm7fmQRJKPVw8PxrgdrN';
const web = {
  id: 'dPm7fmQRJKPVw8PxrgdrN',
  name: 'Storefront',
  kind: 'CLIENT_KIND_WEB',
  registration: 'CLIENT_REGISTRATION_REGISTERED',
  redirectUris: ['https://store.example/cb', 'https://store.example/cb2'],
  allowedScopes: ['openid', 'email'],
  tokenEndpointAuthMethod: 'client_secret_basic',
  pkceRequired: true,
  consentPolicy: 'CONSENT_POLICY_IMPLICIT',
  secrets: [{ id: 's1', hint: 'ab12' }],
  consoleUrl,
};
const system = { id: 'Lq3vT8nWx2KpZr6YbM9cD', name: 'Othra console', kind: 'CLIENT_KIND_WEB', registration: 'CLIENT_REGISTRATION_SYSTEM' };
const global = { global: { provide: { mcpApp: { callServerTool: vi.fn() }, mcpInput: ref({ project_id: 'x6MH9WPWjktMwCBKHy4pF' }) } } };

describe('client views', () => {
  it('lists clients with their kind, the system badge, copyable ids and secret hints', () => {
    const w = mount(ListView, { props: { data: { clients: [web, system] }, descriptor: descriptors.client_list as ListDescriptor }, ...global });
    expect(w.find('h1').text()).toBe('Clients');
    const [first, second] = w.findAll('tbody tr');
    expect(first!.text()).toContain('Web app');
    expect(first!.text()).toContain('…ab12');
    expect(first!.findAll('.badge').map(b => b.text())).toEqual(['Web app']);
    expect(second!.text()).toContain('System');
    expect(second!.text()).toContain('None');
    expect(w.find('button[aria-label="Copy Client ID"]').exists()).toBe(true);
  });

  it('shows one client in full, with each redirect URI on its own line and the console link', () => {
    const w = mount(DetailView, { props: { data: { client: web }, descriptor: descriptors.client_get as DetailDescriptor }, ...global });
    expect(w.findAll('li code').map(c => c.text())).toEqual(['https://store.example/cb', 'https://store.example/cb2', 'openid', 'email']);
    expect(w.text()).toContain('Implicit');
    expect(w.text()).toContain('Manage secrets in the console');
    expect(w.findAll('dt').map(d => d.text())).not.toContain('Sign-out return URIs');
  });

  it('tells the person a new web app needs its secret from the console, and a CLI app none', () => {
    const created = mount(DetailView, { props: { data: { client: web }, descriptor: descriptors.client_create as DetailDescriptor }, ...global });
    expect(created.find('[role="status"]').exists()).toBe(true);
    expect(created.find('[role="note"]').text()).toContain('Generate it in the console');
    expect(created.text()).toContain('Manage secrets in the console');
    const cli = mount(DetailView, { props: { data: { client: { ...web, kind: 'CLIENT_KIND_CLI', consoleUrl: '' } }, descriptor: descriptors.client_create as DetailDescriptor }, ...global });
    expect(cli.find('[role="note"]').text()).toContain('no secret');
    expect(cli.text()).not.toContain('Manage secrets');
  });

  it('sums up secrets by hint only', () => {
    expect(secretHints(undefined)).toBe('None');
    expect(secretHints([{ hint: 'ab12' }, { hint: 'cd34' }])).toBe('…ab12, …cd34');
    expect(secretHints([{}])).toBe('…');
  });
});
