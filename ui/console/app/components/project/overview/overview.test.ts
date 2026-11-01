import { describe, expect, it, vi } from 'vitest';
import { setupItems } from './setup';
import { useDiscoveryCheck } from './useDiscoveryCheck';

const empty = { issuerHandle: '', signinEmailLink: false, upstreams: 0, clients: 0, logoUrl: '', primaryColor: '', termsUrl: '', privacyUrl: '' };

describe('setupItems', () => {
  it('has five steps, each linked to the page that completes it', () => {
    const items = setupItems(empty, '/o/a/projects/p');
    expect(items.map(i => [i.key, i.done, i.to])).toEqual([
      ['handle', false, '/o/a/projects/p/settings/issuer'],
      ['method', false, '/o/a/projects/p/settings/sign-in'],
      ['client', false, '/o/a/projects/p/clients/new'],
      ['branding', false, '/o/a/projects/p/settings/branding'],
      ['legal', false, '/o/a/projects/p/settings/legal'],
    ]);
  });

  it('counts a step done from the data that shows it', () => {
    const done = (p: Partial<typeof empty>) => setupItems({ ...empty, ...p }, '').filter(i => i.done).map(i => i.key);
    expect(done({ issuerHandle: 'acme' })).toEqual(['handle']);
    expect(done({ upstreams: 1 })).toEqual(['method']);
    expect(done({ signinEmailLink: true })).toEqual(['method']);
    expect(done({ clients: 2 })).toEqual(['client']);
    expect(done({ primaryColor: '#0a7' })).toEqual(['branding']);
    expect(done({ termsUrl: 'https://t' })).toEqual([]);
    expect(done({ termsUrl: 'https://t', privacyUrl: 'https://p' })).toEqual(['legal']);
  });
});

describe('useDiscoveryCheck', () => {
  const issuer = 'https://id.acme.test/p/apps';
  const reply = (body: unknown, ok = true) => ({ ok, json: async () => body });

  it('reads discovery, then counts the keys at its jwks_uri', async () => {
    const fetcher = vi.fn()
      .mockResolvedValueOnce(reply({ issuer, jwks_uri: `${issuer}/oauth2/jwks.json` }))
      .mockResolvedValueOnce(reply({ keys: [{ kid: 'a' }, { kid: 'b' }] }));
    const c = useDiscoveryCheck(issuer, fetcher);
    expect(c.state.value).toBe('checking');
    await c.check();
    expect(fetcher.mock.calls.map(([u]) => u)).toEqual([`${issuer}/.well-known/openid-configuration`, `${issuer}/oauth2/jwks.json`]);
    expect([c.state.value, c.keys.value, c.doc.value?.issuer]).toEqual(['serving', 2, issuer]);
  });

  it('reports unreachable on a failed fetch, a bad status or no issuer', async () => {
    const down = useDiscoveryCheck(issuer, vi.fn().mockRejectedValue(new Error('offline')));
    await down.check();
    expect(down.state.value).toBe('unreachable');
    const bad = useDiscoveryCheck(issuer, vi.fn().mockResolvedValue(reply({}, false)));
    await bad.check();
    expect(bad.state.value).toBe('unreachable');
    const none = useDiscoveryCheck('', vi.fn());
    await none.check();
    expect(none.state.value).toBe('unreachable');
  });

  it('counts no keys when discovery names no JWKS', async () => {
    const c = useDiscoveryCheck(issuer, vi.fn().mockResolvedValue(reply({ issuer })));
    await c.check();
    expect([c.state.value, c.keys.value]).toEqual(['serving', 0]);
  });
});
