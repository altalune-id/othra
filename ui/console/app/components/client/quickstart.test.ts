import { describe, expect, it } from 'vitest';
import { authorizeLines, goLines, quickstartEnv, tryAuthorizeUrl } from './quickstart';

const q = { issuer: 'https://id.acme.test/p/apps/', clientId: 'dPm7fmQRJKPVw8PxrgdrN', redirectUri: 'https://store.acme.test/cb' };

describe('quickstartEnv', () => {
  it('has the real values, and the secret only when it is known', () => {
    expect(quickstartEnv(q)).toEqual({ OIDC_ISSUER: q.issuer, OIDC_CLIENT_ID: q.clientId, OIDC_REDIRECT_URI: q.redirectUri });
    expect(quickstartEnv({ ...q, secret: 'othra_cs_x' }).OIDC_CLIENT_SECRET).toBe('othra_cs_x');
    expect(quickstartEnv({ ...q, redirectUri: '' })).not.toHaveProperty('OIDC_REDIRECT_URI');
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

describe('quickstart snippets', () => {
  const q = { issuer: 'https://auth.test/p/acme/', clientId: 'c1', redirectUri: 'https://app.test/cb' };

  it('lists the authorization request with the encoded redirect and PKCE placeholders', () => {
    expect(authorizeLines(q)).toEqual([
      'https://auth.test/p/acme/oauth2/authorize',
      '  ?client_id=c1',
      '  &response_type=code&scope=openid%20profile%20email',
      '  &state=...&nonce=...',
      '  &redirect_uri=https%3A%2F%2Fapp.test%2Fcb',
      '  &code_challenge=...&code_challenge_method=S256',
    ]);
    expect(authorizeLines({ ...q, redirectUri: '' }).some(l => l.includes('redirect_uri'))).toBe(false);
  });

  it('reads the secret from the environment in the Go snippet only for a secret client', () => {
    expect(goLines(q, true)).toContain('\tClientSecret: os.Getenv("OIDC_CLIENT_SECRET"),');
    expect(goLines(q, true).slice(0, 2)).toEqual(['import "os"', 'import "github.com/altalune-id/oidcrp"']);
    expect(goLines(q, false).join('\n')).not.toContain('ClientSecret');
    expect(goLines(q, false).slice(0, 3)).toEqual(['import "github.com/altalune-id/oidcrp"', '', 'rp := oidcrp.New(oidcrp.Config{']);
  });
});
