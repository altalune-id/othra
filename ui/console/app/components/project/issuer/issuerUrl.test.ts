import { describe, expect, it } from 'vitest';
import { splitIssuerUrl } from './issuerUrl';

describe('splitIssuerUrl', () => {
  it('splits an issuer URL after its /p/ so it can wrap there', () => {
    expect(splitIssuerUrl('https://auth.example.com/p/acme-web')).toEqual({ base: 'https://auth.example.com/p/', handle: 'acme-web' });
    expect(splitIssuerUrl('http://localhost:5150/p/default-storefront')).toEqual({ base: 'http://localhost:5150/p/', handle: 'default-storefront' });
  });

  it('keeps a URL without /p/ whole', () => {
    expect(splitIssuerUrl('https://auth.example.com')).toEqual({ base: 'https://auth.example.com', handle: '' });
    expect(splitIssuerUrl('')).toEqual({ base: '', handle: '' });
  });
});
