import { describe, expect, it } from 'vitest';

import { reauthURL, signInURL, switchAccountURL, termsURL } from './hosted';

describe('hosted links', () => {
  it('signs in and comes back to the same portal page', () => {
    expect(signInURL('/p/acme', { pathname: '/p/acme/account/sessions', search: '?x=1' })).toBe('/p/acme/login?return_to=%2Fp%2Facme%2Faccount%2Fsessions%3Fx%3D1');
  });

  it('adds another account and returns to wherever the portal is mounted', () => {
    expect(switchAccountURL('/p/acme', '/p/acme/account/')).toBe('/p/acme/login?add=1&return_to=%2Fp%2Facme%2Faccount%2F');
    expect(switchAccountURL('/p/acme', '/p/acme/me/')).toBe('/p/acme/login?add=1&return_to=%2Fp%2Facme%2Fme%2F');
  });

  it('asks for a fresh sign-in before coming back', () => {
    expect(reauthURL('/p/acme', '/p/acme/account/tokens/new')).toBe('/p/acme/login?prompt=login&return_to=%2Fp%2Facme%2Faccount%2Ftokens%2Fnew');
  });

  it('accepts the updated terms, then comes back', () => {
    expect(termsURL('/p/acme', { pathname: '/p/acme/account/apps', search: '' })).toBe('/p/acme/terms?return_to=%2Fp%2Facme%2Faccount%2Fapps');
  });
});
