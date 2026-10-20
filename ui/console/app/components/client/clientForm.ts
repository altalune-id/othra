import type { Client } from '~~/gen/client/v1/client_pb';
import type { ClientKindName } from './kinds';
import { ConsentPolicy } from '~~/gen/client/v1/client_pb';
import { kindName } from './kinds';

export type ConsentName = 'require' | 'implicit';

export interface ClientFields {
  kind: ClientKindName;
  name: string;
  redirectUris: string[];
  postLogoutUris: string[];
  refreshTokens: boolean;
  consent: ConsentName;
}

export function consentName(policy: ConsentPolicy): ConsentName {
  return policy === ConsentPolicy.IMPLICIT ? 'implicit' : 'require';
}

export function consentValue(consent: ConsentName): ConsentPolicy {
  return consent === 'implicit' ? ConsentPolicy.IMPLICIT : ConsentPolicy.REQUIRE;
}

// emptyFields is the create form's start, matching the API defaults: a web app with one empty redirect row and refresh tokens off.
export function emptyFields(kind: ClientKindName = 'web'): ClientFields {
  return { kind, name: '', redirectUris: [''], postLogoutUris: [], refreshTokens: false, consent: 'require' };
}

// fieldsOf fills the edit form from a saved client.
export function fieldsOf(client: Client): ClientFields {
  return {
    kind: kindName(client.kind),
    name: client.name,
    redirectUris: client.redirectUris.length > 0 ? [...client.redirectUris] : [''],
    postLogoutUris: [...client.postLogoutRedirectUris],
    refreshTokens: client.refreshTokens,
    consent: consentName(client.consentPolicy),
  };
}
