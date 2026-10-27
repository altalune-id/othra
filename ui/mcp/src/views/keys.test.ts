// @vitest-environment happy-dom
import type { DetailDescriptor, ListDescriptor } from '../generic/descriptor';
import { mount } from '@vue/test-utils';
import { describe, expect, it, vi } from 'vitest';
import { ref } from 'vue';
import DetailView from '../generic/DetailView.vue';
import ListView from '../generic/ListView.vue';
import { descriptors, rotatedNote } from './descriptors';

const provide = { global: { provide: { mcpApp: { callServerTool: vi.fn() }, mcpInput: ref({ projectId: 'p1' }) } } };
const active = { publicId: 'k1', kid: 'kid-1', alg: 'RS256', status: 'SIGNING_KEY_STATUS_ACTIVE', createdAt: '2026-10-01T00:00:00Z' };
const next = { publicId: 'k2', kid: 'kid-2', alg: 'RS256', status: 'SIGNING_KEY_STATUS_NEXT', createdAt: '2026-10-27T03:00:00Z', activatesAt: '2026-10-27T03:10:00Z' };

describe('signing key views', () => {
  it('lists keys with their status in the console\'s words', () => {
    const w = mount(ListView, { props: { data: { keys: [next, active] }, descriptor: descriptors.signing_key_list as ListDescriptor }, ...provide });
    expect(w.findAll('tbody tr').map(r => r.find('.badge').text())).toEqual(['Next', 'Active']);
    expect(w.text()).toContain('2026-10-27 03:10 UTC');
  });

  it('says when a rotated key starts signing, or that it signs now', () => {
    const w = mount(DetailView, { props: { data: { key: next }, descriptor: descriptors.signing_key_rotate as DetailDescriptor }, ...provide });
    expect(w.find('[role="note"]').text()).toBe('New key kid-2 is published. It starts signing at about 2026-10-27 03:10 UTC.');
    expect(rotatedNote(active)).toBe('New key kid-1 is active and signs tokens now.');
  });
});

describe('upstream IdP views', () => {
  const google = { id: 'u1', kind: 'UPSTREAM_KIND_GOOGLE', clientId: 'abc.apps.googleusercontent.com', enabled: true, secretSet: true, redirectUri: 'http://localhost:5150/p/web/federation/google/callback' };

  it('lists upstreams with the redirect URI to copy', () => {
    const w = mount(ListView, { props: { data: { upstreams: [google] }, descriptor: descriptors.upstream_list as ListDescriptor }, ...provide });
    expect(w.text()).toContain('Google');
    expect(w.find('button[aria-label="Copy Redirect URI"]').exists()).toBe(true);
  });

  it('asks for the secret before an upstream can be turned on', () => {
    const w = mount(DetailView, { props: { data: { upstream: { ...google, secretSet: false, enabled: false } }, descriptor: descriptors.upstream_get as DetailDescriptor }, ...provide });
    expect(w.find('[role="note"]').text()).toContain('client secret isn\'t set');
    const ok = mount(DetailView, { props: { data: { upstream: google }, descriptor: descriptors.upstream_get as DetailDescriptor }, ...provide });
    expect(ok.find('[role="note"]').exists()).toBe(false);
  });
});
