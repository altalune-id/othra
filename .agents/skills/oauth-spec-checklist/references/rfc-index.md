# RFC index — OAuth 2.0 / 2.1 / OIDC

One-line description per spec, with the specific section anchors this skill cites. Prefer the sub-anchor link when quoting.

## Core

- [RFC 6749](https://datatracker.ietf.org/doc/html/rfc6749) — The OAuth 2.0 Authorization Framework (baseline)
  - [Section 2.3 client authentication](https://datatracker.ietf.org/doc/html/rfc6749#section-2.3) · [Section 2.3.1 Basic](https://datatracker.ietf.org/doc/html/rfc6749#section-2.3.1)
  - [Section 3.1.2.1 authz endpoint requires TLS](https://datatracker.ietf.org/doc/html/rfc6749#section-3.1.2.1)
  - [Section 4.1 auth code grant](https://datatracker.ietf.org/doc/html/rfc6749#section-4.1) · [Section 4.1.2 response](https://datatracker.ietf.org/doc/html/rfc6749#section-4.1.2) · [Section 4.1.2.1 error response](https://datatracker.ietf.org/doc/html/rfc6749#section-4.1.2.1)
  - [Section 4.1.3 access token request](https://datatracker.ietf.org/doc/html/rfc6749#section-4.1.3) · [Section 4.3 password grant (removed in 2.1)](https://datatracker.ietf.org/doc/html/rfc6749#section-4.3) · [Section 4.4 client credentials](https://datatracker.ietf.org/doc/html/rfc6749#section-4.4)
  - [Section 5.1 success response](https://datatracker.ietf.org/doc/html/rfc6749#section-5.1) · [Section 5.2 error response](https://datatracker.ietf.org/doc/html/rfc6749#section-5.2)
  - [Section 6 refresh](https://datatracker.ietf.org/doc/html/rfc6749#section-6) · [Section 10.12 CSRF / state](https://datatracker.ietf.org/doc/html/rfc6749#section-10.12)
- [draft-ietf-oauth-v2-1](https://datatracker.ietf.org/doc/html/draft-ietf-oauth-v2-1) — OAuth 2.1 (removes implicit + ROPC, PKCE mandatory, HTTPS-strict redirect)
  - [Section 4.1.3 auth code redemption + replay revocation SHOULD](https://datatracker.ietf.org/doc/html/draft-ietf-oauth-v2-1#section-4.1.3)
  - [Section 4.3 refresh + reuse detection](https://datatracker.ietf.org/doc/html/draft-ietf-oauth-v2-1#section-4.3)
  - [Section 7.5 removed grants (implicit + ROPC)](https://datatracker.ietf.org/doc/html/draft-ietf-oauth-v2-1#section-7.5)
- [RFC 6750](https://datatracker.ietf.org/doc/html/rfc6750) — Bearer Token Usage
  - [Section 2 authenticated requests](https://datatracker.ietf.org/doc/html/rfc6750#section-2) · [Section 3 WWW-Authenticate](https://datatracker.ietf.org/doc/html/rfc6750#section-3)

## Client + native

- [RFC 7636](https://datatracker.ietf.org/doc/html/rfc7636) — PKCE
  - [Section 4.1 code_verifier 43–128 chars](https://datatracker.ietf.org/doc/html/rfc7636#section-4.1) · [Section 4.2 code_challenge](https://datatracker.ietf.org/doc/html/rfc7636#section-4.2) · [Section 4.6 server verification](https://datatracker.ietf.org/doc/html/rfc7636#section-4.6)
- [RFC 8252](https://datatracker.ietf.org/doc/html/rfc8252) — OAuth 2.0 for Native Apps
  - [Section 7.3 loopback redirect exception](https://datatracker.ietf.org/doc/html/rfc8252#section-7.3) · [Section 8.1 no `plain` PKCE for native](https://datatracker.ietf.org/doc/html/rfc8252#section-8.1)

## Client authentication

- [RFC 7523](https://datatracker.ietf.org/doc/html/rfc7523) — JWT profile (`client_secret_jwt`, `private_key_jwt`)
  - [Section 2.2 client authentication assertion](https://datatracker.ietf.org/doc/html/rfc7523#section-2.2) · [Section 3 JWT format](https://datatracker.ietf.org/doc/html/rfc7523#section-3)
- [RFC 8705](https://datatracker.ietf.org/doc/html/rfc8705) — mTLS client auth + certificate-bound tokens
  - [Section 2 mTLS client auth](https://datatracker.ietf.org/doc/html/rfc8705#section-2) · [Section 3 sender-constrained tokens](https://datatracker.ietf.org/doc/html/rfc8705#section-3)

## Server endpoints

- [RFC 7662](https://datatracker.ietf.org/doc/html/rfc7662) — Token Introspection
  - [Section 2.1 request](https://datatracker.ietf.org/doc/html/rfc7662#section-2.1) · [Section 2.2 response fields](https://datatracker.ietf.org/doc/html/rfc7662#section-2.2)
- [RFC 7009](https://datatracker.ietf.org/doc/html/rfc7009) — Token Revocation
  - [Section 2.1 request](https://datatracker.ietf.org/doc/html/rfc7009#section-2.1) · [Section 2.2 200 idempotent](https://datatracker.ietf.org/doc/html/rfc7009#section-2.2)
- [RFC 8414](https://datatracker.ietf.org/doc/html/rfc8414) — Authorization Server Metadata
  - [Section 2 required metadata](https://datatracker.ietf.org/doc/html/rfc8414#section-2) · [Section 3 obtaining metadata](https://datatracker.ietf.org/doc/html/rfc8414#section-3)
- [RFC 9068](https://datatracker.ietf.org/doc/html/rfc9068) — JWT Profile for Access Tokens
  - [Section 2.2 header + claims](https://datatracker.ietf.org/doc/html/rfc9068#section-2.2) · [Section 4 privacy considerations](https://datatracker.ietf.org/doc/html/rfc9068#section-4)

## Registration + resource + advanced

- [RFC 7591](https://datatracker.ietf.org/doc/html/rfc7591) — Dynamic Client Registration
  - [Section 3.1 registration endpoint](https://datatracker.ietf.org/doc/html/rfc7591#section-3.1)
- [RFC 7592](https://datatracker.ietf.org/doc/html/rfc7592) — DCR Management
- [RFC 8707](https://datatracker.ietf.org/doc/html/rfc8707) — Resource Indicators (`resource` param)
  - [Section 2 resource parameter](https://datatracker.ietf.org/doc/html/rfc8707#section-2)
- [RFC 9101](https://datatracker.ietf.org/doc/html/rfc9101) — JAR (Request Object)
- [RFC 9126](https://datatracker.ietf.org/doc/html/rfc9126) — PAR (Pushed Authorization Requests)
- [RFC 9449](https://datatracker.ietf.org/doc/html/rfc9449) — DPoP (Demonstrating Proof-of-Possession)
  - [Section 4 DPoP proof JWT](https://datatracker.ietf.org/doc/html/rfc9449#section-4)

## Security BCPs

- [RFC 6819](https://datatracker.ietf.org/doc/html/rfc6819) — OAuth 2.0 Threat Model + Security Considerations
- [draft-ietf-oauth-security-topics](https://datatracker.ietf.org/doc/html/draft-ietf-oauth-security-topics) — OAuth 2.0 Security BCP
  - [Section 2.1 recommendations](https://datatracker.ietf.org/doc/html/draft-ietf-oauth-security-topics#section-2.1) · [Section 4.8 refresh-token protection](https://datatracker.ietf.org/doc/html/draft-ietf-oauth-security-topics#section-4.8)

## OpenID Connect

- [OIDC Core 1.0](https://openid.net/specs/openid-connect-core-1_0.html) — Authentication
  - [Section 2 ID Token](https://openid.net/specs/openid-connect-core-1_0.html#IDToken) · [Section 3.1.3.7 ID Token validation](https://openid.net/specs/openid-connect-core-1_0.html#IDTokenValidation)
  - [Section 3.1.2.1 auth request](https://openid.net/specs/openid-connect-core-1_0.html#AuthRequest) · [Section 5.3 UserInfo](https://openid.net/specs/openid-connect-core-1_0.html#UserInfo)
- [OIDC Discovery 1.0](https://openid.net/specs/openid-connect-discovery-1_0.html)
  - [Section 4 obtaining OP config](https://openid.net/specs/openid-connect-discovery-1_0.html#ProviderConfig)
- [OIDC RP-Initiated Logout 1.0](https://openid.net/specs/openid-connect-rpinitiated-1_0.html)
  - [Section 2 logout endpoint](https://openid.net/specs/openid-connect-rpinitiated-1_0.html#RPLogout)
- [OIDC Session Management 1.0](https://openid.net/specs/openid-connect-session-1_0.html)
- [OIDC Back-Channel Logout 1.0](https://openid.net/specs/openid-connect-backchannel-1_0.html)
- [OIDC Front-Channel Logout 1.0](https://openid.net/specs/openid-connect-frontchannel-1_0.html)

## FAPI (high-assurance)

- [FAPI 2.0 Security Profile](https://openid.net/specs/fapi-2_0-security-profile.html)
- [FAPI 2.0 Message Signing](https://openid.net/specs/fapi-2_0-message-signing.html)
