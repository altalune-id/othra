// Package oidc serves each issuer's public OAuth and OpenID Connect documents: discovery, metadata and the JWKS.
package oidc

const (
	jwksPath      = "/oauth2/jwks.json"
	authorizePath = "/oauth2/authorize"
	tokenPath     = "/oauth2/token" //nolint:gosec // G101: an endpoint path, not a credential.
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
		GrantTypesSupported:              []string{"authorization_code"},
	}
}
