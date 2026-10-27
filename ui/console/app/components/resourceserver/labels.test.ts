import { create } from '@bufbuild/protobuf';
import { timestampFromDate } from '@bufbuild/protobuf/wkt';
import { describe, expect, it } from 'vitest';

import { ResourceServerSchema, ResourceServerState, VerificationFailureSchema } from '#gen/resourceserver/v1/resourceserver_pb';
import { t } from '#test/nuxt-globals';
import { cimdCardLabels, introspectionSecretLabels, resourceServerListLabels, scopeTableLabels, verificationCardLabels, verificationText } from './labels';

const at = (iso: string) => timestampFromDate(new Date(iso));
const format = (when: Date) => when.toISOString().slice(0, 10);
const issuer = 'https://auth.example/p/acme';

describe('resource server labels', () => {
  it('translates the list, the scope editor and the card', () => {
    expect(resourceServerListLabels(t).checked).toBe('rs.col_checked');
    expect(scopeTableLabels(t).add).toBe('rs.add_scope');
    expect(scopeTableLabels(t).description).toBe('rs.scope_description');
    expect(cimdCardLabels(t)).toMatchObject({ title: 'rs.cimd_title', locked: 'rs.cimd_locked', firstSeen: 'links.first_seen', save: 'common.save' });
    expect(verificationCardLabels(t).copyLabel).toBe('common.copy_label|{"label":"{label}"}');
  });

  it('words a pending server with no check yet', () => {
    const rs = create(ResourceServerSchema, { identifier: 'https://store.example/mcp', state: ResourceServerState.PENDING });
    expect(verificationText(rs, 'pending', issuer, t, format)).toEqual({ body: 'rs.pending_body', lastCheck: '', verifiedAt: '', failure: '', recheck: '', passed: '' });
  });

  it('shows a failed server\'s reason and check date once, and no verified date', () => {
    const rs = create(ResourceServerSchema, {
      identifier: 'https://store.example/mcp',
      state: ResourceServerState.FAILED,
      verifiedAt: at('2026-10-18T09:00:00Z'),
      lastCheckedAt: at('2026-10-20T10:00:00Z'),
      lastError: create(VerificationFailureSchema, { reason: 'timeout', checkedAt: at('2026-10-20T10:00:00Z') }),
    });
    const text = verificationText(rs, 'failed', issuer, t, format);
    expect(text.body).toBe('rs.failed_body');
    expect(text.failure).toBe('rs.last_error|{"date":"2026-10-20","reason":"rs.reason_timeout|{\\"status\\":0,\\"got\\":\\"\\",\\"uri\\":\\"https://store.example/mcp\\",\\"issuer\\":\\"https://auth.example/p/acme\\"}"}');
    expect([text.lastCheck, text.verifiedAt, text.recheck]).toEqual(['', '', '']);
  });

  it('shows a verified server\'s dates, and its misses once as a warning', () => {
    const clean = create(ResourceServerSchema, { state: ResourceServerState.VERIFIED, verifiedAt: at('2026-10-18T09:00:00Z'), lastCheckedAt: at('2026-10-20T10:00:00Z') });
    expect(verificationText(clean, 'verified', issuer, t, format)).toMatchObject({ body: 'rs.verified_body|{"date":"2026-10-18"}', lastCheck: '2026-10-20', verifiedAt: '2026-10-18', failure: '' });
    const missing = create(ResourceServerSchema, { ...clean, consecutiveFailures: 2, lastError: create(VerificationFailureSchema, { reason: 'odd' }) });
    const text = verificationText(missing, 'verified', issuer, t, format);
    expect(text.recheck).toBe('rs.recheck_failing|{"count":2,"reason":"rs.reason_unknown"}');
    expect([text.failure, text.lastCheck]).toEqual(['', '2026-10-20']);
  });

  it('puts a fresh Verify now failure in place of the stored one, and notes a pass', () => {
    const rs = create(ResourceServerSchema, {
      identifier: 'https://store.example/mcp',
      state: ResourceServerState.PENDING,
      lastCheckedAt: at('2026-10-19T10:00:00Z'),
      lastError: create(VerificationFailureSchema, { reason: 'timeout' }),
    });
    const now = new Date('2026-10-20T12:00:00Z');
    const failed = verificationText(rs, 'pending', issuer, t, format, { passed: false, failure: create(VerificationFailureSchema, { reason: 'not_found' }) }, now);
    expect(failed.failure).toMatch(/^rs\.last_error\|\{"date":"2026-10-20","reason":"rs\.reason_not_found/);
    expect([failed.lastCheck, failed.passed]).toEqual(['', '']);
    expect(verificationText(rs, 'pending', issuer, t, format, { passed: false, failure: null }, now).failure).toBe('rs.last_error|{"date":"2026-10-20","reason":"rs.reason_unknown"}');
    const passed = verificationText(create(ResourceServerSchema, { state: ResourceServerState.VERIFIED }), 'verified', issuer, t, format, { passed: true, failure: null });
    expect([passed.passed, passed.failure, passed.lastCheck]).toEqual(['rs.verify_passed', '', '']);
  });

  it('leaves out the date when the server sent none, and ignores errors on a trusted server', () => {
    const failing = create(ResourceServerSchema, { lastError: create(VerificationFailureSchema, { reason: 'odd' }) });
    expect(verificationText(failing, 'pending', issuer, t, format).failure).toBe('rs.last_error|{"date":"","reason":"rs.reason_unknown"}');
    expect(verificationText(failing, 'trusted', issuer, t, format, { passed: false, failure: null })).toEqual({ body: 'rs.trusted_body', lastCheck: '', verifiedAt: '', failure: '', recheck: '', passed: '' });
  });

  it('words the introspection secret for a resource server, never a client (M8 design 10)', () => {
    const l = introspectionSecretLabels(t);
    expect(l.card.title).toBe('rs.pat_secret_title');
    expect(l.rotate).toMatchObject({ title: 'rs.pat_rotate_title', body: 'rs.pat_rotate_body', nowWarning: 'rs.pat_now_warning' });
    expect(l.dialog).toMatchObject({ body: 'rs.pat_dialog_body', ownerId: 'rs.pat_server_id', secret: 'rs.pat_secret_label' });
    expect(l.revokeTitle).toBe('rs.pat_revoke_title');
  });
});
