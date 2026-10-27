import type { ConnectedApp, PersonalToken, Session } from '#gen/account/v1/account_pb';
import type { ExpiryState, KeyStatus } from '../tokens/keyStatus';
import { timestampDate } from '@bufbuild/protobuf/wkt';
import { PersonalTokenState, SignInMethodKind } from '#gen/account/v1/account_pb';
import { device } from '../device';
import { expiryState } from '../tokens/keyStatus';

export interface SessionView {
  id: string;
  browser: string;
  os: string;
  userAgent: string;
  google: boolean;
  signedInAt?: Date;
  lastSeenAt?: Date;
  current: boolean;
}

// sessionView is what a session row shows: the device it claims, how it signed in and when.
export function sessionView(s: Session): SessionView {
  const d = device(s.userAgent);
  return {
    id: s.id,
    browser: d.browser,
    os: d.os,
    userAgent: s.userAgent,
    google: s.method === SignInMethodKind.GOOGLE,
    signedInAt: s.authTime ? timestampDate(s.authTime) : undefined,
    lastSeenAt: s.lastSeenAt ? timestampDate(s.lastSeenAt) : undefined,
    current: s.current,
  };
}

export interface AccessLine {
  server: string;
  items: string[];
}

export type SignInSees = 'nothing' | 'email' | 'name' | 'email_name';

// signInSees reads what signing in lets the app see from the granted OIDC scopes on the serverless entry.
export function signInSees(scopes: string[]): SignInSees {
  const email = scopes.includes('email');
  const name = scopes.includes('profile');
  if (email && name) {
    return 'email_name';
  }
  if (email) {
    return 'email';
  }
  return name ? 'name' : 'nothing';
}

export interface AppView {
  clientId: string;
  name: string;
  host: string;
  access: AccessLine[];
  signsIn: boolean;
  sees: SignInSees;
  consented: boolean;
  connectedAt?: Date;
  lastUsedAt?: Date;
}

// appView says what an app can reach: signing the person in (the entry with no server, or no entry at all) and, per server, the server's own scope words.
export function appView(app: ConnectedApp): AppView {
  return {
    clientId: app.clientId,
    name: app.name,
    host: app.metadataHost,
    access: app.resources.filter(r => r.identifier).map(r => ({ server: r.name || r.identifier, items: r.scopes.map(s => r.scopeDescriptions[s] || s) })),
    signsIn: app.resources.length === 0 || app.resources.some(r => !r.identifier),
    sees: signInSees(app.resources.find(r => !r.identifier)?.scopes ?? []),
    consented: app.consented,
    connectedAt: app.connectedAt ? timestampDate(app.connectedAt) : undefined,
    lastUsedAt: app.lastUsedAt ? timestampDate(app.lastUsedAt) : undefined,
  };
}

export interface TokenView {
  id: string;
  name: string;
  hint: string;
  server: string;
  scopes: string[];
  status: KeyStatus;
  expiry: ExpiryState;
  expiresAt?: Date;
  lastUsedAt?: Date;
  revokedAt?: Date;
}

const statusOf: Record<PersonalTokenState, KeyStatus> = {
  [PersonalTokenState.UNSPECIFIED]: 'active',
  [PersonalTokenState.ACTIVE]: 'active',
  [PersonalTokenState.EXPIRED]: 'expired',
  [PersonalTokenState.REVOKED]: 'revoked',
};

// tokenView is what a token row shows; the server decides the state, the expiry badge counts from now.
export function tokenView(t: PersonalToken, now: Date): TokenView {
  const expiresAt = t.expiresAt ? timestampDate(t.expiresAt) : undefined;
  return {
    id: t.id,
    name: t.name,
    hint: t.hint,
    server: t.server?.name || t.server?.identifier || '',
    scopes: [...t.scopes],
    status: statusOf[t.state],
    expiry: expiryState(expiresAt, now),
    expiresAt,
    lastUsedAt: t.lastUsedAt ? timestampDate(t.lastUsedAt) : undefined,
    revokedAt: t.revokedAt ? timestampDate(t.revokedAt) : undefined,
  };
}

// tokenWhen picks a token row's date line by its status first: a revoked token never reads as expiring.
export function tokenWhen(t: Pick<TokenView, 'status' | 'expiry' | 'expiresAt' | 'revokedAt'>): { key: string, date?: Date, days?: number } {
  switch (t.status) {
    case 'revoked':
      return { key: 'tokens.revoked_on', date: t.revokedAt };
    case 'expired':
      return { key: 'tokens.expired_on', date: t.expiresAt };
    default:
      return t.expiry.kind === 'soon' ? { key: 'tokens.expires_soon', days: t.expiry.days } : { key: 'tokens.expires_on', date: t.expiresAt };
  }
}

// deviceName names a session's device in words, or the unknown-device text.
export function deviceName(s: Pick<SessionView, 'browser' | 'os'>, t: (key: string, params?: Record<string, unknown>) => string): string {
  if (s.browser && s.os) {
    return t('sessions.device', { browser: s.browser, os: s.os });
  }
  return s.browser || s.os || t('sessions.unknown_device');
}
