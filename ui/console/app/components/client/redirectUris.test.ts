import { describe, expect, it } from 'vitest';

import { checkUriList, redirectUriProblem, serverRow, usesLocalhost } from './redirectUris';

describe('redirectUriProblem', () => {
  it.each([
    ['web', 'https://store.example/oauth/callback'],
    ['web', 'http://127.0.0.1:5100/cb'],
    ['web', 'http://[::1]:8080/cb'],
    ['web', 'http://localhost:5173/oauth/callback'],
    ['cli', 'http://127.0.0.1/callback'],
    ['cli', 'http://[::1]/callback'],
    ['cli', 'http://localhost/callback'],
  ] as const)('accepts %s %s', (kind, uri) => {
    expect(redirectUriProblem(kind, uri)).toBeNull();
  });

  it.each([
    ['web', '', 'required'],
    ['web', `https://a.example/${'x'.repeat(2000)}`, 'too_long'],
    ['web', 'https://app.example/cb#frag', 'fragment'],
    ['web', 'https://*.example/cb', 'wildcard'],
    ['web', 'https://u:p@app.example/cb', 'userinfo'],
    ['web', 'https://app.example/a/../cb', 'dot_segments'],
    ['web', 'https://app.example/a/.', 'dot_segments'],
    ['web', 'cb', 'unparsable'],
    ['web', 'com.example.app:/cb', 'https_required'],
    ['web', 'javascript:alert(1)', 'https_required'],
    ['web', 'http://app.example/cb', 'https_required'],
    ['web', 'http://127.0.0.1/cb', 'https_required'],
    ['cli', 'https://app.example/cb', 'loopback_only'],
    ['cli', 'http://0.0.0.0/cb', 'loopback_only'],
    ['cli', 'http://127.0.0.2/cb', 'loopback_only'],
    ['cli', 'http://foo.localhost/cb', 'loopback_only'],
    ['cli', 'https://127.0.0.1/cb', 'loopback_only'],
    ['cli', 'http://127.0.0.1:8080/cb', 'port_ignored'],
    ['cli', 'com.example.app:/cb', 'loopback_only'],
  ] as const)('refuses %s %s: %s', (kind, uri, reason) => {
    expect(redirectUriProblem(kind, uri)?.reason).toBe(reason);
  });

  it('says how to write a URI in canonical form', () => {
    expect(redirectUriProblem('web', 'https://Store.Example/cb')).toEqual({ reason: 'not_canonical', canonical: 'https://store.example/cb' });
    expect(redirectUriProblem('web', 'https://store.example:443/cb')).toEqual({ reason: 'not_canonical', canonical: 'https://store.example/cb' });
    expect(redirectUriProblem('web', 'HTTPS://store.example/cb')).toEqual({ reason: 'not_canonical', canonical: 'https://store.example/cb' });
  });

  it('refuses a URI with no host', () => {
    expect(redirectUriProblem('web', 'https:/cb')?.reason).toBe('not_canonical');
    expect(redirectUriProblem('web', 'https:')?.reason).toBe('unparsable');
  });
});

describe('usesLocalhost', () => {
  it('flags localhost only', () => {
    expect(usesLocalhost('http://localhost/callback')).toBe(true);
    expect(usesLocalhost('http://[::1]/callback')).toBe(false);
    expect(usesLocalhost('not a url')).toBe(false);
  });
});

describe('checkUriList', () => {
  it('drops empty rows, keeps the row of each URI, and marks a repeat', () => {
    const list = checkUriList('web', ['https://a.example/cb', ' ', ' https://a.example/cb ', 'https://b.example/cb'], true);
    expect(list.uris).toEqual(['https://a.example/cb', 'https://a.example/cb', 'https://b.example/cb']);
    expect(list.rows).toEqual([0, 2, 3]);
    expect(list.problems.map(p => p?.reason ?? null)).toEqual([null, null, 'duplicate', null]);
  });

  it('flags the first row of a required list with no URI, and not an optional one', () => {
    expect(checkUriList('web', [''], true).problems[0]?.reason).toBe('required');
    expect(checkUriList('web', [], true).problems[0]?.reason).toBe('required');
    expect(checkUriList('web', [''], false).problems).toEqual([null]);
  });
});

describe('serverRow', () => {
  const list = checkUriList('web', ['https://a.example/cb', '', 'https://b.example/cb'], true);

  it('maps an indexed field onto the row it came from', () => {
    expect(serverRow('redirect_uris[1]', '', list)).toBe(2);
    expect(serverRow('redirect_uris[9]', '', list)).toBe(-1);
  });

  it('falls back to the echoed URI, and gives up without either', () => {
    expect(serverRow('redirect_uris', 'https://b.example/cb', list)).toBe(2);
    expect(serverRow('redirect_uris', 'https://c.example/cb', list)).toBe(-1);
    expect(serverRow('', '', list)).toBe(-1);
  });
});
