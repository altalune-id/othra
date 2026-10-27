// @vitest-environment happy-dom
import type { DetailDescriptor, ListDescriptor } from '../generic/descriptor';
import { flushPromises, mount } from '@vue/test-utils';
import { describe, expect, it, vi } from 'vitest';
import { ref } from 'vue';
import DetailView from '../generic/DetailView.vue';
import ListView from '../generic/ListView.vue';
import { descriptors, failureText } from './descriptors';

const verified = {
  id: 'R7mPq2XwL9kZt4VnB8cYd',
  name: 'Notes MCP',
  identifier: 'https://notes.example/mcp',
  scopes: ['notes:read', 'notes:write'],
  state: 'RESOURCE_SERVER_STATE_VERIFIED',
  metadataUrl: 'https://notes.example/.well-known/oauth-protected-resource/mcp',
  verifiedAt: '2026-10-27T03:04:05Z',
};
const failed = { ...verified, id: 'F2nQw8XpL5kZr3VbT7cYd', state: 'RESOURCE_SERVER_STATE_FAILED', lastError: { reason: 'status', status: 503 } };

function provide(callServerTool = vi.fn(), input: Record<string, unknown> = { projectId: 'x6MH9WPWjktMwCBKHy4pF' }) {
  return { global: { provide: { mcpApp: { callServerTool }, mcpInput: ref(input) } } };
}

describe('resource server views', () => {
  it('lists servers with their state badge and refreshes for the same project', async () => {
    const callServerTool = vi.fn().mockResolvedValue({ structuredContent: { resourceServers: [verified] } });
    const w = mount(ListView, { props: { data: { resourceServers: [verified, { ...failed, trusted: true }] }, descriptor: descriptors.resource_server_list as ListDescriptor }, ...provide(callServerTool) });
    const rows = w.findAll('tbody tr');
    expect(rows[0]!.text()).toContain('Verified');
    expect(rows[1]!.text()).toContain('Failed');
    expect(rows[1]!.text()).toContain('Trusted');
    await w.findAll('button').find(b => b.text() === 'Refresh')!.trigger('click');
    await flushPromises();
    expect(callServerTool).toHaveBeenCalledWith({ name: 'resource_server_list', arguments: { projectId: 'x6MH9WPWjktMwCBKHy4pF' } });
  });

  it('explains a failed server in the console\'s words', () => {
    const w = mount(DetailView, { props: { data: { resourceServer: failed }, descriptor: descriptors.resource_server_get as DetailDescriptor }, ...provide() });
    expect(w.find('[role="note"]').text()).toBe('The metadata URL returned HTTP 503. Apps can\'t get tokens for it. Fix the metadata, then verify again.');
    const ok = mount(DetailView, { props: { data: { resourceServer: verified }, descriptor: descriptors.resource_server_get as DetailDescriptor }, ...provide() });
    expect(ok.find('[role="note"]').exists()).toBe(false);
    expect(ok.text()).toContain('2026-10-27 03:04 UTC');
  });

  it('shows a verify result as passed, or failed with its reason', () => {
    const passed = mount(DetailView, { props: { data: { passed: true, resourceServer: verified }, descriptor: descriptors.resource_server_verify as DetailDescriptor }, ...provide() });
    expect(passed.text()).toContain('Passed');
    expect(passed.text()).toContain('https://notes.example/mcp');
    const no = mount(DetailView, { props: { data: { resourceServer: failed, error: { reason: 'resource_mismatch', resource: 'https://other.example/mcp' } }, descriptor: descriptors.resource_server_verify as DetailDescriptor }, ...provide() });
    expect(no.text()).toContain('Failed');
    expect(no.find('[role="note"]').text()).toContain('resource is https://other.example/mcp, not https://notes.example/mcp.');
  });

  it('asks for verification after create unless already verified', () => {
    const pending = mount(DetailView, { props: { data: { resourceServer: { ...verified, state: 'RESOURCE_SERVER_STATE_PENDING' } }, descriptor: descriptors.resource_server_create as DetailDescriptor }, ...provide() });
    expect(pending.find('[role="note"]').text()).toContain('resource_server_verify');
    const done = mount(DetailView, { props: { data: { resourceServer: verified }, descriptor: descriptors.resource_server_create as DetailDescriptor }, ...provide() });
    expect(done.find('[role="note"]').exists()).toBe(false);
  });

  it('lists links keyed by client and server, refreshing with whichever target the call used', async () => {
    const callServerTool = vi.fn().mockResolvedValue({ structuredContent: { links: [] } });
    const links = [
      { clientId: 'c1', clientName: 'Store', resourceServerId: 'r1', resourceServerName: 'Notes', resourceServerIdentifier: 'https://notes.example/mcp', resourceServerState: 'RESOURCE_SERVER_STATE_VERIFIED', scopes: ['notes:read'] },
      { clientId: 'c2', clientName: 'CLI', resourceServerId: 'r1', resourceServerName: 'Notes', resourceServerIdentifier: 'https://notes.example/mcp', resourceServerState: 'RESOURCE_SERVER_STATE_VERIFIED', scopes: ['notes:read'] },
    ];
    const w = mount(ListView, { props: { data: { links }, descriptor: descriptors.resource_server_link_list as ListDescriptor }, ...provide(callServerTool, { project_id: 'p1', resource_server_id: 'r1' }) });
    expect(w.findAll('tbody tr')).toHaveLength(2);
    await w.findAll('button').find(b => b.text() === 'Refresh')!.trigger('click');
    await flushPromises();
    expect(callServerTool).toHaveBeenCalledWith({ name: 'resource_server_link_list', arguments: { projectId: 'p1', resourceServerId: 'r1' } });
    expect(w.text()).toContain('No links.');
  });

  it('falls back to a plain reason for one it does not know', () => {
    expect(failureText(undefined, '')).toBe('The check failed.');
    expect(failureText({ reason: 'issuer_missing' }, '')).toBe('authorization_servers doesn\'t list this project\'s issuer.');
  });
});
