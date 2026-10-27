import type { Client } from '#gen/client/v1/client_pb';
import type { ClientKindName } from './kinds';
import { AuthMethod, ConsentPolicy } from '#gen/client/v1/client_pb';
import { kindName } from './kinds';

export type ConsentName = 'require' | 'implicit';

export type AuthMethodName = 'basic' | 'post';

export interface ClientFields {
  kind: ClientKindName;
  name: string;
  redirectUris: string[];
  postLogoutUris: string[];
  refreshTokens: boolean;
  consent: ConsentName;
  authMethod: AuthMethodName;
  pkceRequired: boolean;
}

export function consentName(policy: ConsentPolicy): ConsentName {
  return policy === ConsentPolicy.IMPLICIT ? 'implicit' : 'require';
}

export function consentValue(consent: ConsentName): ConsentPolicy {
  return consent === 'implicit' ? ConsentPolicy.IMPLICIT : ConsentPolicy.REQUIRE;
}

// authMethodName reads how a saved web client sends its secret; anything but client_secret_post is the header.
export function authMethodName(tokenEndpointAuthMethod: string): AuthMethodName {
  return tokenEndpointAuthMethod === 'client_secret_post' ? 'post' : 'basic';
}

export function authMethodValue(name: AuthMethodName): AuthMethod {
  return name === 'post' ? AuthMethod.CLIENT_SECRET_POST : AuthMethod.CLIENT_SECRET_BASIC;
}

// emptyFields is the create form's start, matching the API defaults: a web app with one empty redirect row and refresh tokens off.
export function emptyFields(kind: ClientKindName = 'web'): ClientFields {
  return { kind, name: '', redirectUris: [''], postLogoutUris: [], refreshTokens: false, consent: 'require', authMethod: 'basic', pkceRequired: true };
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
    authMethod: authMethodName(client.tokenEndpointAuthMethod),
    pkceRequired: client.pkceRequired,
  };
}
