import { describe, expect, it } from 'vitest';
import { discoveryUrls } from './discovery';

describe('discoveryUrls', () => {
  it('derives the OIDC, RFC 8414 and JWKS URLs from the issuer', () => {
    expect(discoveryUrls('https://auth.example.com/p/acme')).toEqual({
      oidc: 'https://auth.example.com/p/acme/.well-known/openid-configuration',
      oauth: 'https://auth.example.com/.well-known/oauth-authorization-server/p/acme',
      jwks: 'https://auth.example.com/p/acme/oauth2/jwks.json',
    });
  });

  it('inserts the RFC 8414 well-known path at the host root, before a base path', () => {
    expect(discoveryUrls('https://h/othra/p/x')).toEqual({
      oidc: 'https://h/othra/p/x/.well-known/openid-configuration',
      oauth: 'https://h/.well-known/oauth-authorization-server/othra/p/x',
      jwks: 'https://h/othra/p/x/oauth2/jwks.json',
    });
  });

  it('keeps a port and drops a trailing slash', () => {
    expect(discoveryUrls('http://localhost:5150/p/default-storefront/')).toEqual({
      oidc: 'http://localhost:5150/p/default-storefront/.well-known/openid-configuration',
      oauth: 'http://localhost:5150/.well-known/oauth-authorization-server/p/default-storefront',
      jwks: 'http://localhost:5150/p/default-storefront/oauth2/jwks.json',
    });
  });

  it('returns nothing for an empty or unparsable issuer', () => {
    expect(discoveryUrls('')).toBeNull();
    expect(discoveryUrls('not a url')).toBeNull();
  });
});
