package client

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"altalune.id/othra/apperror"
)

const claudeURL = "https://claude.ai/oauth/claude-code-client-metadata"

const claudeDocument = `{
  "client_id": "https://claude.ai/oauth/claude-code-client-metadata",
  "client_name": "Claude Code",
  "client_uri": "https://claude.ai",
  "redirect_uris": ["http://localhost/callback", "http://127.0.0.1/callback"],
  "grant_types": ["authorization_code", "refresh_token"],
  "response_types": ["code"],
  "token_endpoint_auth_method": "none",
  "something_new": {"ignored": true}
}`

func TestIsMetadataURL(t *testing.T) {
	assert.True(t, IsMetadataURL(claudeURL))
	assert.False(t, IsMetadataURL("V1StGXR8_Z5jdHi6B-myT"), "a registered client_id never takes the CIMD path")
	assert.False(t, IsMetadataURL("http://claude.ai/x"))
}

// the client_id URL rules.
func TestValidateMetadataURL(t *testing.T) {
	require.NoError(t, ValidateMetadataURL(claudeURL, "othra.example.com"))
	require.NoError(t, ValidateMetadataURL("https://app.example.com:8443/client.json", "othra.example.com"), "a port is allowed")
	for name, raw := range map[string]string{
		"http":            "http://claude.ai/oauth/x",
		"upper scheme":    "HTTPS://claude.ai/oauth/x",
		"no path":         "https://claude.ai",
		"root path":       "https://claude.ai/",
		"query":           "https://claude.ai/x?a=1",
		"empty query":     "https://claude.ai/x?",
		"fragment":        "https://claude.ai/x#f",
		"userinfo":        "https://u:p@claude.ai/x",
		"dot segment":     "https://claude.ai/a/../x",
		"single dot":      "https://claude.ai/./x",
		"ip literal":      "https://203.0.113.9/x",
		"ipv6 literal":    "https://[2001:db8::1]/x",
		"single label":    "https://intranet/x",
		"own host":        "https://othra.example.com/client.json",
		"own host, upper": "https://OTHRA.example.com/client.json",
		"too long":        "https://claude.ai/" + strings.Repeat("a", MaxMetadataURLLength),
		"space":           "https://claude.ai/a b",
	} {
		assert.Error(t, ValidateMetadataURL(raw, "othra.example.com"), name)
	}
}

// Claude Code's real document is admitted, as a native public client.
func TestParseMetadata_ClaudeCode(t *testing.T) {
	m, err := ParseMetadata(claudeURL, []byte(claudeDocument))
	require.NoError(t, err)
	assert.Equal(t, claudeURL, m.ClientID)
	assert.Equal(t, "Claude Code", m.ClientName)
	assert.Equal(t, "https://claude.ai", m.ClientURI)
	assert.Equal(t, []string{"http://localhost/callback", "http://127.0.0.1/callback"}, m.RedirectURIs)
	assert.True(t, m.Refresh)
	assert.Equal(t, "claude.ai", m.Host())
}

