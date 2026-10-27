import { create } from '@bufbuild/protobuf';
import { timestampFromDate } from '@bufbuild/protobuf/wkt';
import { describe, expect, it } from 'vitest';

import { AppResourceSchema, ConnectedAppSchema, PersonalTokenSchema, PersonalTokenState, SessionSchema, SignInMethodKind, TokenServerSchema } from '#gen/account/v1/account_pb';
import { appView, deviceName, sessionView, signInSees, tokenView, tokenWhen } from './views';

const mac = 'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/141.0.0.0 Safari/537.36';
const other = create(SessionSchema, { id: 'H3nVb8XqT1wLp6RkZ2mJc', method: SignInMethodKind.GOOGLE, userAgent: 'curl/8', authTime: timestampFromDate(new Date('2026-10-20T00:00:00Z')) });
const here = create(SessionSchema, { id: 'Q7mZp2LkV9xRt4NbW8cYd', method: SignInMethodKind.EMAIL_LINK, userAgent: mac, current: true });
const at = new Date('2026-10-01T00:00:00Z');
const claude = create(ConnectedAppSchema, {
  clientId: 'dPm7fmQRJKPVw8PxrgdrN',
  name: 'Claude Code',
  metadataHost: 'claude.ai',
  consented: true,
  connectedAt: timestampFromDate(at),
  resources: [create(AppResourceSchema, { name: '', identifier: 'https://mcp.acme.test/mcp', scopes: ['notes:read', 'notes:write'], scopeDescriptions: { 'notes:read': 'Read your notes' } })],
});
const server = create(TokenServerSchema, { id: 'dPm7fmQRJKPVw8PxrgdrN', name: 'Notes MCP', identifier: 'https://mcp.acme.test/mcp', scopes: ['notes:read'] });
const now = new Date('2026-10-26T00:00:00Z');
const token = create(PersonalTokenSchema, { id: 'K9tWq4ZbN2xLm7VcR5pHs', name: 'cli', hint: 'ab12', server, scopes: ['notes:read'], state: PersonalTokenState.ACTIVE, expiresAt: timestampFromDate(new Date('2026-10-29T00:00:00Z')) });

describe('appView, signing in', () => {
  it('marks the serverless OIDC entry as signing in, apart from the server lines', () => {
    const oidcOnly = appView(create(ConnectedAppSchema, { consented: true, resources: [create(AppResourceSchema, { scopes: ['openid', 'email'] })] }));
    expect(oidcOnly).toMatchObject({ access: [], signsIn: true });
    const both = appView(create(ConnectedAppSchema, {
      resources: [
        create(AppResourceSchema, { scopes: ['openid'] }),
        create(AppResourceSchema, { id: 'B6rTn1YkM8wQz3XpL5vGd', name: '', identifier: 'https://mcp2.test/mcp', scopes: ['sample:read'], scopeDescriptions: { 'sample:read': 'Read samples' } }),
      ],
    }));
    expect(both.signsIn).toBe(true);
    expect(both.access).toEqual([{ server: 'https://mcp2.test/mcp', items: ['Read samples'] }]);
    expect(appView(create(ConnectedAppSchema, {})).signsIn).toBe(true);
    expect(appView(claude).signsIn).toBe(false);
  });
});

describe('deviceName', () => {
  const t = (key: string, params?: Record<string, unknown>) => (params ? `${key}|${JSON.stringify(params)}` : key);
  it('says browser on system, or what it knows', () => {
    expect(deviceName({ browser: 'Chrome', os: 'macOS' }, t)).toBe('sessions.device|{"browser":"Chrome","os":"macOS"}');
    expect(deviceName({ browser: '', os: 'iOS' }, t)).toBe('iOS');
    expect(deviceName({ browser: '', os: '' }, t)).toBe('sessions.unknown_device');
  });
});

