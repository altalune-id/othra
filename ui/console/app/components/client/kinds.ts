import type { Client } from '#gen/client/v1/client_pb';
import { ClientKind, ClientRegistration } from '#gen/client/v1/client_pb';

export type ClientKindName = 'web' | 'cli';

export interface RedirectUriRule {
  https: boolean;
  loopbackPort: 'required' | 'forbidden';
  refusal: 'https_required' | 'loopback_only';
}

export interface ClientKindSpec {
  value: ClientKind;
  icon: string;
  labelKey: string;
  helpKey: string;
  redirectHelpKey: string;
  placeholder: string;
  uris: RedirectUriRule;
  secret: boolean;
  postLogout: boolean;
}

// clientKinds is every kind the console offers, with its copy, its redirect URI rule and what it carries.
export const clientKinds: Record<ClientKindName, ClientKindSpec> = {
  web: {
    value: ClientKind.WEB,
    icon: 'lucide:globe',
    labelKey: 'clients.kind_web',
    helpKey: 'clients.kind_web_help',
    redirectHelpKey: 'clients.redirect_help_web',
    placeholder: 'https://app.example.com/oauth/callback',
    uris: { https: true, loopbackPort: 'required', refusal: 'https_required' },
    secret: true,
    postLogout: true,
  },
  cli: {
    value: ClientKind.CLI,
    icon: 'lucide:terminal',
    labelKey: 'clients.kind_native',
    helpKey: 'clients.kind_native_help',
    redirectHelpKey: 'clients.redirect_help_native',
    placeholder: 'http://127.0.0.1/callback',
    uris: { https: false, loopbackPort: 'forbidden', refusal: 'loopback_only' },
    secret: false,
    postLogout: false,
  },
};

export const clientKindNames = Object.keys(clientKinds) as ClientKindName[];

// kindName maps the proto kind onto a console kind; anything unknown reads as the first one.
export function kindName(kind: ClientKind): ClientKindName {
  return clientKindNames.find(name => clientKinds[name].value === kind) ?? clientKindNames[0]!;
}

// isCimdClient reports a client that signed in with a client metadata document.
export function isCimdClient(c: Pick<Client, 'registration'>): boolean {
  return c.registration === ClientRegistration.CIMD;
}

// isSystemClient reports the console client Othra seeds at install.
export function isSystemClient(c: Pick<Client, 'registration'>): boolean {
  return c.registration === ClientRegistration.SYSTEM;
}
