# Othra v1 OAuth profile

The profile an Othra review scores against. It narrows OAuth 2.1 + OIDC Core 1.0 + RFC 9700 (and the MCP authorization spec for MCP surfaces): score what it requires, and flag anything under **Never** or **Deferred** that is live or advertised.

## Never

- Implicit or hybrid flows (`response_type` other than `code`), the password grant (ROPC), `alg=none`, `plain` PKCE.
- Wildcard or partial redirect-URI matching; tokens in URL fragments.
- Advertising a capability in discovery that does not work.
- Dynamic client registration (RFC 7591): MCP and third-party clients use CIMD instead.

## Decisions a reviewer needs

- **Issuer per project.** Every project is its own issuer at `<public URL>/p/<handle>`, with its own signing keys, JWKS, clients, end users, resource servers, upstream IdPs and branding. End users are per project: the same email in two projects is two users.
- **No server-side authorize-flow store.** The authorize request stays in the URL across the hosted pages, and every hop re-validates it at `/oauth2/authorize`. Hosted forms carry a CSRF token; a magic link stores the authorize URL in its own row, and an upstream IdP round trip keeps it in its sealed state cookie. `return_to` is always re-checked: same origin, and an authorize URL under this issuer or a portal path.
- **CIMD, not DCR.** A `client_id` that is an HTTPS URL is a Client ID Metadata Document. Othra fetches it with an SSRF-safe client, validates and caches it, and admits it per resource server (allow-listed hosts, pinned scopes). `client_id_metadata_document_supported` is always in discovery, and true only while a verified resource server of the project admits CIMD clients.
- **Resource-server ownership is RFC 9728 verification.** Othra fetches `<origin>/.well-known/oauth-protected-resource<path>` for the registered URI and requires `resource` to equal it exactly and `authorization_servers` to contain this project's issuer. It re-checks on a schedule; a failed re-check marks the server unverified. Only verified servers can be a token audience or admit CIMD clients. Selfhosted adds an admin "trust without verification" switch. There is no custom challenge endpoint.
- **Passwordless end users.** Every project signs end users in by magic link or an upstream IdP (Google). An email collision never auto-links: linking goes through a magic link to the existing account's inbox. Passwords exist only for console admins.
- **Postgres holds every durable token artifact** (codes, refresh families, sessions, consents, keys, personal tokens), hashed where it is a secret.

## URL map

Under each issuer `/p/<handle>`:

| Path                                                                                                 | Purpose                                                                                    |
| ---------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------ |
| `/.well-known/openid-configuration`                                                                  | OIDC discovery                                                                             |
| `/oauth2/authorize`                                                                                  | Authorization endpoint; leads to sign-in, terms and consent, then the code                 |
| `/oauth2/token`, `/oauth2/revoke`, `/oauth2/introspect`                                              | Token, revocation (RFC 7009), introspection (RFC 7662)                                     |
| `/oauth2/userinfo`, `/oauth2/jwks.json`, `/oauth2/end-session`                                       | OIDC endpoints                                                                             |
| `/login`, `/login/sent`, `/login/resend`, `/login/verify`, `/login/select`, `/login/signout`         | Hosted sign-in: magic link, upstream IdP buttons, account picker, sign-out                 |
| `/federation/<upstream>/start`, `/federation/<upstream>/continue`, `/federation/<upstream>/callback` | Upstream IdP sign-in (start is POST with CSRF; continue is the GET hop to the upstream)    |
| `/federation/linking`                                                                                | The email-collision page: a magic link to the existing account links the upstream identity |
| `/terms`, `/consent`, `/signed-out`                                                                  | Terms acceptance, scope consent, the signed-out end state                                  |
| `/account/`                                                                                          | End-user portal (SPA)                                                                      |
| `/api/`                                                                                              | Portal Connect API (`account.v1`), end-user session cookie, project-bound                  |

