import type { ResourceServer, VerificationFailure } from '#gen/resourceserver/v1/resourceserver_pb';
import { ResourceServerState } from '#gen/resourceserver/v1/resourceserver_pb';

export type VerificationName = 'pending' | 'verified' | 'failed' | 'trusted';

type Translate = (key: string, params?: Record<string, unknown>) => string;

export interface VerificationStateSpec {
  dot: string;
  labelKey: string;
  bodyKey: string;
}

// verificationStates gives each state its status dot, its name and the card's body copy.
export const verificationStates: Record<VerificationName, VerificationStateSpec> = {
  pending: { dot: 'status-neutral', labelKey: 'rs.state_pending', bodyKey: 'rs.pending_body' },
  verified: { dot: 'status-success', labelKey: 'rs.state_verified', bodyKey: 'rs.verified_body' },
  failed: { dot: 'status-error', labelKey: 'rs.state_failed', bodyKey: 'rs.failed_body' },
  trusted: { dot: 'status-warning', labelKey: 'rs.state_trusted', bodyKey: 'rs.trusted_body' },
};

// failureReasons maps each RFC 9728 check failure the server reports onto its copy.
export const failureReasons: Record<string, string> = {
  not_found: 'rs.reason_not_found',
  status: 'rs.reason_status',
  redirect: 'rs.reason_redirect',
  not_json: 'rs.reason_not_json',
  too_large: 'rs.reason_too_large',
  resource_mismatch: 'rs.reason_resource_mismatch',
  issuer_missing: 'rs.reason_issuer_missing',
  timeout: 'rs.reason_timeout',
  network: 'rs.reason_network',
  blocked: 'rs.reason_blocked',
};

// verificationName is the state the console shows: trust wins over the RFC 9728 check.
export function verificationName(state: ResourceServerState, trusted: boolean): VerificationName {
  if (trusted) {
    return 'trusted';
  }
  if (state === ResourceServerState.VERIFIED) {
    return 'verified';
  }
  return state === ResourceServerState.FAILED ? 'failed' : 'pending';
}

export function verificationDot(name: VerificationName): string {
  return verificationStates[name].dot;
}

// failureText explains why a check failed, naming the values the server sent.
export function failureText(failure: VerificationFailure, rs: Pick<ResourceServer, 'identifier'>, issuer: string, t: Translate): string {
  const key = failureReasons[failure.reason];
  if (!key) {
    return t('rs.reason_unknown');
  }
  return t(key, { status: failure.status, got: failure.resource, uri: rs.identifier, issuer });
}

// metadataSnippet is the part of the RFC 9728 document that makes the check pass.
export function metadataSnippet(identifier: string, issuer: string): string {
  return JSON.stringify({ resource: identifier, authorization_servers: [issuer] }, null, 2);
}

const units: [Intl.RelativeTimeFormatUnit, number][] = [
  ['day', 86_400_000],
  ['hour', 3_600_000],
  ['minute', 60_000],
];

// relativeTime says how long ago a moment was, in the largest whole unit, or "now" under a minute.
export function relativeTime(when: Date, locale: string, now: Date = new Date()): string {
  const fmt = new Intl.RelativeTimeFormat(locale, { numeric: 'auto' });
  const diff = when.getTime() - now.getTime();
  for (const [unit, ms] of units) {
    if (Math.abs(diff) >= ms) {
      return fmt.format(Math.round(diff / ms), unit);
    }
  }
  return fmt.format(0, 'second');
}