func TestParseMetadata_Refusals(t *testing.T) {
	for name, tc := range map[string]struct {
		doc    string
		reason MetadataReason
	}{
		"not json":            {`<html>`, MetadataNotJSON},
		"array":               {`[]`, MetadataNotJSON},
		"client_id differs":   {`{"client_id":"https://claude.ai/other","client_name":"C","redirect_uris":["http://localhost/cb"]}`, MetadataClientID},
		"client_id case":      {`{"client_id":"https://Claude.ai/oauth/claude-code-client-metadata","client_name":"C","redirect_uris":["http://localhost/cb"]}`, MetadataClientID},
		"no client_name":      {doc(`"redirect_uris":["http://localhost/cb"]`), MetadataClientName},
		"blank client_name":   {doc(`"client_name":"  ","redirect_uris":["http://localhost/cb"]`), MetadataClientName},
		"long client_name":    {doc(`"client_name":"` + strings.Repeat("é", 101) + `","redirect_uris":["http://localhost/cb"]`), MetadataClientName},
		"no redirect_uris":    {doc(`"client_name":"C"`), MetadataRedirectURIs},
		"https redirect":      {doc(`"client_name":"C","redirect_uris":["https://app.example/cb"]`), MetadataRedirectURIs},
		"loopback with port":  {doc(`"client_name":"C","redirect_uris":["http://localhost:8080/cb"]`), MetadataRedirectURIs},
		"secret basic":        {doc(`"client_name":"C","redirect_uris":["http://localhost/cb"],"token_endpoint_auth_method":"client_secret_basic"`), MetadataAuthMethod},
		"private_key_jwt":     {doc(`"client_name":"C","redirect_uris":["http://localhost/cb"],"token_endpoint_auth_method":"private_key_jwt"`), MetadataAuthMethod},
		"auth method omitted": {`{"client_id":"` + claudeURL + `","client_name":"C","redirect_uris":["http://localhost/cb"]}`, MetadataAuthMethod},
		"client_secret":       {doc(`"client_name":"C","redirect_uris":["http://localhost/cb"],"client_secret":"s"`), MetadataAuthMethod},
		"implicit grant":      {doc(`"client_name":"C","redirect_uris":["http://localhost/cb"],"grant_types":["implicit"]`), MetadataGrantTypes},
		"client_credentials":  {doc(`"client_name":"C","redirect_uris":["http://localhost/cb"],"grant_types":["client_credentials"]`), MetadataGrantTypes},
		"token response":      {doc(`"client_name":"C","redirect_uris":["http://localhost/cb"],"response_types":["token"]`), MetadataGrantTypes},
		"http client_uri":     {doc(`"client_name":"C","redirect_uris":["http://localhost/cb"],"client_uri":"http://claude.ai"`), MetadataURI},
		"http logo_uri":       {doc(`"client_name":"C","redirect_uris":["http://localhost/cb"],"logo_uri":"http://claude.ai/l.png"`), MetadataURI},
	} {
		_, err := ParseMetadata(claudeURL, []byte(tc.doc))
		me, ok := err.(*MetadataError) //nolint:errorlint // ParseMetadata returns it unwrapped.
		require.True(t, ok, "%s: %v", name, err)
		assert.Equal(t, tc.reason, me.Reason, name)
	}
}

func doc(fields string) string {
	return `{"client_id":"` + claudeURL + `","token_endpoint_auth_method":"none",` + fields + `}`
}

func TestParseMetadata_NoRefreshWithoutTheGrant(t *testing.T) {
	m, err := ParseMetadata(claudeURL, []byte(doc(`"client_name":"C","redirect_uris":["http://localhost/cb"]`)))
	require.NoError(t, err)
	assert.False(t, m.Refresh, "grant_types defaults to authorization_code")
}

// Cache-Control max-age clamped to the bounds; anything else gets the default.
func TestMetadataCache_TTL(t *testing.T) {
	c := MetadataCache{Min: 5 * time.Minute, Max: 24 * time.Hour, Default: time.Hour}
	for header, want := range map[string]time.Duration{
		"":                      time.Hour,
		"max-age=600":           10 * time.Minute,
		"public, max-age=60":    5 * time.Minute,
		"max-age=999999":        24 * time.Hour,
		"no-store":              5 * time.Minute,
		"no-cache, max-age=600": 5 * time.Minute,
		"max-age=abc":           time.Hour,
		"max-age=-5":            time.Hour,
		"MAX-AGE=900":           15 * time.Minute,
	} {
		assert.Equal(t, want, c.TTL(header), header)
	}
}

func TestIsJSONMediaType(t *testing.T) {
	for raw, want := range map[string]bool{
		"application/json":                true,
		"application/json; charset=utf-8": true,
		"application/oauth-client+json":   true,
		"text/html":                       false,
		"":                                false,
		"application/jsonx":               false,
	} {
		assert.Equal(t, want, IsJSONMediaType(raw), raw)
	}
}

// every admission miss is SIG010; an unusable document is SIG011, whose reason stays out of the response.
func TestCIMDErrors_AppErrors(t *testing.T) {
	app := (&MetadataError{Reason: MetadataTooLarge}).ToAppError()
	assert.Equal(t, apperror.CodeAuthorizeClientMetadataUnusable, app.Code())
	assert.Empty(t, app.Meta())
	app = (&NotAdmittedError{Reason: AdmissionHost}).ToAppError()
	assert.Equal(t, apperror.CodeAuthorizeClientNotAdmitted, app.Code())
	assert.Empty(t, app.Meta())
	assert.True(t, IsNotAdmittedError(&NotAdmittedError{}))
}
