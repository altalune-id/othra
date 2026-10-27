import { create } from '@bufbuild/protobuf';
import { describe, expect, it } from 'vitest';

import { ResourceServerState, VerificationFailureSchema } from '#gen/resourceserver/v1/resourceserver_pb';
import { t } from '#test/nuxt-globals';
import baseEnUS from '../../../../base/i18n/locales/en-US.json';
import consoleEnUS from '../../../i18n/locales/en-US.json';
import { identifierProblem, isHttpIdentifier } from './resourceUri';
import { checkScopes, scopeMessageKey, scopeProblem } from './scopes';
import { failureReasons, failureText, metadataSnippet, relativeTime, verificationDot, verificationName, verificationStates } from './verification';

describe('identifierProblem', () => {
  it.each(['https://store.example/mcp', 'https://store.example', 'https://api.store.example/v1/', 'http://127.0.0.1:8080/mcp', 'http://localhost:8080/api'])('accepts %s', (uri) => {
    expect(identifierProblem(uri)).toBeNull();
  });

  it.each([
    ['urn:store:api', 'unparsable'],
    ['https://store.example/mcp#x', 'fragment'],
    ['https://store.example/mcp?a=1', 'query'],
    ['https://u:p@store.example/mcp', 'userinfo'],
    ['ftp://store.example/mcp', 'https_required'],
    ['nope', 'unparsable'],
    [`https://store.example/${'x'.repeat(512)}`, 'too_long'],
  ])('refuses %s: %s', (uri, reason) => {
    expect(identifierProblem(uri)?.reason).toBe(reason);
  });

  it.each([
    ['https://Store.Example/mcp', 'https://store.example/mcp'],
    ['https://store.example:443/mcp', 'https://store.example/mcp'],
    ['https://store.example/', 'https://store.example'],
  ])('says how to write %s', (uri, canonical) => {
    expect(identifierProblem(uri)).toEqual({ reason: 'not_canonical', canonical });
  });

  it('leaves an http identifier to the server, which needs trust for it', () => {
    expect(identifierProblem('http://10.0.0.1/mcp')).toBeNull();
    expect(isHttpIdentifier(' http://127.0.0.1:8080/mcp ')).toBe(true);
    expect(isHttpIdentifier('http://10.0.0.1/mcp')).toBe(true);
    expect(isHttpIdentifier('https://127.0.0.1/mcp')).toBe(false);
    expect(isHttpIdentifier('nope')).toBe(false);
  });
});

describe('scopes', () => {
  it('applies the token grammar and the built-in names', () => {
    expect(scopeProblem('posts:read')).toBeNull();
    expect(scopeProblem('')).toBe('required');
    expect(scopeProblem('has space')).toBe('format');
    expect(scopeProblem('a"b')).toBe('format');
    expect(scopeProblem('x'.repeat(129))).toBe('format');
    expect(scopeProblem('openid')).toBe('reserved');
    expect(scopeProblem('offline_access')).toBe('reserved');
  });

  it('drops empty rows, marks repeats, and flags an empty list', () => {
    const list = checkScopes(['posts:read', '', ' posts:read ', 'email']);
    expect(list.scopes).toEqual(['posts:read', 'posts:read', 'email']);
    expect(list.rows).toEqual([0, 2, 3]);
    expect(list.problems).toEqual([null, null, 'duplicate', 'reserved']);
    expect(checkScopes(['']).problems).toEqual(['required']);
    expect(checkScopes([]).problems).toEqual(['required']);
  });

  it('names the message of each problem', () => {
    expect(scopeMessageKey('required')).toBe('rs.scopes_required');
    expect(scopeMessageKey('format')).toBe('apperror.RSV004_format');
  });
});

describe('verification', () => {
  it('shows trust over the check state', () => {
    expect(verificationName(ResourceServerState.VERIFIED, true)).toBe('trusted');
    expect(verificationName(ResourceServerState.VERIFIED, false)).toBe('verified');
    expect(verificationName(ResourceServerState.FAILED, false)).toBe('failed');
    expect(verificationName(ResourceServerState.PENDING, false)).toBe('pending');
    expect(verificationName(ResourceServerState.UNSPECIFIED, false)).toBe('pending');
    expect(verificationDot('verified')).toBe('status-success');
    expect(verificationDot('trusted')).toBe('status-warning');
  });

  it('explains a failure with the values the server sent', () => {
    const rs = { identifier: 'https://store.example/mcp' };
    const issuer = 'https://auth.example/p/acme';
    const mismatch = create(VerificationFailureSchema, { reason: 'resource_mismatch', resource: 'https://store.example/mcp/' });
    expect(failureText(mismatch, rs, issuer, t)).toBe(`rs.reason_resource_mismatch|${JSON.stringify({ status: 0, got: 'https://store.example/mcp/', uri: rs.identifier, issuer })}`);
    expect(failureText(create(VerificationFailureSchema, { reason: 'weird' }), rs, issuer, t)).toBe('rs.reason_unknown');
  });

  it('has copy for every state and every failure reason in the lookups', () => {
    const keys = [...Object.values(verificationStates).flatMap(s => [s.labelKey, s.bodyKey]), ...Object.values(failureReasons), 'rs.reason_unknown'];
    const enUS = { ...baseEnUS, ...consoleEnUS };
    expect(keys.filter(k => !(k in enUS))).toEqual([]);
  });

  it('writes the metadata the server must publish', () => {
    expect(JSON.parse(metadataSnippet('https://store.example/mcp', 'https://auth.example/p/acme'))).toEqual({
      resource: 'https://store.example/mcp',
      authorization_servers: ['https://auth.example/p/acme'],
    });
  });

  it('says how long ago a check ran', () => {
    const now = new Date('2026-10-20T12:00:00Z');
    const fmt = new Intl.RelativeTimeFormat('en-US', { numeric: 'auto' });
    expect(relativeTime(new Date('2026-10-20T10:00:00Z'), 'en-US', now)).toBe(fmt.format(-2, 'hour'));
    expect(relativeTime(new Date('2026-10-20T11:55:00Z'), 'en-US', now)).toBe(fmt.format(-5, 'minute'));
    expect(relativeTime(new Date('2026-10-17T12:00:00Z'), 'en-US', now)).toBe(fmt.format(-3, 'day'));
    expect(relativeTime(new Date('2026-10-20T11:59:30Z'), 'en-US', now)).toBe(fmt.format(0, 'second'));
    expect(relativeTime(new Date(), 'en-US')).toBe(fmt.format(0, 'second'));
  });
});
