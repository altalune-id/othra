import { describe, expect, it } from 'vitest';
import { quickstartEnv, tryAuthorizeUrl } from './quickstart';

const q = { issuer: 'https://id.acme.test/p/apps/', clientId: 'dPm7fmQRJKPVw8PxrgdrN', redirectUri: 'https://store.acme.test/cb' };

describe('quickstartEnv', () => {
  it('has the real values, and the secret only when it is known', () => {
    expect(quickstartEnv(q)).toEqual({ OTHRA_ISSUER: q.issuer, OTHRA_CLIENT_ID: q.clientId, OTHRA_REDIRECT_URI: q.redirectUri });
    expect(quickstartEnv({ ...q, secret: 'othra_cs_x' }).OTHRA_CLIENT_SECRET).toBe('othra_cs_x');
    expect(quickstartEnv({ ...q, redirectUri: '' })).not.toHaveProperty('OTHRA_REDIRECT_URI');
  });
});

describe('tryAuthorizeUrl', () => {
  it('builds a PKCE S256 authorize request on the issuer, fresh each time', async () => {
    const a = new URL(await tryAuthorizeUrl(q));
    const b = new URL(await tryAuthorizeUrl(q));
    expect(a.origin + a.pathname).toBe('https://id.acme.test/p/apps/oauth2/authorize');
    expect(Object.fromEntries(a.searchParams)).toMatchObject({ response_type: 'code', client_id: q.clientId, redirect_uri: q.redirectUri, scope: 'openid profile email', code_challenge_method: 'S256' });
    expect(a.searchParams.get('code_challenge')).toMatch(/^[\w-]{43}$/);
    expect(a.searchParams.get('state')).not.toBe(b.searchParams.get('state'));
    expect(a.searchParams.get('nonce')).not.toBe(b.searchParams.get('nonce'));
  });
});
