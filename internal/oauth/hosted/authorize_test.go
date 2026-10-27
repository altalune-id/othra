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
	out := baseClients()
	for id, c := range out {
		c.RowID = rowID(id)
		out[id] = c
	}
	return out
}

func baseClients() fakeClients {
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
		"nat": {
			ID: "nat", Name: "Native App", Native: true, RedirectURIs: []string{cliRedirect},
			AllowedScopes: []string{"openid", "profile", "email", "offline_access"}, Refresh: true, PKCERequired: true, ConsentRequired: true,
		},
		"req": {
			ID: "req", Name: "Require App", Confidential: true, RedirectURIs: []string{webRedirect},
			AllowedScopes: []string{"openid", "profile", "email"}, PKCERequired: true, ConsentRequired: true,
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

func testOAuth() OAuth {
	return OAuth{Clients: testClients(), Resources: testResources(), Admissions: &fakeAdmissions{}}
}

func parse(t *testing.T, q url.Values) (*authRequest, error) {
	t.Helper()
	return parseAuthorize(context.Background(), testOAuth(), Project{}, q)
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

// a native client's loopback redirect matches on any port; a web one only exactly.
func TestParseAuthorize_RedirectMatch(t *testing.T) {
	req, err := parse(t, valid("client_id", "cli", "redirect_uri", "http://127.0.0.1:53123/callback"))
	require.NoError(t, err)
	assert.Equal(t, "http://127.0.0.1:53123/callback", req.redirectURI, "the redirect goes back exactly as sent")
	_, err = parse(t, valid("redirect_uri", "https://app.example/cb/"))
	assert.Equal(t, apperror.CodeAuthorizeRedirectNotRegistered, pageCode(t, err))
}

// before the redirect URI is trusted, errors are a page, never a redirect.
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

// once the redirect is trusted, errors go back to the client.
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
		assert.Regexp(t, `^[\x20-\x21\x23-\x5B\x5D-\x7E]*$`, re.description, "%s: a safe description", name)
	}
}

// unknown scopes are dropped; offline_access only for a client that may refresh.
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

// a confidential client with PKCE turned off may skip it when it sends a nonce.
func TestParseAuthorize_PKCEOffNeedsNonce(t *testing.T) {
	req, err := parse(t, valid("client_id", "conf", "code_challenge", "-", "code_challenge_method", "-", "nonce", "n"))
	require.NoError(t, err)
	assert.Empty(t, req.challenge)
	req, err = parse(t, valid("client_id", "conf"))
	require.NoError(t, err, "PKCE is still honoured when sent")
	assert.Equal(t, challenge, req.challenge)
}

func redirectCode(t *testing.T, name string, err error) string {
	t.Helper()
	re, ok := err.(*authorizeError) //nolint:errorlint // parseAuthorize returns it unwrapped.
	require.True(t, ok, "%s: %v", name, err)
	assert.NotEmpty(t, re.request, name)
	return re.code
}

// one exact resource, linked to the client; its scopes narrowed to the link's pin.
func TestParseAuthorize_Resource(t *testing.T) {
	cli := func(extra ...string) url.Values {
		return valid(append([]string{"client_id", "cli", "redirect_uri", cliRedirect}, extra...)...)
	}
	req, err := parse(t, cli("resource", mcpResource, "scope", "openid sample:read sample:write offline_access"))
	require.NoError(t, err)
	require.NotNil(t, req.resource)
	assert.Equal(t, mcpResource, req.resource.Identifier)
	assert.Equal(t, []string{"openid", "sample:read", "offline_access"}, req.scopes, "sample:write is outside the pin, so narrowed")

	q := cli("scope", "sample:read")
	q["resource"] = []string{mcpResource, mcpResource}
	req, err = parse(t, q)
	require.NoError(t, err, "a repeated identical resource is one")
	assert.Equal(t, []string{"sample:read"}, req.scopes)

	req, err = parse(t, cli("scope", "openid sample:read"))
	require.NoError(t, err)
	assert.Nil(t, req.resource)
	assert.Equal(t, []string{"openid"}, req.scopes, "without a resource, its scopes are unknown")

	two := cli("scope", "sample:read")
	two["resource"] = []string{mcpResource, otherRS}
	for name, q := range map[string]url.Values{
		"two resources":  two,
		"unknown":        cli("resource", "https://nope.test/mcp"),
		"not linked":     cli("resource", otherRS),
		"trailing slash": cli("resource", mcpResource+"/"),
		"relative":       cli("resource", "/mcp"),
		"fragment":       cli("resource", mcpResource+"#f"),
		"too long":       cli("resource", mcpResource+"/"+strings.Repeat("a", maxParamLen)),
	} {
		_, err := parse(t, q)
		assert.Equal(t, "invalid_target", redirectCode(t, name, err), name)
	}
	_, err = parse(t, cli("resource", mcpResource, "scope", "sample:write"))
	assert.Equal(t, "invalid_scope", redirectCode(t, "only outside the pin", err), "nothing left")
	_, err = parse(t, cli("resource", mcpResource, "scope", "openid sample:write"))
	assert.Equal(t, "invalid_scope", redirectCode(t, "only openid left", err), "no resource scope left, openid doesn't mask it")
}

// a client_id URL is admitted through the request's resource before its redirect is trusted.
func TestParseAuthorize_MetadataClient(t *testing.T) {
	q := valid("client_id", cimdClient, "redirect_uri", "http://localhost:53999/callback", "resource", mcpResource, "scope", "sample:read")
	req, err := parse(t, q)
	require.NoError(t, err)
	assert.Equal(t, cimdClient, req.client.ID)
	assert.Equal(t, mcpResource, req.resource.Identifier)
	assert.Equal(t, []string{"sample:read"}, req.scopes)

	for name, tc := range map[string]struct {
		q    url.Values
		code apperror.Code
	}{
		"no resource":     {valid("client_id", cimdClient, "redirect_uri", "http://localhost/callback"), apperror.CodeAuthorizeClientNotAdmitted},
		"not admitted":    {valid("client_id", cimdClient, "redirect_uri", "http://localhost/callback", "resource", otherRS), apperror.CodeAuthorizeClientNotAdmitted},
		"other document":  {valid("client_id", "https://evil.example/c.json", "redirect_uri", "http://localhost/callback", "resource", mcpResource), apperror.CodeAuthorizeClientNotAdmitted},
		"redirect not in": {valid("client_id", cimdClient, "redirect_uri", "http://localhost/other", "resource", mcpResource), apperror.CodeAuthorizeRedirectNotRegistered},
	} {
		_, err := parse(t, tc.q)
		assert.Equal(t, tc.code, pageCode(t, err), name)
	}
	o := testOAuth()
	o.Admissions = &fakeAdmissions{fail: apperror.New(apperror.CodeAuthorizeClientMetadataUnusable, "unusable")}
	_, err = parseAuthorize(context.Background(), o, Project{}, q)
	assert.Equal(t, apperror.CodeAuthorizeClientMetadataUnusable, pageCode(t, err), "SIG011 is a page too")
	o.Admissions = &fakeAdmissions{fail: errFake}
	_, err = parseAuthorize(context.Background(), o, Project{}, q)
	assert.ErrorIs(t, err, errFake)
}
