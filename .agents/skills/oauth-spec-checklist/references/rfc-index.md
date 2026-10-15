# RFC index — OAuth 2.0 / 2.1 / OIDC / MCP

One-line description per spec, with the specific section anchors this skill cites. Prefer the sub-anchor link when quoting. Drafts without a version suffix resolve to the latest revision; anchors below were checked against draft-ietf-oauth-v2-1-16 and draft-ietf-oauth-client-id-metadata-document-02 (pinned, because its sections moved between revisions).

## Core

- [RFC 6749](https://datatracker.ietf.org/doc/html/rfc6749) — The OAuth 2.0 Authorization Framework (baseline)
  - [Section 2.1 client types](https://datatracker.ietf.org/doc/html/rfc6749#section-2.1) · [Section 2.3 client authentication](https://datatracker.ietf.org/doc/html/rfc6749#section-2.3) · [Section 2.3.1 Basic](https://datatracker.ietf.org/doc/html/rfc6749#section-2.3.1)
  - [Section 3.1 unrecognized parameters ignored, no repeats](https://datatracker.ietf.org/doc/html/rfc6749#section-3.1) · [Section 3.1.2.1 authz endpoint requires TLS](https://datatracker.ietf.org/doc/html/rfc6749#section-3.1.2.1)
  - [Section 4.1 auth code grant](https://datatracker.ietf.org/doc/html/rfc6749#section-4.1) · [Section 4.1.2 response](https://datatracker.ietf.org/doc/html/rfc6749#section-4.1.2) · [Section 4.1.2.1 error response](https://datatracker.ietf.org/doc/html/rfc6749#section-4.1.2.1)
  - [Section 4.1.3 access token request](https://datatracker.ietf.org/doc/html/rfc6749#section-4.1.3) · [Section 4.3 password grant (removed in 2.1)](https://datatracker.ietf.org/doc/html/rfc6749#section-4.3) · [Section 4.4 client credentials](https://datatracker.ietf.org/doc/html/rfc6749#section-4.4) · [Section 4.4.3 no refresh token](https://datatracker.ietf.org/doc/html/rfc6749#section-4.4.3)
  - [Section 5.1 success response](https://datatracker.ietf.org/doc/html/rfc6749#section-5.1) · [Section 5.2 error response](https://datatracker.ietf.org/doc/html/rfc6749#section-5.2)
  - [Section 6 refresh](https://datatracker.ietf.org/doc/html/rfc6749#section-6) · [Section 10.10 credential guessing](https://datatracker.ietf.org/doc/html/rfc6749#section-10.10) · [Section 10.12 CSRF / state, AS endpoint CSRF](https://datatracker.ietf.org/doc/html/rfc6749#section-10.12)
- [draft-ietf-oauth-v2-1](https://datatracker.ietf.org/doc/html/draft-ietf-oauth-v2-1) — OAuth 2.1 (omits implicit + ROPC, PKCE mandatory, HTTPS-strict redirect)
  - [Section 2.1 client types](https://datatracker.ietf.org/doc/html/draft-ietf-oauth-v2-1#section-2.1) · [Section 2.3.1 redirect URI registration](https://datatracker.ietf.org/doc/html/draft-ietf-oauth-v2-1#section-2.3.1) · [Section 3.2 token endpoint](https://datatracker.ietf.org/doc/html/draft-ietf-oauth-v2-1#section-3.2) · [Section 3.2.1 confidential clients authenticate](https://datatracker.ietf.org/doc/html/draft-ietf-oauth-v2-1#section-3.2.1)
  - [Section 4.1.3 auth code redemption + replay revocation SHOULD](https://datatracker.ietf.org/doc/html/draft-ietf-oauth-v2-1#section-4.1.3) · [Section 7.5.3 reuse of authorization codes](https://datatracker.ietf.org/doc/html/draft-ietf-oauth-v2-1#section-7.5.3)
  - [Section 4.2 client credentials](https://datatracker.ietf.org/doc/html/draft-ietf-oauth-v2-1#section-4.2) · [Section 4.3 refresh + reuse detection](https://datatracker.ietf.org/doc/html/draft-ietf-oauth-v2-1#section-4.3)
  - [Section 7.15 mix-up mitigation](https://datatracker.ietf.org/doc/html/draft-ietf-oauth-v2-1#section-7.15)
  - [Section 10 differences from OAuth 2.0 (implicit + ROPC omitted)](https://datatracker.ietf.org/doc/html/draft-ietf-oauth-v2-1#section-10)
- [RFC 6750](https://datatracker.ietf.org/doc/html/rfc6750) — Bearer Token Usage
  - [Section 2 authenticated requests](https://datatracker.ietf.org/doc/html/rfc6750#section-2) · [Section 3 WWW-Authenticate](https://datatracker.ietf.org/doc/html/rfc6750#section-3) · [Section 3.1 error codes](https://datatracker.ietf.org/doc/html/rfc6750#section-3.1)

## Client + native

- [RFC 7636](https://datatracker.ietf.org/doc/html/rfc7636) — PKCE
  - [Section 4.1 code_verifier 43–128 chars](https://datatracker.ietf.org/doc/html/rfc7636#section-4.1) · [Section 4.2 code_challenge](https://datatracker.ietf.org/doc/html/rfc7636#section-4.2) · [Section 4.6 server verification](https://datatracker.ietf.org/doc/html/rfc7636#section-4.6)
- [RFC 8252](https://datatracker.ietf.org/doc/html/rfc8252) — OAuth 2.0 for Native Apps
  - [Section 7.3 loopback redirect exception, any port](https://datatracker.ietf.org/doc/html/rfc8252#section-7.3) · [Section 8.3 loopback considerations (`localhost` NOT RECOMMENDED)](https://datatracker.ietf.org/doc/html/rfc8252#section-8.3) · [Section 8.1 protecting the code (S256)](https://datatracker.ietf.org/doc/html/rfc8252#section-8.1) · [Section 8.5 no shipped client secrets](https://datatracker.ietf.org/doc/html/rfc8252#section-8.5)
- [RFC 8628](https://datatracker.ietf.org/doc/html/rfc8628) — Device Authorization Grant (Othra: v1.1; do not advertise `device_authorization_endpoint` until it ships)
  - [Section 3.1 device authorization request](https://datatracker.ietf.org/doc/html/rfc8628#section-3.1) · [Section 3.2 device authorization response](https://datatracker.ietf.org/doc/html/rfc8628#section-3.2) · [Section 3.3 user interaction](https://datatracker.ietf.org/doc/html/rfc8628#section-3.3)
  - [Section 3.4 device access token request](https://datatracker.ietf.org/doc/html/rfc8628#section-3.4) · [Section 3.5 device access token response (`authorization_pending`, `slow_down`)](https://datatracker.ietf.org/doc/html/rfc8628#section-3.5)
  - [Section 4 discovery metadata](https://datatracker.ietf.org/doc/html/rfc8628#section-4) · [Section 5.1 user code brute forcing](https://datatracker.ietf.org/doc/html/rfc8628#section-5.1) · [Section 5.4 remote phishing](https://datatracker.ietf.org/doc/html/rfc8628#section-5.4) · [Section 6.1 user code recommendations](https://datatracker.ietf.org/doc/html/rfc8628#section-6.1)

## Client authentication + identification

- [RFC 7523](https://datatracker.ietf.org/doc/html/rfc7523) — JWT profile (`client_secret_jwt`, `private_key_jwt`)
  - [Section 2.2 client authentication assertion](https://datatracker.ietf.org/doc/html/rfc7523#section-2.2) · [Section 3 JWT format](https://datatracker.ietf.org/doc/html/rfc7523#section-3)
- [RFC 8705](https://datatracker.ietf.org/doc/html/rfc8705) — mTLS client auth + certificate-bound tokens
  - [Section 2 mTLS client auth](https://datatracker.ietf.org/doc/html/rfc8705#section-2) · [Section 3 sender-constrained tokens](https://datatracker.ietf.org/doc/html/rfc8705#section-3)
- [draft-ietf-oauth-client-id-metadata-document-02](https://datatracker.ietf.org/doc/html/draft-ietf-oauth-client-id-metadata-document-02) — OAuth Client ID Metadata Document (CIMD)
  - [Section 3 client identifier URL](https://datatracker.ietf.org/doc/html/draft-ietf-oauth-client-id-metadata-document-02#section-3) · [Section 4 metadata document](https://datatracker.ietf.org/doc/html/draft-ietf-oauth-client-id-metadata-document-02#section-4) · [Section 4.1 no shared secrets](https://datatracker.ietf.org/doc/html/draft-ietf-oauth-client-id-metadata-document-02#section-4.1) · [Section 4.2 redirect URL registration](https://datatracker.ietf.org/doc/html/draft-ietf-oauth-client-id-metadata-document-02#section-4.2)
  - [Section 5 client information discovery (no redirects)](https://datatracker.ietf.org/doc/html/draft-ietf-oauth-client-id-metadata-document-02#section-5) · [Section 5.1 discovery errors](https://datatracker.ietf.org/doc/html/draft-ietf-oauth-client-id-metadata-document-02#section-5.1) · [Section 5.2 caching](https://datatracker.ietf.org/doc/html/draft-ietf-oauth-client-id-metadata-document-02#section-5.2)
  - [Section 6 AS metadata `client_id_metadata_document_supported`](https://datatracker.ietf.org/doc/html/draft-ietf-oauth-client-id-metadata-document-02#section-6) · [Section 7.1 pre-registered and URL clients](https://datatracker.ietf.org/doc/html/draft-ietf-oauth-client-id-metadata-document-02#section-7.1)
  - [Section 8.2 client authentication](https://datatracker.ietf.org/doc/html/draft-ietf-oauth-client-id-metadata-document-02#section-8.2) · [Section 8.4 metadata changes](https://datatracker.ietf.org/doc/html/draft-ietf-oauth-client-id-metadata-document-02#section-8.4) · [Section 8.5 phishing](https://datatracker.ietf.org/doc/html/draft-ietf-oauth-client-id-metadata-document-02#section-8.5)
  - [Section 8.6 SSRF](https://datatracker.ietf.org/doc/html/draft-ietf-oauth-client-id-metadata-document-02#section-8.6) · [Section 8.7 max response size (5 KB)](https://datatracker.ietf.org/doc/html/draft-ietf-oauth-client-id-metadata-document-02#section-8.7) · [Section 8.8 logos](https://datatracker.ietf.org/doc/html/draft-ietf-oauth-client-id-metadata-document-02#section-8.8) · [Section 8.9 domain trust](https://datatracker.ietf.org/doc/html/draft-ietf-oauth-client-id-metadata-document-02#section-8.9)

## Server endpoints

- [RFC 7662](https://datatracker.ietf.org/doc/html/rfc7662) — Token Introspection
  - [Section 2.1 request](https://datatracker.ietf.org/doc/html/rfc7662#section-2.1) · [Section 2.2 response fields](https://datatracker.ietf.org/doc/html/rfc7662#section-2.2)
- [RFC 7009](https://datatracker.ietf.org/doc/html/rfc7009) — Token Revocation
  - [Section 2.1 request (refresh revoke invalidates the grant's access tokens)](https://datatracker.ietf.org/doc/html/rfc7009#section-2.1) · [Section 2.2 200 idempotent](https://datatracker.ietf.org/doc/html/rfc7009#section-2.2)
- [RFC 8414](https://datatracker.ietf.org/doc/html/rfc8414) — Authorization Server Metadata
  - [Section 2 required metadata](https://datatracker.ietf.org/doc/html/rfc8414#section-2) · [Section 3 obtaining metadata](https://datatracker.ietf.org/doc/html/rfc8414#section-3) · [Section 3.1 well-known path insertion](https://datatracker.ietf.org/doc/html/rfc8414#section-3.1)
  - [Section 3.3 metadata validation (`issuer` identical)](https://datatracker.ietf.org/doc/html/rfc8414#section-3.3) · [Section 5 compatibility with OIDC discovery](https://datatracker.ietf.org/doc/html/rfc8414#section-5)
- [RFC 9207](https://datatracker.ietf.org/doc/html/rfc9207) — Authorization Server Issuer Identification (`iss` response parameter)
  - [Section 2 `iss` response parameter](https://datatracker.ietf.org/doc/html/rfc9207#section-2) · [Section 2.3 providing the issuer](https://datatracker.ietf.org/doc/html/rfc9207#section-2.3) · [Section 2.4 validating the issuer (client)](https://datatracker.ietf.org/doc/html/rfc9207#section-2.4)
  - [Section 3 `authorization_response_iss_parameter_supported`](https://datatracker.ietf.org/doc/html/rfc9207#section-3)
- [RFC 9068](https://datatracker.ietf.org/doc/html/rfc9068) — JWT Profile for Access Tokens
  - [Section 2.1 header (`typ=at+jwt`, no `none`, RS256)](https://datatracker.ietf.org/doc/html/rfc9068#section-2.1) · [Section 2.2 required claims](https://datatracker.ietf.org/doc/html/rfc9068#section-2.2) · [Section 2.2.3 authorization claims (`scope`)](https://datatracker.ietf.org/doc/html/rfc9068#section-2.2.3)
  - [Section 3 requesting (`aud` from `resource`)](https://datatracker.ietf.org/doc/html/rfc9068#section-3) · [Section 4 validating](https://datatracker.ietf.org/doc/html/rfc9068#section-4) · [Section 5 security](https://datatracker.ietf.org/doc/html/rfc9068#section-5) · [Section 6 privacy](https://datatracker.ietf.org/doc/html/rfc9068#section-6)

- [RFC 7517](https://datatracker.ietf.org/doc/html/rfc7517) — JSON Web Key
  - [Section 4.5 `kid`](https://datatracker.ietf.org/doc/html/rfc7517#section-4.5)
- [RFC 6797](https://datatracker.ietf.org/doc/html/rfc6797) — HTTP Strict Transport Security
  - [Section 7.1 HSTS response header](https://datatracker.ietf.org/doc/html/rfc6797#section-7.1)

## Resource servers

- [RFC 8707](https://datatracker.ietf.org/doc/html/rfc8707) — Resource Indicators (`resource` param)
  - [Section 2 resource parameter (absolute URI, no fragment, `invalid_target`)](https://datatracker.ietf.org/doc/html/rfc8707#section-2) · [Section 2.1 authorization request](https://datatracker.ietf.org/doc/html/rfc8707#section-2.1) · [Section 2.2 access token request](https://datatracker.ietf.org/doc/html/rfc8707#section-2.2)
- [RFC 9728](https://datatracker.ietf.org/doc/html/rfc9728) — OAuth 2.0 Protected Resource Metadata (Othra: resource-server ownership verification, decision O20)
  - [Section 2 metadata (`resource`, `authorization_servers`)](https://datatracker.ietf.org/doc/html/rfc9728#section-2) · [Section 3 well-known URL construction](https://datatracker.ietf.org/doc/html/rfc9728#section-3) · [Section 3.1 request](https://datatracker.ietf.org/doc/html/rfc9728#section-3.1)
  - [Section 3.3 validation (`resource` identical, else unused)](https://datatracker.ietf.org/doc/html/rfc9728#section-3.3) · [Section 5 WWW-Authenticate](https://datatracker.ietf.org/doc/html/rfc9728#section-5) · [Section 5.1 `resource_metadata` parameter](https://datatracker.ietf.org/doc/html/rfc9728#section-5.1)
  - [Section 7.3 impersonation](https://datatracker.ietf.org/doc/html/rfc9728#section-7.3) · [Section 7.6 authorization servers](https://datatracker.ietf.org/doc/html/rfc9728#section-7.6) · [Section 7.7 SSRF](https://datatracker.ietf.org/doc/html/rfc9728#section-7.7) · [Section 7.10 caching](https://datatracker.ietf.org/doc/html/rfc9728#section-7.10)

## Registration + advanced

- [RFC 7591](https://datatracker.ietf.org/doc/html/rfc7591) — Dynamic Client Registration (not in Othra v1; deprecated by MCP)
  - [Section 3.1 registration endpoint](https://datatracker.ietf.org/doc/html/rfc7591#section-3.1)
- [RFC 7592](https://datatracker.ietf.org/doc/html/rfc7592) — DCR Management
- [RFC 9101](https://datatracker.ietf.org/doc/html/rfc9101) — JAR (Request Object)
- [RFC 9126](https://datatracker.ietf.org/doc/html/rfc9126) — PAR (Pushed Authorization Requests)
- [RFC 9449](https://datatracker.ietf.org/doc/html/rfc9449) — DPoP (Demonstrating Proof-of-Possession)
  - [Section 4 DPoP proof JWT](https://datatracker.ietf.org/doc/html/rfc9449#section-4)

## Security BCPs

- [RFC 6819](https://datatracker.ietf.org/doc/html/rfc6819) — OAuth 2.0 Threat Model + Security Considerations
- [RFC 9700](https://datatracker.ietf.org/doc/html/rfc9700) — Best Current Practice for OAuth 2.0 Security (published draft-ietf-oauth-security-topics)
  - [Section 2 recommendations](https://datatracker.ietf.org/doc/html/rfc9700#section-2) · [Section 2.1 redirect-based flows](https://datatracker.ietf.org/doc/html/rfc9700#section-2.1) · [Section 2.4 ROPC MUST NOT](https://datatracker.ietf.org/doc/html/rfc9700#section-2.4)
  - [Section 4.1.3 exact redirect matching](https://datatracker.ietf.org/doc/html/rfc9700#section-4.1.3) · [Section 4.2.4 Referer leakage countermeasures](https://datatracker.ietf.org/doc/html/rfc9700#section-4.2.4) · [Section 4.4.2.1 mix-up defence via `iss`](https://datatracker.ietf.org/doc/html/rfc9700#section-4.4.2.1) · [Section 4.8 PKCE downgrade](https://datatracker.ietf.org/doc/html/rfc9700#section-4.8)
  - [Section 4.10.2 audience-restricted tokens](https://datatracker.ietf.org/doc/html/rfc9700#section-4.10.2) · [Section 4.11.2 AS as open redirector](https://datatracker.ietf.org/doc/html/rfc9700#section-4.11.2) · [Section 4.14 refresh-token protection](https://datatracker.ietf.org/doc/html/rfc9700#section-4.14) · [Section 4.16 clickjacking](https://datatracker.ietf.org/doc/html/rfc9700#section-4.16)

## OpenID Connect

- [OIDC Core 1.0](https://openid.net/specs/openid-connect-core-1_0.html) — Authentication
  - [Section 2 ID Token](https://openid.net/specs/openid-connect-core-1_0.html#IDToken) · [Section 3.1.3.7 ID Token validation](https://openid.net/specs/openid-connect-core-1_0.html#IDTokenValidation)
  - [Section 3.1.2.1 auth request](https://openid.net/specs/openid-connect-core-1_0.html#AuthRequest) · [Section 3.1.2.4 consent](https://openid.net/specs/openid-connect-core-1_0.html#Consent) · [Section 3.1.2.6 auth error (`login_required`…)](https://openid.net/specs/openid-connect-core-1_0.html#AuthError)
  - [Section 5.3 UserInfo](https://openid.net/specs/openid-connect-core-1_0.html#UserInfo) · [Section 5.4 scope claims](https://openid.net/specs/openid-connect-core-1_0.html#ScopeClaims) · [Section 11 offline access](https://openid.net/specs/openid-connect-core-1_0.html#OfflineAccess)
- [OIDC Discovery 1.0](https://openid.net/specs/openid-connect-discovery-1_0.html)
  - [Section 3 provider metadata (`RS256` MUST be offered)](https://openid.net/specs/openid-connect-discovery-1_0.html#ProviderMetadata) · [Section 4 obtaining OP config](https://openid.net/specs/openid-connect-discovery-1_0.html#ProviderConfig) · [Section 4.3 config validation](https://openid.net/specs/openid-connect-discovery-1_0.html#ProviderConfigurationValidation)
- [OIDC RP-Initiated Logout 1.0](https://openid.net/specs/openid-connect-rpinitiated-1_0.html)
  - [Section 2 logout endpoint](https://openid.net/specs/openid-connect-rpinitiated-1_0.html#RPLogout)
- [OIDC Session Management 1.0](https://openid.net/specs/openid-connect-session-1_0.html)
- [OIDC Back-Channel Logout 1.0](https://openid.net/specs/openid-connect-backchannel-1_0.html)
- [OIDC Front-Channel Logout 1.0](https://openid.net/specs/openid-connect-frontchannel-1_0.html)
  - [Section 3 OP logout (`sid` claim)](https://openid.net/specs/openid-connect-frontchannel-1_0.html#OPLogout)

## MCP Authorization (revision 2026-07-28, current)

Base: `https://modelcontextprotocol.io/specification/2026-07-28/basic/authorization`. The revision splits the spec into a main page and three sub-pages; `/specification/latest/…` redirects to the current revision.

- [Authorization](https://modelcontextprotocol.io/specification/2026-07-28/basic/authorization)
  - [Overview (OAuth 2.1, CIMD SHOULD, DCR deprecated, RFC 9728 MUST)](https://modelcontextprotocol.io/specification/2026-07-28/basic/authorization#overview) · [Scope selection strategy](https://modelcontextprotocol.io/specification/2026-07-28/basic/authorization#scope-selection-strategy) · [Authorization flow steps](https://modelcontextprotocol.io/specification/2026-07-28/basic/authorization#authorization-flow-steps)
  - [Authorization response validation (RFC 9207 `iss`)](https://modelcontextprotocol.io/specification/2026-07-28/basic/authorization#authorization-response-validation) · [Resource parameter implementation](https://modelcontextprotocol.io/specification/2026-07-28/basic/authorization#resource-parameter-implementation) · [Canonical server URI](https://modelcontextprotocol.io/specification/2026-07-28/basic/authorization#canonical-server-uri)
  - [Token requirements](https://modelcontextprotocol.io/specification/2026-07-28/basic/authorization#token-requirements) · [Token handling](https://modelcontextprotocol.io/specification/2026-07-28/basic/authorization#token-handling) · [Refresh tokens](https://modelcontextprotocol.io/specification/2026-07-28/basic/authorization#refresh-tokens)
  - [Error handling](https://modelcontextprotocol.io/specification/2026-07-28/basic/authorization#error-handling) · [Runtime insufficient scope errors](https://modelcontextprotocol.io/specification/2026-07-28/basic/authorization#runtime-insufficient-scope-errors) · [Step-up authorization flow](https://modelcontextprotocol.io/specification/2026-07-28/basic/authorization#step-up-authorization-flow)
- [Authorization Server Discovery](https://modelcontextprotocol.io/specification/2026-07-28/basic/authorization/authorization-server-discovery)
  - [Authorization server location](https://modelcontextprotocol.io/specification/2026-07-28/basic/authorization/authorization-server-discovery#authorization-server-location) · [Protected resource metadata discovery requirements](https://modelcontextprotocol.io/specification/2026-07-28/basic/authorization/authorization-server-discovery#protected-resource-metadata-discovery-requirements) · [AS metadata discovery (three URLs, in order)](https://modelcontextprotocol.io/specification/2026-07-28/basic/authorization/authorization-server-discovery#authorization-server-metadata-discovery)
- [Client Registration](https://modelcontextprotocol.io/specification/2026-07-28/basic/authorization/client-registration)
  - [Client ID Metadata Documents](https://modelcontextprotocol.io/specification/2026-07-28/basic/authorization/client-registration#client-id-metadata-documents) · [Implementation requirements](https://modelcontextprotocol.io/specification/2026-07-28/basic/authorization/client-registration#implementation-requirements) · [Advertising CIMD support](https://modelcontextprotocol.io/specification/2026-07-28/basic/authorization/client-registration#advertising-cimd-support)
  - [Pre-registration](https://modelcontextprotocol.io/specification/2026-07-28/basic/authorization/client-registration#pre-registration) · [Dynamic client registration (deprecated)](https://modelcontextprotocol.io/specification/2026-07-28/basic/authorization/client-registration#dynamic-client-registration) · [Authorization server binding](https://modelcontextprotocol.io/specification/2026-07-28/basic/authorization/client-registration#authorization-server-binding)
- [Security Considerations](https://modelcontextprotocol.io/specification/2026-07-28/basic/authorization/security-considerations)
  - [Token audience binding](https://modelcontextprotocol.io/specification/2026-07-28/basic/authorization/security-considerations#token-audience-binding-and-validation) · [Token theft](https://modelcontextprotocol.io/specification/2026-07-28/basic/authorization/security-considerations#token-theft) · [Communication security](https://modelcontextprotocol.io/specification/2026-07-28/basic/authorization/security-considerations#communication-security)
  - [Authorization code protection (`code_challenge_methods_supported`)](https://modelcontextprotocol.io/specification/2026-07-28/basic/authorization/security-considerations#authorization-code-protection) · [Mix-up attacks](https://modelcontextprotocol.io/specification/2026-07-28/basic/authorization/security-considerations#mix-up-attacks) · [Open redirection](https://modelcontextprotocol.io/specification/2026-07-28/basic/authorization/security-considerations#open-redirection)
  - [CIMD security](https://modelcontextprotocol.io/specification/2026-07-28/basic/authorization/security-considerations#client-id-metadata-document-security) · [Localhost redirect URI risks](https://modelcontextprotocol.io/specification/2026-07-28/basic/authorization/security-considerations#localhost-redirect-uri-risks) · [Access token privilege restriction](https://modelcontextprotocol.io/specification/2026-07-28/basic/authorization/security-considerations#access-token-privilege-restriction)

## FAPI (high-assurance)

- [FAPI 2.0 Security Profile](https://openid.net/specs/fapi-2_0-security-profile.html)
- [FAPI 2.0 Message Signing](https://openid.net/specs/fapi-2_0-message-signing.html)
