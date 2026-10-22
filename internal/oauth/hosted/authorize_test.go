package hosted

import (
	"context"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"altalune.id/othra/apperror"
)

const (
	webRedirect = "https://app.example/cb"
	cliRedirect = "http://127.0.0.1/callback"
	byeRedirect = "https://app.example/bye"
	challenge   = "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM"
)

type fakeClients map[string]Client

func (f fakeClients) Client(_ context.Context, _ Project, id string) (Client, bool, error) {
	c, ok := f[id]
	return c, ok, nil
}

func testClients() fakeClients {
	return fakeClients{
		"web": {
			ID: "web", Name: "Openwa Web", Confidential: true, RedirectURIs: []string{webRedirect}, PostLogoutRedirectURIs: []string{byeRedirect},
			AllowedScopes: []string{"openid", "profile", "email"}, PKCERequired: true,
		},
		"cli": {
			ID: "cli", Name: "Openwa CLI", Native: true, RedirectURIs: []string{cliRedirect},
			AllowedScopes: []string{"openid", "profile", "email", "offline_access"}, Refresh: true, PKCERequired: true,
		},
		"narrow": {
			ID: "narrow", Name: "Narrow", Confidential: true, RedirectURIs: []string{webRedirect},
			AllowedScopes: []string{"openid"}, PKCERequired: true,
		},
		"conf": {
			ID: "conf", Name: "Conformance", Confidential: true, RedirectURIs: []string{webRedirect},
			AllowedScopes: []string{"openid", "profile", "email", "offline_access"}, Refresh: true,
		},
	}
}

func valid(extra ...string) url.Values {
	q := url.Values{
		"client_id": {"web"}, "redirect_uri": {webRedirect}, "response_type": {"code"}, "scope": {"openid email"},
		"state": {"st"}, "code_challenge": {challenge}, "code_challenge_method": {"S256"},
	}
	for i := 0; i+1 < len(extra); i += 2 {
		if extra[i+1] == "-" {
			q.Del(extra[i])
			continue
		}
		q.Set(extra[i], extra[i+1])
	}
	return q
}

func parse(t *testing.T, q url.Values) (*authRequest, error) {
	t.Helper()
	return parseAuthorize(context.Background(), testClients(), Project{}, q)
}

func TestParseAuthorize_Valid(t *testing.T) {
	req, err := parse(t, valid("nonce", "n-1", "prompt", "select_account", "max_age", "300", "login_hint", "ana@example.com",
		"access_type", "offline", "display", "page", "ui_locales", "id", "claims", `{"x":1}`, "acr_values", "1"))
	require.NoError(t, err)
	assert.Equal(t, "web", req.client.ID)
	assert.Equal(t, webRedirect, req.redirectURI)
	assert.Equal(t, []string{"openid", "email"}, req.scopes)
	assert.Equal(t, "st", req.state)
	assert.Equal(t, "n-1", req.nonce)
	assert.Equal(t, challenge, req.challenge)
	assert.True(t, req.prompt[promptSelectAccount])
	assert.Equal(t, 300, req.maxAge)
	assert.True(t, req.hasMaxAge)
	assert.Equal(t, "ana@example.com", req.loginHint)
}

// A3: a native client's loopback redirect matches on any port; a web one only exactly.
func TestParseAuthorize_RedirectMatch(t *testing.T) {
	req, err := parse(t, valid("client_id", "cli", "redirect_uri", "http://127.0.0.1:53123/callback"))
	require.NoError(t, err)
	assert.Equal(t, "http://127.0.0.1:53123/callback", req.redirectURI, "the redirect goes back exactly as sent")
	_, err = parse(t, valid("redirect_uri", "https://app.example/cb/"))
	assert.Equal(t, apperror.CodeAuthorizeRedirectNotRegistered, pageCode(t, err))
}

// A2, A16, A6: before the redirect URI is trusted, errors are a page, never a redirect.
func TestParseAuthorize_PageErrors(t *testing.T) {
	for name, tc := range map[string]struct {
		q    url.Values
		code apperror.Code
	}{
		"no client":          {valid("client_id", "-"), apperror.CodeAuthorizeClientUnknown},
		"unknown client":     {valid("client_id", "nope"), apperror.CodeAuthorizeClientUnknown},
		"no redirect":        {valid("redirect_uri", "-"), apperror.CodeAuthorizeRedirectNotRegistered},
		"unregistered":       {valid("redirect_uri", "https://evil.example/cb"), apperror.CodeAuthorizeRedirectNotRegistered},
		"repeated client":    {url.Values{"client_id": {"web", "web"}, "redirect_uri": {webRedirect}}, apperror.CodeAuthorizeRequestInvalid},
		"form_post":          {valid("response_mode", "form_post"), apperror.CodeAuthorizeRequestInvalid},
		"fragment":           {valid("response_mode", "fragment"), apperror.CodeAuthorizeRequestInvalid},
		"long state":         {valid("state", strings.Repeat("s", maxParamLen+1)), apperror.CodeAuthorizeRequestInvalid},
		"request, bad outer": {valid("request", "eyJ...", "redirect_uri", "https://evil.example/cb"), apperror.CodeAuthorizeRedirectNotRegistered},
		"repeated redirect":  {func() url.Values { q := valid(); q["redirect_uri"] = []string{webRedirect, webRedirect}; return q }(), apperror.CodeAuthorizeRequestInvalid},
	} {
		_, err := parse(t, tc.q)
		assert.Equal(t, tc.code, pageCode(t, err), name)
	}
	_, err := parse(t, valid("response_mode", "query"))
	assert.NoError(t, err, "an explicit query mode is fine")
}

