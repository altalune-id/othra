export interface DiscoveryUrls {
  oidc: string;
  oauth: string;
  jwks: string;
}

// discoveryUrls derives an issuer's OIDC discovery, RFC 8414 metadata and JWKS URLs; RFC 8414 §3.1 puts its well-known path at the host root.
export function discoveryUrls(issuer: string): DiscoveryUrls | null {
  let u: URL;
  try {
    u = new URL(issuer);
  } catch {
    return null;
  }
  const path = u.pathname.replace(/\/+$/, '');
  return {
    oidc: `${u.origin}${path}/.well-known/openid-configuration`,
    oauth: `${u.origin}/.well-known/oauth-authorization-server${path}`,
    jwks: `${u.origin}${path}/oauth2/jwks.json`,
  };
}
