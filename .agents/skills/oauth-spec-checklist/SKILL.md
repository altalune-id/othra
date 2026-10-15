---
name: oauth-spec-checklist
description: OAuth 2.0 / OAuth 2.1 conformance dimensions with a scoring rubric. Covers both authorization servers (IdP-side — /authorize, /token, discovery, JWKS, /userinfo, /introspect, /revoke, end-session, id_token) and OAuth clients (RP-side — PKCE, state, redirect handling, ID-token verification, refresh rotation, token storage). Load when reviewing an OAuth implementation against RFC 6749, draft-ietf-oauth-v2-1, or OIDC Core 1.0. Companion RFC index in references/rfc-index.md — cite the section-anchor URL, not the bare RFC URL.
---

# oauth-spec-checklist

A dimension list for scoring OAuth 2.0 / 2.1 conformance. Load once per review; cite `file:line` evidence per dimension, score 0-10, give a one-line recommendation. When citing a spec, link the section anchor (e.g. `#section-4.1.2.1`), not the bare RFC URL — sections in `references/rfc-index.md` are the shortlist.

## Two questions before reading

1. **Server or client?** Server = IdP endpoints (authorize / token / discovery / JWKS / introspect / revoke / userinfo / end-session). Client = RP-side (redirect kickoff, callback handling, token storage, refresh).
2. **Target spec?** OAuth 2.0 = RFC 6749. OAuth 2.1 = draft-ietf-oauth-v2-1 — removes implicit + ROPC ([Section 7.5](https://datatracker.ietf.org/doc/html/draft-ietf-oauth-v2-1#section-7.5)), makes PKCE mandatory for every client, requires HTTPS redirects except loopback. Default to 2.1 when ambiguous.

## Server-side dimensions

**/authorize** — [RFC 6749 Section 4.1](https://datatracker.ietf.org/doc/html/rfc6749#section-4.1)
- PKCE required (2.1: every client; 2.0: public clients per RFC 7636). Accept only `code_challenge_method=S256` ([RFC 7636 Section 4.2](https://datatracker.ietf.org/doc/html/rfc7636#section-4.2)); reject `plain`. Enforce verifier 43–128 chars ([RFC 7636 Section 4.1](https://datatracker.ietf.org/doc/html/rfc7636#section-4.1)).
- `response_type=code` only. 2.1 forbids `token` and hybrid.
- `redirect_uri`: exact string match against a registered value; HTTPS-only except loopback (`127.0.0.1`, `::1`) per [RFC 8252 Section 7.3](https://datatracker.ietf.org/doc/html/rfc8252#section-7.3). No wildcards, no fragments.
- `state` round-tripped on success and error redirects ([RFC 6749 Section 4.1.2](https://datatracker.ietf.org/doc/html/rfc6749#section-4.1.2), [Section 10.12](https://datatracker.ietf.org/doc/html/rfc6749#section-10.12)).
- Errors returned via redirect with `error` + `error_description` + `state` when a valid `redirect_uri` is bound ([RFC 6749 Section 4.1.2.1](https://datatracker.ietf.org/doc/html/rfc6749#section-4.1.2.1)); rendered as JSON only when no redirect is trusted.
- Error codes: `invalid_request` / `unauthorized_client` / `access_denied` / `unsupported_response_type` / `invalid_scope` / `server_error` / `temporarily_unavailable`.
- Authorization code: single-use, short-lived (≤10 min per Section 4.1.2; ≤60 s recommended), bound to `client_id` + `redirect_uri` + `code_challenge` + user. Store the SHA-256 hash, not the plaintext.
- OIDC extras ([OIDC Core Section 3.1.2.1](https://openid.net/specs/openid-connect-core-1_0.html#AuthRequest)): `nonce` (required when `id_token` is issued), `prompt`, `max_age`, `login_hint`, `ui_locales`, `acr_values`.

**/token** — [RFC 6749 Section 4.1.3](https://datatracker.ietf.org/doc/html/rfc6749#section-4.1.3)
- Grant types: `authorization_code`, `refresh_token`, `client_credentials`. **`password` (ROPC) forbidden in 2.1** ([Section 7.5](https://datatracker.ietf.org/doc/html/draft-ietf-oauth-v2-1#section-7.5)); flag any live code path.
- PKCE verifier check with constant-time compare ([RFC 7636 Section 4.6](https://datatracker.ietf.org/doc/html/rfc7636#section-4.6)).
- `redirect_uri` bound to the code — strict equality on exchange.
- Auth-code replay revokes previously issued tokens (SHOULD per [OAuth 2.1 Section 4.1.3](https://datatracker.ietf.org/doc/html/draft-ietf-oauth-v2-1#section-4.1.3)). Requires `session_id` on the code row.
- Client auth ([RFC 6749 Section 2.3](https://datatracker.ietf.org/doc/html/rfc6749#section-2.3), [RFC 7523 Section 2.2](https://datatracker.ietf.org/doc/html/rfc7523#section-2.2)): `client_secret_basic` / `_post` / `_jwt`, `private_key_jwt`, `tls_client_auth`, `none`. Cross-check Basic `client_id` vs form ([Section 2.3.1](https://datatracker.ietf.org/doc/html/rfc6749#section-2.3.1)). Constant-time secret compare.
- Access token: JWT ([RFC 9068 Section 2.2](https://datatracker.ietf.org/doc/html/rfc9068#section-2.2) `at+jwt`) OR opaque. If JWT: `iss`, `sub`, `aud`, `exp`, `iat`, `jti`, `client_id`, `scope`; `alg=none` MUST be refused; verifier alg-list pinned.
- Refresh rotation ([RFC 6749 Section 6](https://datatracker.ietf.org/doc/html/rfc6749#section-6), [OAuth 2.1 Section 4.3](https://datatracker.ietf.org/doc/html/draft-ietf-oauth-v2-1#section-4.3), [Security BCP Section 4.8](https://datatracker.ietf.org/doc/html/draft-ietf-oauth-security-topics#section-4.8)): new token on every use, old invalidated; **reuse of an already-consumed token revokes the whole lineage / family**, not just the presented token. Track `session_id` + generation counter.
- Response: `Cache-Control: no-store`, `Pragma: no-cache`, `token_type=Bearer`, `expires_in` ([Section 5.1](https://datatracker.ietf.org/doc/html/rfc6749#section-5.1)).
- Errors: `invalid_request` / `invalid_client` (401 + [`WWW-Authenticate`](https://datatracker.ietf.org/doc/html/rfc6750#section-3)) / `invalid_grant` / `unauthorized_client` / `unsupported_grant_type` / `invalid_scope` ([Section 5.2](https://datatracker.ietf.org/doc/html/rfc6749#section-5.2)).

**Discovery + JWKS** — [RFC 8414](https://datatracker.ietf.org/doc/html/rfc8414#section-2) + [OIDC Discovery Section 4](https://openid.net/specs/openid-connect-discovery-1_0.html#ProviderConfig)
- Publish `/.well-known/oauth-authorization-server` AND `/.well-known/openid-configuration` when OIDC is advertised.
- `issuer` stable; matches ID-token `iss`; matches JWKS `iss`. Don't advertise `openid` in `scopes_supported` or list `id_token_signing_alg_values_supported` without a live ID-token code path — Sev 1.
- `jwks_uri` publishes public-only keys with `kid`, `alg`, `use=sig`. `alg=none` MUST NOT appear. Rotation window overlaps (retired `kid` verifiable during publish window; typical 30 min to 24 h).
- CORS: public / permissive on discovery + JWKS. Cache-Control: `public, max-age=…` acceptable; `ETag` a nice-to-have.

**OIDC extras** — [OIDC Core Section 2](https://openid.net/specs/openid-connect-core-1_0.html#IDToken)
- `id_token`: signed JWT with `iss`, `sub`, `aud`, `exp`, `iat`, `nonce` echoed from `/authorize`, `at_hash` when issued with an access token, `azp` when audience ≠ client_id, `auth_time` when `max_age` was requested.
- [`/userinfo`](https://openid.net/specs/openid-connect-core-1_0.html#UserInfo): Bearer-authenticated; returns stable `sub`; scope-gated `profile` / `email` / `address` / `phone` claims. Support GET + POST + CORS.
- [`end_session_endpoint`](https://openid.net/specs/openid-connect-rpinitiated-1_0.html#RPLogout): `id_token_hint`, `post_logout_redirect_uri` allow-listed per client, `state`.

**Introspect ([RFC 7662](https://datatracker.ietf.org/doc/html/rfc7662#section-2.2)) + Revoke ([RFC 7009](https://datatracker.ietf.org/doc/html/rfc7009#section-2.2))**
- Introspect: client-auth required; foreign-client tokens → `{active:false}` (no cross-client probing); `Cache-Control: no-store`. Response: `active`, `scope`, `client_id`, `exp`, `iat`, `sub`, `aud`, `iss`, `token_type`, `jti`.
- Revoke: idempotent 200 (even for unknown tokens per Section 2.2); accepts access + refresh with `token_type_hint`; access-token revoke cascades to session; client-auth required for confidential clients.

## Client-side dimensions

- PKCE always: `S256`; verifier 43–128 chars from crypto-random source ([RFC 7636 Section 4.1](https://datatracker.ietf.org/doc/html/rfc7636#section-4.1)).
- `state`: crypto-random, verified on callback, single-use, bound to the session ([RFC 6749 Section 10.12](https://datatracker.ietf.org/doc/html/rfc6749#section-10.12)).
- `redirect_uri`: registered exactly at the AS; HTTPS only except loopback ([RFC 8252 Section 7.3](https://datatracker.ietf.org/doc/html/rfc8252#section-7.3)).
- HTTPS on all AS endpoints ([RFC 6749 Section 3.1.2.1](https://datatracker.ietf.org/doc/html/rfc6749#section-3.1.2.1)).
- Never place tokens in URL fragments (2.1 removes implicit specifically to close this).
- ID-token verification ([OIDC Core Section 3.1.3.7](https://openid.net/specs/openid-connect-core-1_0.html#IDTokenValidation)): fetch keys from `jwks_uri`, verify signature with an alg from the discovery-published list (never trust `alg` in the header alone), check `iss` exact match, `aud` contains `client_id` (reject if `aud` is an array with other entries unless `azp` names your `client_id`), `exp` / `iat` within a small clock skew (≤5 min), `nonce` matches the one sent. Cache JWKS; re-fetch on `kid` miss.
- Identify the user by `sub` from the ID token, not `email` — email can be reassigned.
- Access-token storage: HTTP-only cookie or in-memory. Never `localStorage` for confidential clients.
- Refresh rotation handled: on `400 invalid_grant`, clear both tokens and force re-auth — the AS treats this as reuse and has revoked the lineage.
- Discovery + JWKS cached with respect for `Cache-Control`; JWKS re-fetched on `kid` miss with a rate-limit to prevent unbounded fetching.

## Scoring rubric

| Score | Meaning |
|---|---|
| **10** | Spec-clean; nothing to add |
| **8-9** | Mostly right; minor gaps or missing nice-to-haves |
| **5-7** | Partial coverage; meaningful spec deviation |
| **1-4** | Attempted but broken, or missing a key protection |
| **0** | Feature absent when the spec requires it |

Aggregate per section = arithmetic mean of in-scope dimensions. Overall = mean of section aggregates. Individual rows outweigh the aggregate — report both.

## Severity rubric for the recommendation plan

- **Sev 1** — spec violation or silent security regression (auth bypass, token forgery, replay, `alg=none` accepted, OIDC advertised without id_token, `password` grant present). Ship before external RPs.
- **Sev 2** — parity gap vs mainstream IdPs / SDKs (Auth0, Keycloak, Clerk): DCR, private_key_jwt, rate limiting, audit log, RP-initiated logout, scope allowlist, `prompt` handling.
- **Sev 3** — high-assurance / FAPI-only: DPoP, PAR, JAR, mTLS, RFC 8707 `resource`, back-channel + front-channel logout.

## References

`references/rfc-index.md` — RFC + OIDC + FAPI index, with the specific section anchors this skill cites. Copy the anchor URL from there instead of constructing one.
