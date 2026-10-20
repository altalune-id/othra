import { create } from '@bufbuild/protobuf';
import { describe, expect, it } from 'vitest';

import { ClientKind, ClientSchema, ConsentPolicy } from '../../../gen/client/v1/client_pb';
import { consentName, consentValue, emptyFields, fieldsOf } from './clientForm';

describe('clientForm', () => {
  it('maps consent both ways, defaulting to asking the end user', () => {
    expect(consentName(ConsentPolicy.IMPLICIT)).toBe('implicit');
    expect(consentName(ConsentPolicy.UNSPECIFIED)).toBe('require');
    expect(consentValue('implicit')).toBe(ConsentPolicy.IMPLICIT);
    expect(consentValue('require')).toBe(ConsentPolicy.REQUIRE);
  });

  it('starts a web app with one empty redirect row and refresh tokens off, as the API does', () => {
    expect(emptyFields()).toEqual({ kind: 'web', name: '', redirectUris: [''], postLogoutUris: [], refreshTokens: false, consent: 'require' });
    expect(emptyFields('cli').kind).toBe('cli');
  });

  it('fills the edit form from a client, keeping one row when it has no URI', () => {
    const client = create(ClientSchema, {
      id: 'dPm7fmQRJKPVw8PxrgdrN',
      name: 'Store web',
      kind: ClientKind.WEB,
      redirectUris: ['https://store.example/cb'],
      postLogoutRedirectUris: ['https://store.example/'],
      refreshTokens: false,
      consentPolicy: ConsentPolicy.IMPLICIT,
    });
    expect(fieldsOf(client)).toEqual({ kind: 'web', name: 'Store web', redirectUris: ['https://store.example/cb'], postLogoutUris: ['https://store.example/'], refreshTokens: false, consent: 'implicit' });
    expect(fieldsOf(create(ClientSchema, { kind: ClientKind.CLI })).redirectUris).toEqual(['']);
  });
});
