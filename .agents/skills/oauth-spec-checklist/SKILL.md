---
name: oauth-spec-checklist
description: OAuth 2.0 / OAuth 2.1 / OIDC conformance dimensions with a scoring rubric. Covers authorization servers (IdP-side — /authorize, /token, discovery, JWKS, /userinfo, /introspect, /revoke, end-session, id_token, RFC 9207 iss, RFC 8707 resource indicators, RFC 9068 JWT access tokens, CIMD client_id URLs, RFC 9728 resource-server verification, RFC 8628 device grant), OAuth clients (RP-side — PKCE, state, iss check, redirect handling, ID-token verification, refresh rotation, token storage) and resource servers (RFC 9728 metadata, WWW-Authenticate, at+jwt validation), plus the MCP Authorization spec. Load when reviewing an OAuth implementation against RFC 6749, draft-ietf-oauth-v2-1, OIDC Core 1.0, RFC 9700 or the MCP authorization spec. Companion RFC index in references/rfc-index.md — cite the section-anchor URL, not the bare RFC URL.
---

# oauth-spec-checklist

A dimension list for scoring OAuth 2.0 / 2.1 conformance. Load once per review; cite `file:line` evidence per dimension, score 0-10, give a one-line recommendation. When citing a spec, link the section anchor (e.g. `#section-4.1.2.1`), not the bare RFC URL — sections in `references/rfc-index.md` are the shortlist.

## Othra profile

In Othra, the target profile is `docs/othra-handoff/02-oauth-profile.md`, with decisions in `docs/othra-handoff/01-context.md` (O1–O20). The owner keeps that handoff pack on this machine, uncommitted; if it is missing, ask for it. The profile narrows the spec: score what it requires, flag anything it says "never" or "not in v1" that is live or advertised.

## Two questions before reading

