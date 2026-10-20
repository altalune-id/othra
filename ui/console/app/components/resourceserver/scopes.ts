export type ScopeReason = 'required' | 'format' | 'reserved' | 'duplicate';

export const reservedScopes = ['openid', 'profile', 'email', 'offline_access'];
export const maxScopes = 64;
const token = /^[\x21\x23-\x5B\x5D-\x7E]{1,128}$/;

// scopeProblem applies the RFC 6749 §3.3 scope-token grammar, the 128-character limit and the built-in names.
export function scopeProblem(scope: string): ScopeReason | null {
  if (scope === '') {
    return 'required';
  }
  if (!token.test(scope)) {
    return 'format';
  }
  return reservedScopes.includes(scope) ? 'reserved' : null;
}

export interface ScopeList {
  scopes: string[];
  rows: number[];
  problems: (ScopeReason | null)[];
}

// checkScopes checks every row, drops empty ones and marks repeats; a list with no scope flags its first row.
export function checkScopes(rows: string[]): ScopeList {
  const out: ScopeList = { scopes: [], rows: [], problems: rows.map(() => null) };
  const seen = new Set<string>();
  rows.forEach((row, i) => {
    const scope = row.trim();
    if (scope === '') {
      return;
    }
    out.scopes.push(scope);
    out.rows.push(i);
    out.problems[i] = seen.has(scope) ? 'duplicate' : scopeProblem(scope);
    seen.add(scope);
  });
  if (out.scopes.length === 0) {
    out.problems[0] = 'required';
  }
  return out;
}

// scopeMessageKey is the locale key that explains a scope problem.
export function scopeMessageKey(reason: ScopeReason): string {
  return reason === 'required' ? 'rs.scopes_required' : `apperror.RSV004_${reason}`;
}