describe('sessionView', () => {
  it('names the device and the method', () => {
    expect(sessionView(here)).toMatchObject({ id: 'Q7mZp2LkV9xRt4NbW8cYd', browser: 'Chrome', os: 'macOS', google: false, current: true, signedInAt: undefined });
    expect(sessionView(create(SessionSchema, { lastSeenAt: timestampFromDate(new Date('2026-10-21T00:00:00Z')) })).lastSeenAt).toEqual(new Date('2026-10-21T00:00:00Z'));
    expect(sessionView(other)).toMatchObject({ browser: '', os: '', google: true, signedInAt: new Date('2026-10-20T00:00:00Z') });
  });
});

describe('appView', () => {
  it('keeps the last use and a named server', () => {
    const view = appView(create(ConnectedAppSchema, { lastUsedAt: timestampFromDate(at), resources: [create(AppResourceSchema, { name: 'Notes', identifier: 'https://x.test' })] }));
    expect(view.lastUsedAt).toEqual(at);
    expect(view.access[0]!.server).toBe('Notes');
  });

  it('words the access per server, falling back to the identifier and the raw scope', () => {
    expect(appView(claude)).toEqual({
      clientId: 'dPm7fmQRJKPVw8PxrgdrN',
      name: 'Claude Code',
      host: 'claude.ai',
      signsIn: false,
      sees: 'nothing',
      consented: true,
      connectedAt: at,
      lastUsedAt: undefined,
      access: [{ server: 'https://mcp.acme.test/mcp', items: ['Read your notes', 'notes:write'] }],
    });
  });
});

describe('tokenView', () => {
  it('takes the state from the server and counts the expiry from now', () => {
    expect(tokenView(token, now)).toMatchObject({ id: 'K9tWq4ZbN2xLm7VcR5pHs', server: 'Notes MCP', status: 'active', expiry: { kind: 'soon', days: 3 }, lastUsedAt: undefined });
    expect(tokenView(create(PersonalTokenSchema, { state: PersonalTokenState.REVOKED }), now)).toMatchObject({ status: 'revoked', server: '', expiry: { kind: 'none' } });
    expect(tokenView(create(PersonalTokenSchema, { state: PersonalTokenState.EXPIRED, server: create(TokenServerSchema, { identifier: 'https://x.test' }) }), now).server).toBe('https://x.test');
  });
});

describe('tokenWhen', () => {
  const at = new Date('2026-11-01T00:00:00Z');
  it('reads a revoked token as revoked, never as expiring', () => {
    expect(tokenWhen({ status: 'revoked', expiry: { kind: 'soon', days: 2 }, expiresAt: at, revokedAt: now })).toEqual({ key: 'tokens.revoked_on', date: now });
    expect(tokenWhen({ status: 'expired', expiry: { kind: 'expired' }, expiresAt: at })).toEqual({ key: 'tokens.expired_on', date: at });
    expect(tokenWhen({ status: 'active', expiry: { kind: 'soon', days: 2 }, expiresAt: at })).toEqual({ key: 'tokens.expires_soon', days: 2 });
    expect(tokenWhen({ status: 'active', expiry: { kind: 'ok' }, expiresAt: at })).toEqual({ key: 'tokens.expires_on', date: at });
  });

  it('keeps the revoke time', () => {
    expect(tokenView(create(PersonalTokenSchema, { state: PersonalTokenState.REVOKED, revokedAt: timestampFromDate(now) }), now).revokedAt).toEqual(now);
  });
});

describe('signInSees', () => {
  it('words the sign-in line from the granted scopes only', () => {
    expect(signInSees(['openid'])).toBe('nothing');
    expect(signInSees(['openid', 'email'])).toBe('email');
    expect(signInSees(['openid', 'profile'])).toBe('name');
    expect(signInSees(['openid', 'email', 'profile'])).toBe('email_name');
    const app = appView(create(ConnectedAppSchema, { consented: true, resources: [create(AppResourceSchema, { scopes: ['openid'] })] }));
    expect(app.sees).toBe('nothing');
  });
});
