export type IdentifierReason = 'unparsable' | 'https_required' | 'query' | 'fragment' | 'userinfo' | 'not_canonical' | 'too_long';

export interface IdentifierProblem {
  reason: IdentifierReason;
  canonical?: string;
}

const maxLength = 512;

function parse(raw: string): URL | null {
  try {
    return new URL(raw);
  } catch {
    return null;
  }
}

// isHttpIdentifier reports an http identifier, which works only with trust without verification; the server decides which hosts.
export function isHttpIdentifier(raw: string): boolean {
  return parse(raw.trim())?.protocol === 'http:';
}

// canonicalIdentifier writes an identifier with a lowercase scheme and host, no default port and no lone trailing slash.
export function canonicalIdentifier(url: URL): string {
  const path = url.pathname === '/' ? '' : url.pathname;
  return `${url.protocol}//${url.host}${path}`;
}

// identifierProblem mirrors the server's resource server identifier rules; the server stays the authority.
export function identifierProblem(raw: string): IdentifierProblem | null {
  if (raw.length > maxLength) {
    return { reason: 'too_long' };
  }
  if (raw.includes('#')) {
    return { reason: 'fragment' };
  }
  const url = parse(raw);
  if (!url || !url.host) {
    return { reason: 'unparsable' };
  }
  if (url.username || url.password) {
    return { reason: 'userinfo' };
  }
  if (raw.includes('?')) {
    return { reason: 'query' };
  }
  if (url.protocol !== 'https:' && url.protocol !== 'http:') {
    return { reason: 'https_required' };
  }
  const canonical = canonicalIdentifier(url);
  return canonical === raw ? null : { reason: 'not_canonical', canonical };
}
