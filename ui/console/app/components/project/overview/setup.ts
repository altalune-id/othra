export interface SetupInput {
  issuerHandle: string;
  signinEmailLink: boolean;
  upstreams: number;
  clients: number;
  logoUrl: string;
  primaryColor: string;
  termsUrl: string;
  privacyUrl: string;
}

export interface SetupItem {
  key: 'handle' | 'method' | 'client' | 'branding' | 'legal';
  done: boolean;
  to: string;
}

// setupItems is the project's setup checklist, each step derived from data the console already reads.
export function setupItems(p: SetupInput, base: string): SetupItem[] {
  return [
    { key: 'handle', done: !!p.issuerHandle, to: `${base}/settings/issuer` },
    { key: 'method', done: p.signinEmailLink || p.upstreams > 0, to: `${base}/settings/sign-in` },
    { key: 'client', done: p.clients > 0, to: `${base}/clients/new` },
    { key: 'branding', done: !!(p.logoUrl || p.primaryColor), to: `${base}/settings/branding` },
    { key: 'legal', done: !!(p.termsUrl && p.privacyUrl), to: `${base}/settings/legal` },
  ];
}
