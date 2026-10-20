import type { ResourceServer, VerificationFailure } from '~~/gen/resourceserver/v1/resourceserver_pb';
import type { ResourceServerListLabels } from './ResourceServerList.vue';
import type { ScopeTableLabels } from './ScopeTable.vue';
import type { VerificationName } from './verification';
import type { VerificationCardLabels } from './VerificationCard.vue';
import { timestampDate } from '@bufbuild/protobuf/wkt';
import { failureText, verificationStates } from './verification';

type Translate = (key: string, params?: Record<string, unknown>) => string;

export function resourceServerListLabels(t: Translate): ResourceServerListLabels {
  return { name: t('common.name'), identifier: t('rs.uri'), status: t('rs.col_verification'), checked: t('rs.col_checked') };
}

export function scopeTableLabels(t: Translate): ScopeTableLabels {
  return { legend: t('rs.scopes'), help: t('rs.scopes_help'), scope: t('rs.scope_n'), remove: t('rs.scope_remove'), add: t('rs.add_scope') };
}

export function verificationCardLabels(t: Translate): VerificationCardLabels {
  return {
    title: t('rs.verify_title'),
    help: t('rs.verify_help'),
    metadataUrl: t('rs.metadata_url'),
    lastCheck: t('rs.last_check'),
    verifiedAt: t('rs.verified_at'),
    mustInclude: t('rs.must_include'),
    copySnippet: t('rs.copy_snippet'),
    verifyNow: t('rs.verify_now'),
    verifying: t('rs.verifying'),
    trust: t('rs.trust'),
    trustHelp: t('rs.trust_help'),
    untrustHelp: t('rs.untrust_body'),
    httpLocked: t('rs.http_locked'),
    copyLabel: t('common.copy_label', { label: '{label}' }),
    openLabel: t('common.open_new_tab', { label: '{label}' }),
  };
}

export interface VerificationText {
  body: string;
  lastCheck: string;
  verifiedAt: string;
  failure: string;
  recheck: string;
  passed: string;
}

export interface VerifyOutcomeText {
  passed: boolean;
  failure: VerificationFailure | null;
}

// verificationText words the verification card once per fact: a fresh Verify now result wins over the stored last error.
export function verificationText(rs: ResourceServer, name: VerificationName, issuer: string, t: Translate, format: (when: Date) => string, outcome: VerifyOutcomeText | null = null, now: Date = new Date()): VerificationText {
  const verifiedOn = rs.verifiedAt ? format(timestampDate(rs.verifiedAt)) : '';
  const body = name === 'verified' ? t(verificationStates[name].bodyKey, { date: verifiedOn }) : t(verificationStates[name].bodyKey);
  const out: VerificationText = { body, lastCheck: '', verifiedAt: name === 'verified' ? verifiedOn : '', failure: '', recheck: '', passed: '' };
  if (name === 'trusted') {
    return out;
  }
  if (outcome && !outcome.passed) {
    const reason = outcome.failure ? failureText(outcome.failure, rs, issuer, t) : t('rs.reason_unknown');
    return { ...out, failure: t('rs.last_error', { date: format(now), reason }) };
  }
  if (outcome?.passed) {
    out.passed = t('rs.verify_passed');
  }
  const reason = rs.lastError ? failureText(rs.lastError, rs, issuer, t) : '';
  const checkedAt = rs.lastError?.checkedAt ?? rs.lastCheckedAt;
  const checked = checkedAt ? format(timestampDate(checkedAt)) : '';
  if (reason && name === 'verified' && rs.consecutiveFailures > 0) {
    out.recheck = t('rs.recheck_failing', { count: rs.consecutiveFailures, reason });
  } else if (reason) {
    out.failure = t('rs.last_error', { date: checked, reason });
  }
  if (!out.failure) {
    out.lastCheck = rs.lastCheckedAt ? format(timestampDate(rs.lastCheckedAt)) : '';
  }
  return out;
}