func pageCode(t *testing.T, err error) apperror.Code {
	t.Helper()
	pe, ok := err.(*RefusedError) //nolint:errorlint // parseAuthorize returns it unwrapped.
	require.True(t, ok, "%v", err)
	return pe.Code
}

// A4, A5, A9, A10, A15, A19, P1–P3, D1: once the redirect is trusted, errors go back to the client.
func TestParseAuthorize_RedirectErrors(t *testing.T) {
	for name, tc := range map[string]struct {
		q    url.Values
		want string
	}{
		"no response_type":     {valid("response_type", "-"), "invalid_request"},
		"token response_type":  {valid("response_type", "token"), "unsupported_response_type"},
		"code id_token":        {valid("response_type", "code id_token"), "unsupported_response_type"},
		"request object":       {valid("request", "eyJ..."), "request_not_supported"},
		"request uri":          {valid("request_uri", "https://x"), "request_uri_not_supported"},
		"registration":         {valid("registration", "{}"), "registration_not_supported"},
		"no scope":             {valid("scope", "-"), "invalid_scope"},
		"only unknown scopes":  {valid("scope", "address phone"), "invalid_scope"},
		"scope not allowed":    {valid("scope", "openid profile", "client_id", "narrow"), "invalid_scope"},
		"no pkce":              {valid("code_challenge", "-", "code_challenge_method", "-"), "invalid_request"},
		"plain pkce":           {valid("code_challenge_method", "plain"), "invalid_request"},
		"pkce without method":  {valid("code_challenge_method", "-"), "invalid_request"},
		"short challenge":      {valid("code_challenge", "abc"), "invalid_request"},
		"bad challenge chars":  {valid("code_challenge", strings.Repeat("+", 43)), "invalid_request"},
		"challenge too long":   {valid("code_challenge", challenge+"A"), "invalid_request"},
		"challenge 128 chars":  {valid("code_challenge", strings.Repeat("A", 128)), "invalid_request"},
		"challenge with tilde": {valid("code_challenge", challenge[:42]+"~"), "invalid_request"},
		"challenge with dot":   {valid("code_challenge", challenge[:42]+"."), "invalid_request"},
		"pkce off, no nonce":   {valid("client_id", "conf", "code_challenge", "-", "code_challenge_method", "-"), "invalid_request"},
		"prompt none + login":  {valid("prompt", "none login"), "invalid_request"},
		"prompt none + select": {valid("prompt", "none select_account"), "invalid_request"},
		"bad max_age":          {valid("max_age", "-1"), "invalid_request"},
		"max_age not a number": {valid("max_age", "soon"), "invalid_request"},
		"repeated scope":       {func() url.Values { q := valid(); q["scope"] = []string{"openid", "email"}; return q }(), "invalid_request"},
	} {
		_, err := parse(t, tc.q)
		re, ok := err.(*authorizeError) //nolint:errorlint // parseAuthorize returns it unwrapped.
		require.True(t, ok, "%s: %v", name, err)
		assert.Equal(t, tc.want, re.code, name)
		assert.NotEmpty(t, re.request, "%s: the redirect is known", name)
		assert.Regexp(t, `^[\x20-\x21\x23-\x5B\x5D-\x7E]*$`, re.description, "%s: E2 safe description", name)
	}
}

// A5, A19: unknown scopes are dropped; offline_access only for a client that may refresh.
func TestParseAuthorize_Scopes(t *testing.T) {
	req, err := parse(t, valid("scope", "email address openid"))
	require.NoError(t, err)
	assert.Equal(t, []string{"email", "openid"}, req.scopes)
	req, err = parse(t, valid("client_id", "cli", "redirect_uri", cliRedirect, "scope", "openid offline_access"))
	require.NoError(t, err)
	assert.Equal(t, []string{"openid", "offline_access"}, req.scopes)
	req, err = parse(t, valid("scope", "openid offline_access"))
	require.NoError(t, err)
	assert.Equal(t, []string{"openid"}, req.scopes, "a client that can't refresh has offline_access dropped, not refused")
}

// D1: a confidential client with PKCE turned off may skip it when it sends a nonce.
func TestParseAuthorize_PKCEOffNeedsNonce(t *testing.T) {
	req, err := parse(t, valid("client_id", "conf", "code_challenge", "-", "code_challenge_method", "-", "nonce", "n"))
	require.NoError(t, err)
	assert.Empty(t, req.challenge)
	req, err = parse(t, valid("client_id", "conf"))
	require.NoError(t, err, "PKCE is still honoured when sent")
	assert.Equal(t, challenge, req.challenge)
}