RFC 8414 metadata is also served at `/.well-known/oauth-authorization-server/p/<handle>`. End-user cookies are scoped to the issuer path, so sessions are per project.

## Required (built)

- **Discovery** on both metadata paths; `issuer` equals the ID token `iss`; only working features advertised.
- **Authorization code + PKCE:** `response_type=code` only; S256 for every client; `state` round-trips on success and error; `iss` on every authorization response, success and error (RFC 9207), with `authorization_response_iss_parameter_supported: true`. Codes are single use, hashed, 60 seconds by default, bound to client, redirect URI, PKCE challenge, user and session; a replayed code revokes what it issued.
- **Redirect URIs:** exact string match, HTTPS except loopback; loopback (`127.0.0.1`, `[::1]`) matches on any port (RFC 8252 §7.3); checked again at `/token`.
- **Request parameters:** `scope`, `nonce` (echoed), `prompt` (`none`, `login`, `consent`, `select_account`), `login_hint`, `max_age` (→ `auth_time`), `ui_locales`, `resource`. Unknown parameters ignored.
- **Token endpoint:** grants `authorization_code` and `refresh_token`; client auth `client_secret_basic`, `client_secret_post`, `none` (public clients only); `Cache-Control: no-store`; RFC error codes; `invalid_client` is 401 with `WWW-Authenticate`.
- **Refresh tokens:** opaque, hashed, in families bound to user, client, resource, scopes and session; rotated on every use; reuse of a consumed token revokes the family. Revocation always answers 200 and revokes the family.
- **ID tokens:** RS256 JWT with `kid`; 5 minutes by default, at most 10 (`oauth.idTokenTTL`); `iss sub aud exp iat nonce auth_time at_hash sid` (and `azp` when needed); `email`, `email_verified`, `name` when their scopes are granted. `sub` is the end user's stable public id, whatever the sign-in method.
- **Access tokens:** RFC 9068 JWT (`typ=at+jwt`), ≤ 15 minutes, one resource per token, `aud` = the resource URI (RFC 8707), scopes ⊆ client ∩ resource-server scopes.
- **Userinfo:** bearer, GET and POST, claims gated by scope.
- **JWKS:** public keys only, with `kid`, `alg`, `use=sig`; rotation keeps retired keys published for an overlap window.
- **Consent:** a scope-consent page for every third-party or CIMD client, remembered per user × client × resource and revocable in the portal. First-party clients may use implicit consent. Terms consent per project `terms_version` sits between sign-in and the code.
- **End-session:** RP-initiated logout with `id_token_hint`, `client_id`, an allow-listed `post_logout_redirect_uri` and `state`; ends the user session at Othra.
- **Introspection:** client auth required; `{"active": false}` for unknown or foreign tokens; advertised only where personal tokens are enabled. **Personal tokens** are opaque, hashed, shown once, expire within 90 days and are validated only through introspection.
- **Security baseline:** HTTPS outside loopback; `frame-ancestors 'none'` and `X-Frame-Options: DENY` on hosted pages; `Referrer-Policy: no-referrer` where authorize parameters or tokens appear; CSRF on every hosted form; crypto-random `state` and `nonce`; rate limits on authorize, the hosted forms, magic-link issue and redeem, token, revocation, userinfo and introspection; client secrets server-generated, stored as a hash and compared in constant time; signing keys and upstream IdP secrets sealed at rest.

## Deferred (not built: must be neither advertised nor reachable)

- The device authorization grant (RFC 8628).
- The `client_credentials` grant and `private_key_jwt` client authentication.
- ES256 and EdDSA signing; RS256 is the only advertised algorithm.
- PAR (RFC 9126), DPoP, back-channel logout.
- `picture` and `locale` claims; an `amr` claim.
- Upstream IdPs other than Google; Google Workspace allowed domains (`hd`).
- Custom issuer domains; change of email in the portal.
- The console signing in through the reserved `/p/system` project as an ordinary relying party; console admins sign in with Othra's own login today.
