// Package oidc serves each issuer's public OAuth and OpenID Connect endpoints: discovery, metadata, the JWKS, token and userinfo.
package oidc

import "altalune.id/othra/internal/web/i18n"

const (
	jwksPath       = "/oauth2/jwks.json"
	authorizePath  = "/oauth2/authorize"
	tokenPath      = "/oauth2/token" //nolint:gosec // G101: an endpoint path, not a credential.
	userinfoPath   = "/oauth2/userinfo"
	endSessionPath = "/oauth2/end-session"
	revokePath     = "/oauth2/revoke"
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
	RevocationEndpoint               string   `json:"revocation_endpoint"`
	RevocationAuthMethods            []string `json:"revocation_endpoint_auth_methods_supported"`
	IntrospectionEndpoint            string   `json:"introspection_endpoint,omitempty"`
	IntrospectionAuthMethods         []string `json:"introspection_endpoint_auth_methods_supported,omitempty"`
	// ClientIDMetadataDocumentSupported is always sent (CIMD §6); true only while a verified resource server admits CIMD clients.
	ClientIDMetadataDocumentSupported bool `json:"client_id_metadata_document_supported"`
}

// Features are the optional parts of an issuer's metadata.
type Features struct {
	// CIMD says whether the issuer admits CIMD clients.
	CIMD bool
	// Introspection says whether the issuer serves the introspection endpoint.
	Introspection bool
}

// Discovery builds the metadata of the issuer at issuerURL with the features f.
func Discovery(issuerURL string, f Features) Document {
	d := Document{
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
		TokenEndpointAuthMethods:          []string{authBasic, authPost, authNone},
		PromptValuesSupported:             []string{"none", "login", "consent", "select_account"},
		ISSParameterSupported:             true,
		UILocalesSupported:                i18n.Locales(),
		RevocationEndpoint:                issuerURL + revokePath,
		RevocationAuthMethods:             []string{authBasic, authPost, authNone},
		ClientIDMetadataDocumentSupported: f.CIMD,
	}
	if f.Introspection {
		d.IntrospectionEndpoint = issuerURL + introspectPath
		d.IntrospectionAuthMethods = []string{authBasic, authPost}
	}
	return d
}
