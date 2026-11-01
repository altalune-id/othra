export interface QuickstartInput {
  issuer: string;
  clientId: string;
  redirectUri: string;
  secret?: string;
}

// quickstartEnv is the .env an app needs to sign in with this client; the secret is in it only while it is known.
export function quickstartEnv(q: QuickstartInput): Record<string, string> {
  const env: Record<string, string> = { OTHRA_ISSUER: q.issuer, OTHRA_CLIENT_ID: q.clientId };
  if (q.redirectUri) {
    env.OTHRA_REDIRECT_URI = q.redirectUri;
  }
  if (q.secret) {
    env.OTHRA_CLIENT_SECRET = q.secret;
  }
  return env;
}

function base64url(bytes: Uint8Array): string {
  return btoa(String.fromCharCode(...bytes)).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '');
}

function random(): string {
  return base64url(crypto.getRandomValues(new Uint8Array(32)));
}

// tryAuthorizeUrl is a real sign-in request for this client, with a fresh PKCE S256 challenge, state and nonce.
export async function tryAuthorizeUrl(q: QuickstartInput): Promise<string> {
  const digest = await crypto.subtle.digest('SHA-256', new TextEncoder().encode(random()));
  const url = new URL(`${q.issuer.replace(/\/+$/, '')}/oauth2/authorize`);
  url.search = new URLSearchParams({
    response_type: 'code',
    client_id: q.clientId,
    redirect_uri: q.redirectUri,
    scope: 'openid profile email',
    state: random(),
    nonce: random(),
    code_challenge: base64url(new Uint8Array(digest)),
    code_challenge_method: 'S256',
  }).toString();
  return url.toString();
}