1. **Server, client or resource server?** Server = IdP endpoints (authorize / token / discovery / JWKS / introspect / revoke / userinfo / end-session). Client = RP-side (redirect kickoff, callback handling, token storage, refresh). Resource server = an API or MCP server that accepts access tokens.
2. **Target spec?** OAuth 2.0 = RFC 6749. OAuth 2.1 = draft-ietf-oauth-v2-1 — omits implicit + ROPC ([Section 10](https://datatracker.ietf.org/doc/html/draft-ietf-oauth-v2-1#section-10)), makes PKCE mandatory for every client, requires HTTPS redirects except loopback. Default to 2.1 + OIDC Core + [RFC 9700](https://datatracker.ietf.org/doc/html/rfc9700#section-2) when ambiguous. Add the MCP authorization spec when an MCP client or server is in scope.

## Server-side dimensions

**/authorize** — [RFC 6749 Section 4.1](https://datatracker.ietf.org/doc/html/rfc6749#section-4.1)
- PKCE required (2.1: every client; 2.0: public clients per RFC 7636). Accept only `code_challenge_method=S256` ([RFC 7636 Section 4.2](https://datatracker.ietf.org/doc/html/rfc7636#section-4.2)); reject `plain`. Enforce verifier 43–128 chars ([RFC 7636 Section 4.1](https://datatracker.ietf.org/doc/html/rfc7636#section-4.1)).
- `response_type=code` only. 2.1 forbids `token` and hybrid.
- `redirect_uri`: exact string match against a registered value; HTTPS-only except loopback (`127.0.0.1`, `::1`) per [RFC 8252 Section 7.3](https://datatracker.ietf.org/doc/html/rfc8252#section-7.3). No wildcards, no fragments.
- Loopback redirects: any port accepted at request time, port ignored when matching ([RFC 8252 Section 7.3](https://datatracker.ietf.org/doc/html/rfc8252#section-7.3)); `localhost` NOT RECOMMENDED ([Section 8.3](https://datatracker.ietf.org/doc/html/rfc8252#section-8.3)).
- Unrecognized parameters ignored; a repeated parameter rejected ([RFC 6749 Section 3.1](https://datatracker.ietf.org/doc/html/rfc6749#section-3.1)).
- `state` round-tripped on success and error redirects ([RFC 6749 Section 4.1.2](https://datatracker.ietf.org/doc/html/rfc6749#section-4.1.2), [Section 10.12](https://datatracker.ietf.org/doc/html/rfc6749#section-10.12)).
- Errors returned via redirect with `error` + `error_description` + `state` when a valid `redirect_uri` is bound ([RFC 6749 Section 4.1.2.1](https://datatracker.ietf.org/doc/html/rfc6749#section-4.1.2.1)); rendered as JSON only when no redirect is trusted.
- Error codes: `invalid_request` / `unauthorized_client` / `access_denied` / `unsupported_response_type` / `invalid_scope` / `server_error` / `temporarily_unavailable`.
- Authorization code: single-use, short-lived (≤10 min per Section 4.1.2; ≤60 s recommended), bound to `client_id` + `redirect_uri` + `code_challenge` + user. Store the SHA-256 hash, not the plaintext.
- OIDC extras ([OIDC Core Section 3.1.2.1](https://openid.net/specs/openid-connect-core-1_0.html#AuthRequest)): `nonce` (required when `id_token` is issued), `prompt`, `max_age`, `login_hint`, `ui_locales`, `acr_values`.
- `prompt`: `none` shows no page and redirects `login_required` / `consent_required` / `interaction_required` / `account_selection_required` ([OIDC Core Section 3.1.2.6](https://openid.net/specs/openid-connect-core-1_0.html#AuthError)); `login` forces re-authentication; `consent` and `select_account` always show their page.
- Consent obtained before the code is issued ([OIDC Core Section 3.1.2.4](https://openid.net/specs/openid-connect-core-1_0.html#Consent)): shown to every third-party or CIMD client, remembered per user × client × resource, revocable. `offline_access` ignored unless `response_type=code` and consent covers it ([OIDC Core Section 11](https://openid.net/specs/openid-connect-core-1_0.html#OfflineAccess)).
- CSRF protection on the authorization endpoint and every hosted form (login, consent, terms, account picker) ([RFC 6749 Section 10.12](https://datatracker.ietf.org/doc/html/rfc6749#section-10.12)).

**Issuer identification** — [RFC 9207 Section 2](https://datatracker.ietf.org/doc/html/rfc9207#section-2)
- `iss` on every authorization response, success **and** error ([Section 2](https://datatracker.ietf.org/doc/html/rfc9207#section-2)); MCP authorization servers SHOULD, a later MCP revision expects MUST ([MCP Authorization Response Validation](https://modelcontextprotocol.io/specification/2026-07-28/basic/authorization#authorization-response-validation)).
- `iss` value identical to metadata `issuer`; metadata sets `authorization_response_iss_parameter_supported: true` ([Section 2.3](https://datatracker.ietf.org/doc/html/rfc9207#section-2.3), [Section 3](https://datatracker.ietf.org/doc/html/rfc9207#section-3)). Advertised but missing on any path (e.g. an error redirect) — Sev 1.

**/token** — [RFC 6749 Section 4.1.3](https://datatracker.ietf.org/doc/html/rfc6749#section-4.1.3)
- Grant types: `authorization_code`, `refresh_token`, `client_credentials`. **`password` (ROPC) omitted in 2.1** ([Section 10](https://datatracker.ietf.org/doc/html/draft-ietf-oauth-v2-1#section-10), [RFC 9700 Section 2.4](https://datatracker.ietf.org/doc/html/rfc9700#section-2.4)); flag any live code path.
- PKCE verifier check with constant-time compare ([RFC 7636 Section 4.6](https://datatracker.ietf.org/doc/html/rfc7636#section-4.6)).
- `redirect_uri` bound to the code — strict equality on exchange.
- Auth-code replay: MUST deny, SHOULD revoke tokens previously issued from that code ([OAuth 2.1 Section 4.1.3](https://datatracker.ietf.org/doc/html/draft-ietf-oauth-v2-1#section-4.1.3), [Section 7.5.3](https://datatracker.ietf.org/doc/html/draft-ietf-oauth-v2-1#section-7.5.3)). Requires `session_id` on the code row.
- Client auth ([RFC 6749 Section 2.3](https://datatracker.ietf.org/doc/html/rfc6749#section-2.3), [RFC 7523 Section 2.2](https://datatracker.ietf.org/doc/html/rfc7523#section-2.2)): `client_secret_basic` / `_post` / `_jwt`, `private_key_jwt`, `tls_client_auth`, `none`. Cross-check Basic `client_id` vs form ([Section 2.3.1](https://datatracker.ietf.org/doc/html/rfc6749#section-2.3.1)). Constant-time secret compare.
- `none` only for public clients; confidential clients MUST authenticate ([OAuth 2.1 Section 2.1](https://datatracker.ietf.org/doc/html/draft-ietf-oauth-v2-1#section-2.1), [Section 3.2.1](https://datatracker.ietf.org/doc/html/draft-ietf-oauth-v2-1#section-3.2.1)). Client secrets stored hashed.
- `client_credentials`: confidential clients only ([OAuth 2.1 Section 4.2](https://datatracker.ietf.org/doc/html/draft-ietf-oauth-v2-1#section-4.2)); no refresh token ([RFC 6749 Section 4.4.3](https://datatracker.ietf.org/doc/html/rfc6749#section-4.4.3)); audience-bound through `resource`.
- Access token: JWT ([RFC 9068](https://datatracker.ietf.org/doc/html/rfc9068#section-2), see below) OR opaque.
- Refresh rotation ([RFC 6749 Section 6](https://datatracker.ietf.org/doc/html/rfc6749#section-6), [OAuth 2.1 Section 4.3](https://datatracker.ietf.org/doc/html/draft-ietf-oauth-v2-1#section-4.3), [RFC 9700 Section 4.14](https://datatracker.ietf.org/doc/html/rfc9700#section-4.14)): new token on every use, old invalidated; **reuse of an already-consumed token revokes the whole lineage / family**, not just the presented token. Track `session_id` + generation counter. Public clients MUST rotate (or sender-constrain).
- Response: `Cache-Control: no-store`, `Pragma: no-cache`, `token_type=Bearer`, `expires_in` ([Section 5.1](https://datatracker.ietf.org/doc/html/rfc6749#section-5.1)).
- Errors: `invalid_request` / `invalid_client` (401 + [`WWW-Authenticate`](https://datatracker.ietf.org/doc/html/rfc6750#section-3)) / `invalid_grant` / `unauthorized_client` / `unsupported_grant_type` / `invalid_scope` ([Section 5.2](https://datatracker.ietf.org/doc/html/rfc6749#section-5.2)).

**Resource indicators** — [RFC 8707 Section 2](https://datatracker.ietf.org/doc/html/rfc8707#section-2)
- `resource` accepted at /authorize ([Section 2.1](https://datatracker.ietf.org/doc/html/rfc8707#section-2.1)) and /token for every grant ([Section 2.2](https://datatracker.ietf.org/doc/html/rfc8707#section-2.2)). Value: absolute URI, no fragment; unknown or unacceptable → `invalid_target`.
- Token-request `resource` ⊆ what was granted at /authorize (code and refresh grants); a refresh family stays bound to its original grant.
- Issued token `aud` = the resource identifier, compared as an exact string; scopes down-scoped to what that resource takes. One resource per token if the profile says so.
- MCP clients MUST send `resource` on both requests ([MCP Resource Parameter Implementation](https://modelcontextprotocol.io/specification/2026-07-28/basic/authorization#resource-parameter-implementation)); an AS serving MCP that ignores it mints unbound tokens — Sev 1.

**JWT access tokens** — [RFC 9068 Section 2](https://datatracker.ietf.org/doc/html/rfc9068#section-2)
- Header `typ=at+jwt`; signed, never `alg=none`; RS256 supported ([Section 2.1](https://datatracker.ietf.org/doc/html/rfc9068#section-2.1)).
- Claims REQUIRED: `iss`, `exp`, `aud`, `sub`, `client_id`, `iat`, `jti`; `scope` when scoped ([Section 2.2](https://datatracker.ietf.org/doc/html/rfc9068#section-2.2), [Section 2.2.3](https://datatracker.ietf.org/doc/html/rfc9068#section-2.2.3)). `aud` per [Section 3](https://datatracker.ietf.org/doc/html/rfc9068#section-3) (from `resource`).
- `iss` and `jwks_uri` consistent across RFC 8414 and OIDC discovery ([Section 4](https://datatracker.ietf.org/doc/html/rfc9068#section-4)). An ID token must not validate as an access token (distinct `typ`) ([Section 5](https://datatracker.ietf.org/doc/html/rfc9068#section-5)).
- Short-lived; no personal data beyond what the resource needs ([Section 6](https://datatracker.ietf.org/doc/html/rfc9068#section-6)).

**Discovery + JWKS** — [RFC 8414](https://datatracker.ietf.org/doc/html/rfc8414#section-2) + [OIDC Discovery Section 4](https://openid.net/specs/openid-connect-discovery-1_0.html#ProviderConfig)
- Publish `/.well-known/oauth-authorization-server` AND `/.well-known/openid-configuration` when OIDC is advertised. For an issuer with a path, RFC 8414 inserts the well-known segment before the path ([Section 3.1](https://datatracker.ietf.org/doc/html/rfc8414#section-3.1)); OIDC appends it ([Section 5](https://datatracker.ietf.org/doc/html/rfc8414#section-5)). MCP clients try RFC 8414 insertion, then OIDC insertion, then OIDC append ([MCP Authorization Server Metadata Discovery](https://modelcontextprotocol.io/specification/2026-07-28/basic/authorization/authorization-server-discovery#authorization-server-metadata-discovery)).
- `issuer` stable; identical to the issuer used to build the metadata URL ([RFC 8414 Section 3.3](https://datatracker.ietf.org/doc/html/rfc8414#section-3.3)); matches ID-token and access-token `iss`. Don't advertise `openid` in `scopes_supported` or list `id_token_signing_alg_values_supported` without a live ID-token code path — Sev 1.
- `code_challenge_methods_supported` present in **both** documents; MCP clients refuse to proceed without it ([MCP Authorization Code Protection](https://modelcontextprotocol.io/specification/2026-07-28/basic/authorization/security-considerations#authorization-code-protection)).
- Only advertise what works: `device_authorization_endpoint`, `registration_endpoint`, `client_id_metadata_document_supported`, `authorization_response_iss_parameter_supported`, grant types and auth methods each need a live code path.
- `jwks_uri` publishes public-only keys with `kid` ([RFC 7517 Section 4.5](https://datatracker.ietf.org/doc/html/rfc7517#section-4.5)), `alg`, `use=sig`. `alg=none` MUST NOT appear. A retired `kid` stays published for at least the longest lifetime of a token it signed.
- `id_token_signing_alg_values_supported` includes `RS256` ([OIDC Discovery Section 3](https://openid.net/specs/openid-connect-discovery-1_0.html#ProviderMetadata)); other algs optional per client.
- CORS: public / permissive on discovery + JWKS. Cache-Control: `public, max-age=…` acceptable; `ETag` a nice-to-have.

**OIDC extras** — [OIDC Core Section 2](https://openid.net/specs/openid-connect-core-1_0.html#IDToken)
- `id_token`: signed JWT with `iss`, `sub`, `aud`, `exp`, `iat`, `nonce` echoed from `/authorize`, `at_hash` when issued with an access token, `azp` when audience ≠ client_id, `auth_time` when `max_age` was requested, `sid` when sessions are tracked ([OIDC Front-Channel Logout Section 3](https://openid.net/specs/openid-connect-frontchannel-1_0.html#OPLogout)).
- Scope claims gated per [OIDC Core Section 5.4](https://openid.net/specs/openid-connect-core-1_0.html#ScopeClaims); the Othra profile also puts granted `email`, `email_verified`, `name`, `picture`, `locale` in the ID token.
- [`/userinfo`](https://openid.net/specs/openid-connect-core-1_0.html#UserInfo): Bearer-authenticated; returns stable `sub`; scope-gated `profile` / `email` / `address` / `phone` claims. Support GET + POST + CORS.
- [`end_session_endpoint`](https://openid.net/specs/openid-connect-rpinitiated-1_0.html#RPLogout): `id_token_hint`, `client_id` (must match the hint's audience), `post_logout_redirect_uri` allow-listed per client, `state`; ends the session at the AS.

**Introspect ([RFC 7662](https://datatracker.ietf.org/doc/html/rfc7662#section-2.2)) + Revoke ([RFC 7009](https://datatracker.ietf.org/doc/html/rfc7009#section-2.2))**
- Introspect: client-auth required; foreign-client tokens → `{active:false}` (no cross-client probing); `Cache-Control: no-store`. Response: `active`, `scope`, `client_id`, `exp`, `iat`, `sub`, `aud`, `iss`, `token_type`, `jti`.
- Revoke: idempotent 200 (even for unknown tokens per Section 2.2); accepts access + refresh with `token_type_hint`; access-token revoke cascades to session; client-auth required for confidential clients.
- Refresh-token revoke invalidates the access tokens of the same grant ([RFC 7009 Section 2.1](https://datatracker.ietf.org/doc/html/rfc7009#section-2.1)); Othra revokes the whole family. Portal "Disconnect" revokes the consent and every family of that client.
- Othra personal tokens: opaque, stored hashed, shown once, ≤ 90 days, bound to one resource; validated only through introspection, `{active:false}` once revoked or expired ([RFC 7662 Section 2.2](https://datatracker.ietf.org/doc/html/rfc7662#section-2.2)).

**Hosted pages + security baseline** — [RFC 9700 Section 2](https://datatracker.ietf.org/doc/html/rfc9700#section-2)
- HTTPS outside loopback, HSTS ([RFC 6797 Section 7.1](https://datatracker.ietf.org/doc/html/rfc6797#section-7.1)).
- `frame-ancestors 'none'` + `X-Frame-Options: DENY` on authorize, login, consent and terms pages ([RFC 9700 Section 4.16](https://datatracker.ietf.org/doc/html/rfc9700#section-4.16)).
- `Referrer-Policy: no-referrer` and no third-party resources on pages that carry authorize parameters or codes ([RFC 9700 Section 4.2.4](https://datatracker.ietf.org/doc/html/rfc9700#section-4.2.4)); `Cache-Control: no-store` on every code or token response.
- Rate limits per IP and per client/email on authorize, token, magic-link issue, sign-in and introspection; codes and tokens ≥ 128 bits of entropy ([RFC 6749 Section 10.10](https://datatracker.ietf.org/doc/html/rfc6749#section-10.10)).

**Client ID Metadata Documents (CIMD)** — [draft-ietf-oauth-client-id-metadata-document-02](https://datatracker.ietf.org/doc/html/draft-ietf-oauth-client-id-metadata-document-02#section-3)
- `client_id` URL: `https`, has a path, no userinfo, no dot segments, no fragment; compared by simple string comparison ([Section 3](https://datatracker.ietf.org/doc/html/draft-ietf-oauth-client-id-metadata-document-02#section-3)). Registered `client_id`s never start with `https://` ([Section 7.1](https://datatracker.ietf.org/doc/html/draft-ietf-oauth-client-id-metadata-document-02#section-7.1)).
- Document: JSON, 200 only, `client_id` equals the fetched URL ([Section 4](https://datatracker.ietf.org/doc/html/draft-ietf-oauth-client-id-metadata-document-02#section-4)); no shared-secret auth methods, no `client_secret`, no private keys ([Section 4.1](https://datatracker.ietf.org/doc/html/draft-ietf-oauth-client-id-metadata-document-02#section-4.1)); `redirect_uri` exact match against its `redirect_uris` ([Section 4.2](https://datatracker.ietf.org/doc/html/draft-ietf-oauth-client-id-metadata-document-02#section-4.2)). MCP also requires `client_name` ([MCP Implementation Requirements](https://modelcontextprotocol.io/specification/2026-07-28/basic/authorization/client-registration#implementation-requirements)).
- Fetch: no redirects followed, failure aborts the request ([Section 5](https://datatracker.ietf.org/doc/html/draft-ietf-oauth-client-id-metadata-document-02#section-5), [5.1](https://datatracker.ietf.org/doc/html/draft-ietf-oauth-client-id-metadata-document-02#section-5.1)); never errors or invalid documents cached, cache bounded ([Section 5.2](https://datatracker.ietf.org/doc/html/draft-ietf-oauth-client-id-metadata-document-02#section-5.2)).
- SSRF: refuse URLs (the document's and any inside it) resolving to special-use addresses, checked on the resolved IP at connect time, not only on the hostname; known schemes only ([Section 8.6](https://datatracker.ietf.org/doc/html/draft-ietf-oauth-client-id-metadata-document-02#section-8.6)). Read cap ~5 KB ([Section 8.7](https://datatracker.ietf.org/doc/html/draft-ietf-oauth-client-id-metadata-document-02#section-8.7)). Timeouts.
- `private_key_jwt` declared → client auth required with the document's keys ([Section 8.2](https://datatracker.ietf.org/doc/html/draft-ietf-oauth-client-id-metadata-document-02#section-8.2)). Metadata changes (redirects, keys) re-evaluated ([Section 8.4](https://datatracker.ietf.org/doc/html/draft-ietf-oauth-client-id-metadata-document-02#section-8.4)).
- Consent page shows the `client_id` host and the redirect host, warns on localhost-only redirects ([Section 8.5](https://datatracker.ietf.org/doc/html/draft-ietf-oauth-client-id-metadata-document-02#section-8.5), [MCP Localhost Redirect URI Risks](https://modelcontextprotocol.io/specification/2026-07-28/basic/authorization/security-considerations#localhost-redirect-uri-risks)); logos prefetched, not hot-linked ([Section 8.8](https://datatracker.ietf.org/doc/html/draft-ietf-oauth-client-id-metadata-document-02#section-8.8)).
- `client_id_metadata_document_supported: true` in metadata only when all of the above work ([Section 6](https://datatracker.ietf.org/doc/html/draft-ietf-oauth-client-id-metadata-document-02#section-6)).

**Resource-server verification (RFC 9728, AS side)** — [RFC 9728 Section 3](https://datatracker.ietf.org/doc/html/rfc9728#section-3)
- Metadata URL built by inserting `/.well-known/oauth-protected-resource` between host and path of the registered resource URI ([Section 3](https://datatracker.ietf.org/doc/html/rfc9728#section-3), [3.1](https://datatracker.ietf.org/doc/html/rfc9728#section-3.1)).
- `resource` in the response identical to the registered URI, else the document is not used ([Section 3.3](https://datatracker.ietf.org/doc/html/rfc9728#section-3.3)); `authorization_servers` contains this issuer exactly ([Section 2](https://datatracker.ietf.org/doc/html/rfc9728#section-2)). Prefix, case-folded or normalised matches — Sev 1.
- SSRF-safe fetch, timeouts, size cap, no cross-host redirects ([Section 7.7](https://datatracker.ietf.org/doc/html/rfc9728#section-7.7)); TLS ([Section 7.1](https://datatracker.ietf.org/doc/html/rfc9728#section-7.1)); cached and re-checked ([Section 7.10](https://datatracker.ietf.org/doc/html/rfc9728#section-7.10)). An unverified resource never becomes a token audience (profile rule; Othra O20).

**Device authorization grant** — [RFC 8628 Section 3](https://datatracker.ietf.org/doc/html/rfc8628#section-3)
- Not shipped → `device_authorization_endpoint` absent from metadata ([Section 4](https://datatracker.ietf.org/doc/html/rfc8628#section-4)) and the `urn:ietf:params:oauth:grant-type:device_code` grant rejected. Advertised without a working flow — Sev 1. (Othra: v1.1, not v1.)
- When shipped: request with `client_id`, client auth for confidential clients ([Section 3.1](https://datatracker.ietf.org/doc/html/rfc8628#section-3.1)); response `device_code`, `user_code`, `verification_uri`, optional `verification_uri_complete`, `expires_in`, `interval` ([Section 3.2](https://datatracker.ietf.org/doc/html/rfc8628#section-3.2)); user enters the code on a signed-in, CSRF-protected page that shows the client ([Section 3.3](https://datatracker.ietf.org/doc/html/rfc8628#section-3.3)).
- Polling ([Section 3.4](https://datatracker.ietf.org/doc/html/rfc8628#section-3.4), [3.5](https://datatracker.ietf.org/doc/html/rfc8628#section-3.5)): `authorization_pending`, `slow_down` (+5 s), `access_denied`, `expired_token`; device code single-use, bound to the client.
- User-code brute force rate-limited, short-lived, high-entropy ([Section 5.1](https://datatracker.ietf.org/doc/html/rfc8628#section-5.1), [6.1](https://datatracker.ietf.org/doc/html/rfc8628#section-6.1)); remote-phishing warning on the confirm page ([Section 5.4](https://datatracker.ietf.org/doc/html/rfc8628#section-5.4)).

**MCP authorization (AS obligations)** — [MCP Authorization, revision 2026-07-28](https://modelcontextprotocol.io/specification/2026-07-28/basic/authorization#overview)
- OAuth 2.1 for public and confidential clients; RFC 8414 or OIDC discovery; CIMD SHOULD, DCR deprecated ([Overview](https://modelcontextprotocol.io/specification/2026-07-28/basic/authorization#overview), [Client Registration](https://modelcontextprotocol.io/specification/2026-07-28/basic/authorization/client-registration#client-id-metadata-documents)).
- `resource` honoured, `iss` emitted, `code_challenge_methods_supported` published (rows above).
- Short-lived access tokens; refresh rotation for public clients ([Token Theft](https://modelcontextprotocol.io/specification/2026-07-28/basic/authorization/security-considerations#token-theft)); HTTPS endpoints, redirects HTTPS or localhost ([Communication Security](https://modelcontextprotocol.io/specification/2026-07-28/basic/authorization/security-considerations#communication-security)).
- Exact redirect match; no auto-redirect to untrusted URIs ([Open Redirection](https://modelcontextprotocol.io/specification/2026-07-28/basic/authorization/security-considerations#open-redirection)). `offline_access` only from AS metadata, never required by a resource ([Refresh Tokens](https://modelcontextprotocol.io/specification/2026-07-28/basic/authorization#refresh-tokens)).

## Client-side dimensions

- PKCE always: `S256`; verifier 43–128 chars from crypto-random source ([RFC 7636 Section 4.1](https://datatracker.ietf.org/doc/html/rfc7636#section-4.1)).
- `state`: crypto-random, verified on callback, single-use, bound to the session ([RFC 6749 Section 10.12](https://datatracker.ietf.org/doc/html/rfc6749#section-10.12)).
- `iss` in the authorization response checked against the recorded issuer by simple string comparison before the code goes to any token endpoint; reject when advertised and absent; on mismatch ignore `error` too ([RFC 9207 Section 2.4](https://datatracker.ietf.org/doc/html/rfc9207#section-2.4), [MCP Authorization Response Validation](https://modelcontextprotocol.io/specification/2026-07-28/basic/authorization#authorization-response-validation)).
- Metadata `issuer` identical to the configured issuer ([RFC 8414 Section 3.3](https://datatracker.ietf.org/doc/html/rfc8414#section-3.3), [OIDC Discovery Section 4.3](https://openid.net/specs/openid-connect-discovery-1_0.html#ProviderConfigurationValidation)).
- `redirect_uri`: registered exactly at the AS; HTTPS only except loopback ([RFC 8252 Section 7.3](https://datatracker.ietf.org/doc/html/rfc8252#section-7.3)).
- HTTPS on all AS endpoints ([RFC 6749 Section 3.1.2.1](https://datatracker.ietf.org/doc/html/rfc6749#section-3.1.2.1)).
- Never place tokens in URL fragments (2.1 removes implicit specifically to close this).
- `resource` sent on authorize and token when the AS or API expects it ([RFC 8707 Section 2](https://datatracker.ietf.org/doc/html/rfc8707#section-2)); native public clients use `token_endpoint_auth_method=none`, never a shipped secret ([RFC 8252 Section 8.5](https://datatracker.ietf.org/doc/html/rfc8252#section-8.5)).
- ID-token verification ([OIDC Core Section 3.1.3.7](https://openid.net/specs/openid-connect-core-1_0.html#IDTokenValidation)): fetch keys from `jwks_uri`, verify signature with an alg from the discovery-published list (never trust `alg` in the header alone), check `iss` exact match, `aud` contains `client_id` (reject if `aud` is an array with other entries unless `azp` names your `client_id`), `exp` / `iat` within a small clock skew (≤5 min), `nonce` matches the one sent. Cache JWKS; re-fetch on `kid` miss.
- Identify the user by `sub` from the ID token, not `email` — email can be reassigned.
- Access-token storage: HTTP-only cookie or in-memory. Never `localStorage` for confidential clients.
- Refresh rotation handled: on `400 invalid_grant`, clear both tokens and force re-auth — the AS treats this as reuse and has revoked the lineage.
- Discovery + JWKS cached with respect for `Cache-Control`; JWKS re-fetched on `kid` miss with a rate-limit to prevent unbounded fetching.
- MCP clients: discover the AS through RFC 9728 (`resource_metadata` from `WWW-Authenticate`, else the well-known URIs), validate `resource`, try the three AS metadata URLs in order, refuse without `code_challenge_methods_supported`, keep credentials per issuer ([MCP Authorization Server Discovery](https://modelcontextprotocol.io/specification/2026-07-28/basic/authorization/authorization-server-discovery#protected-resource-metadata-discovery-requirements), [Authorization Server Binding](https://modelcontextprotocol.io/specification/2026-07-28/basic/authorization/client-registration#authorization-server-binding)).

## Resource-server dimensions

- Protected-resource metadata at `/.well-known/oauth-protected-resource<path>` with `resource` equal to the URI clients use and `authorization_servers` naming the issuer(s) ([RFC 9728 Section 2](https://datatracker.ietf.org/doc/html/rfc9728#section-2), [Section 3](https://datatracker.ietf.org/doc/html/rfc9728#section-3)); MCP servers MUST serve it with at least one AS ([MCP Authorization Server Location](https://modelcontextprotocol.io/specification/2026-07-28/basic/authorization/authorization-server-discovery#authorization-server-location)).
- 401 carries `WWW-Authenticate: Bearer resource_metadata="…"` ([RFC 9728 Section 5.1](https://datatracker.ietf.org/doc/html/rfc9728#section-5.1)) and SHOULD carry `scope` ([MCP Scope Selection Strategy](https://modelcontextprotocol.io/specification/2026-07-28/basic/authorization#scope-selection-strategy)); insufficient scope → 403 `error="insufficient_scope"` with every required scope in one challenge ([RFC 6750 Section 3.1](https://datatracker.ietf.org/doc/html/rfc6750#section-3.1), [MCP Runtime Insufficient Scope Errors](https://modelcontextprotocol.io/specification/2026-07-28/basic/authorization#runtime-insufficient-scope-errors)).
- JWT access-token validation in this order ([RFC 9068 Section 4](https://datatracker.ietf.org/doc/html/rfc9068#section-4)): `typ` is `at+jwt` or `application/at+jwt`; `iss` exact match to the configured issuer; `aud` contains this resource; signature with the AS keys and a pinned alg list, never `none`; `exp` with small skew; failure → 401 `invalid_token`.
- Opaque tokens validated through introspection with `aud`/`active` checked ([RFC 7662 Section 2.2](https://datatracker.ietf.org/doc/html/rfc7662#section-2.2)).
- Tokens only from the Authorization header, never the query string; no token passthrough to upstream APIs ([MCP Token Requirements](https://modelcontextprotocol.io/specification/2026-07-28/basic/authorization#token-requirements), [Access Token Privilege Restriction](https://modelcontextprotocol.io/specification/2026-07-28/basic/authorization/security-considerations#access-token-privilege-restriction)).

## Scoring rubric

| Score | Meaning |
|---|---|
| **10** | Spec-clean; nothing to add |
| **8-9** | Mostly right; minor gaps or missing nice-to-haves |
| **5-7** | Partial coverage; meaningful spec deviation |
| **1-4** | Attempted but broken, or missing a key protection |
| **0** | Feature absent when the spec requires it |

Aggregate per section = arithmetic mean of in-scope dimensions. Overall = mean of section aggregates. Individual rows outweigh the aggregate — report both. A feature the target profile defers (e.g. device grant in Othra v1) is out of scope, scored only on "not advertised".

## Severity rubric for the recommendation plan

- **Sev 1** — spec violation or silent security regression (auth bypass, token forgery, replay, `alg=none` accepted, OIDC advertised without id_token, `password` grant present, a capability advertised in metadata without a working code path, an SSRF-able CIMD or RFC 9728 fetch, a token minted for an unverified or unrequested audience, a cross-issuer token accepted). Ship before external RPs.
- **Sev 2** — parity gap vs mainstream IdPs / SDKs (Auth0, Keycloak, Clerk): DCR, private_key_jwt, rate limiting, RP-initiated logout, scope allowlist, `prompt` handling, `iss` response parameter.
- **Sev 3** — high-assurance / FAPI-only: DPoP, PAR, JAR, mTLS, back-channel + front-channel logout.
- When an MCP surface or the target profile requires them, RFC 8707 `resource`, RFC 9728 and CIMD gaps are Sev 1, RFC 9207 gaps Sev 2.

## References

`references/rfc-index.md` — RFC + OIDC + MCP + FAPI index, with the specific section anchors this skill cites. Copy the anchor URL from there instead of constructing one.
