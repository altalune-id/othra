// Package oidc serves each issuer's public OAuth and OpenID Connect endpoints: discovery, metadata, the JWKS, token and userinfo.
package oidc

import "altalune.id/othra/internal/web/i18n"

const (
	jwksPath       = "/oauth2/jwks.json"
	authorizePath  = "/oauth2/authorize"
	tokenPath      = "/oauth2/token" //nolint:gosec // G101: an endpoint path, not a credential.
	userinfoPath   = "/oauth2/userinfo"
	endSessionPath = "/oauth2/end-session"
)

// Document is an issuer's OpenID Provider and OAuth authorization server metadata; it lists only what the issuer answers.
type Document struct {
	Issuer                           string   `json:"issuer"`
	AuthorizationEndpoint            string   `json:"authorization_endpoint"`
	TokenEndpoint                    string   `json:"token_endpoint"`
	JWKSURI                          string   `json:"jwks_uri"`
	ResponseTypesSupported           []string `json:"response_types_supported"`
	ResponseModesSupported           []string `json:"response_modes_supported"`
	SubjectTypesSupported            []string `json:"subject_types_supported"`
	IDTokenSigningAlgValuesSupported []string `json:"id_token_signing_alg_values_supported"`
	CodeChallengeMethodsSupported    []string `json:"code_challenge_methods_supported"`
	GrantTypesSupported              []string `json:"grant_types_supported"`
	UserinfoEndpoint                 string   `json:"userinfo_endpoint"`
	EndSessionEndpoint               string   `json:"end_session_endpoint"`
	ScopesSupported                  []string `json:"scopes_supported"`
	ClaimsSupported                  []string `json:"claims_supported"`
	TokenEndpointAuthMethods         []string `json:"token_endpoint_auth_methods_supported"`
	PromptValuesSupported            []string `json:"prompt_values_supported"`
	ISSParameterSupported            bool     `json:"authorization_response_iss_parameter_supported"`
	RequestParameterSupported        bool     `json:"request_parameter_supported"`
	RequestURIParameterSupported     bool     `json:"request_uri_parameter_supported"`
	ClaimsParameterSupported         bool     `json:"claims_parameter_supported"`
	UILocalesSupported               []string `json:"ui_locales_supported"`
}

// Discovery builds the metadata of the issuer at issuerURL.
func Discovery(issuerURL string) Document {
	return Document{
		Issuer:                           issuerURL,
		AuthorizationEndpoint:            issuerURL + authorizePath,
		TokenEndpoint:                    issuerURL + tokenPath,
		JWKSURI:                          issuerURL + jwksPath,
		ResponseTypesSupported:           []string{"code"},
		ResponseModesSupported:           []string{"query"},
		SubjectTypesSupported:            []string{"public"},
		IDTokenSigningAlgValuesSupported: []string{"RS256"},
		CodeChallengeMethodsSupported:    []string{"S256"},
		GrantTypesSupported:              []string{grantTypeCode, grantTypeRefresh},
		UserinfoEndpoint:                 issuerURL + userinfoPath,
		EndSessionEndpoint:               issuerURL + endSessionPath,
		ScopesSupported:                  []string{"openid", "profile", "email", "offline_access"},
		ClaimsSupported: []string{
			"sub", "iss", "aud", "exp", "iat", "auth_time", "nonce", "at_hash", "sid", "email", "email_verified", "name",
		},
		TokenEndpointAuthMethods: []string{authBasic, authPost, authNone},
		PromptValuesSupported:    []string{"none", "login", "select_account"},
		ISSParameterSupported:    true,
		UILocalesSupported:       i18n.Locales(),
	}
}
