import type { ClientKindName } from './kinds';
import { clientKinds } from './kinds';

export type UriReason = 'required' | 'unparsable' | 'fragment' | 'wildcard' | 'userinfo' | 'dot_segments' | 'https_required' | 'loopback_only' | 'port_ignored' | 'not_canonical' | 'too_long' | 'duplicate';

export interface UriProblem {
  reason: UriReason;
  canonical?: string;
}

export const maxRedirectUris = 20;
const maxLength = 2000;
const loopbackHosts = new Set(['127.0.0.1', '[::1]', 'localhost']);
const defaultPorts: Record<string, string> = { 'http:': '80', 'https:': '443' };

function authorityOf(raw: string): string {
  const start = raw.indexOf('//');
  if (start < 0) {
    return '';
  }
  const rest = raw.slice(start + 2);
  const end = rest.search(/[/?#]/);
  return end < 0 ? rest : rest.slice(0, end);
}

function pathOf(raw: string): string {
  const start = raw.indexOf('//');
  const rest = start < 0 ? raw : raw.slice(start + 2);
  const slash = rest.indexOf('/');
  if (slash < 0) {
    return '';
  }
  const path = rest.slice(slash);
  const end = path.search(/[?#]/);
  return end < 0 ? path : path.slice(0, end);
}

function nonCanonical(raw: string, url: URL): UriProblem | null {
  const authority = authorityOf(raw);
  const explicitDefault = authority.endsWith(`:${defaultPorts[url.protocol] ?? ''}`);
  if (authority === url.host && !explicitDefault && raw.startsWith(url.protocol)) {
    return null;
  }
  const scheme = raw.indexOf('//');
  return { reason: 'not_canonical', canonical: `${url.protocol}//${url.host}${raw.slice(scheme + 2 + authority.length)}` };
}

// redirectUriProblem mirrors the server's redirect URI rules for a client kind; the server stays the authority.
export function redirectUriProblem(kind: ClientKindName, raw: string): UriProblem | null {
  if (raw === '') {
    return { reason: 'required' };
  }
  if (raw.length > maxLength) {
    return { reason: 'too_long' };
  }
  if (raw.includes('*')) {
    return { reason: 'wildcard' };
  }
  if (raw.includes('#')) {
    return { reason: 'fragment' };
  }
  let url: URL;
  try {
    url = new URL(raw);
  } catch {
    return { reason: 'unparsable' };
  }
  if (url.username || url.password || authorityOf(raw).includes('@')) {
    return { reason: 'userinfo' };
  }
  if (/\/\.{1,2}(?:\/|$)/.test(pathOf(raw))) {
    return { reason: 'dot_segments' };
  }
  const rule = clientKinds[kind].uris;
  const loopbackHttp = url.protocol === 'http:' && loopbackHosts.has(url.hostname);
  if (!(loopbackHttp || (rule.https && url.protocol === 'https:'))) {
    return { reason: rule.refusal };
  }
  if (loopbackHttp && rule.loopbackPort === 'required' && url.port === '') {
    return { reason: rule.refusal };
  }
  if (loopbackHttp && rule.loopbackPort === 'forbidden' && url.port !== '') {
    return { reason: 'port_ignored' };
  }
  if (!url.host) {
    return { reason: 'unparsable' };
  }
  return nonCanonical(raw, url);
}

// usesLocalhost reports a URI on the host name localhost, which the console warns about but saves.
export function usesLocalhost(raw: string): boolean {
  try {
    return new URL(raw).hostname === 'localhost';
  } catch {
    return false;
  }
}

export interface UriList {
  uris: string[];
  rows: number[];
  problems: (UriProblem | null)[];
}

// checkUriList checks every row, drops empty ones, and marks repeats; a required list with no URI flags its first row.
export function checkUriList(kind: ClientKindName, rows: string[], required: boolean): UriList {
  const out: UriList = { uris: [], rows: [], problems: rows.map(() => null) };
  const seen = new Set<string>();
  rows.forEach((row, i) => {
    const uri = row.trim();
    if (uri === '') {
      return;
    }
    out.uris.push(uri);
    out.rows.push(i);
    out.problems[i] = seen.has(uri) ? { reason: 'duplicate' } : redirectUriProblem(kind, uri);
    seen.add(uri);
  });
  if (required && out.uris.length === 0) {
    out.problems[0] = { reason: 'required' };
  }
  return out;
}

// serverRow finds the row a server refusal names: by the field's index, else by the echoed URI.
export function serverRow(field: string, uri: string, list: UriList): number {
  const index = /\[(\d+)\]$/.exec(field)?.[1];
  if (index !== undefined) {
    return list.rows[Number(index)] ?? -1;
  }
  const at = uri ? list.uris.indexOf(uri) : -1;
  return at < 0 ? -1 : list.rows[at]!;
}
