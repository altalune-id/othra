package resourceserver

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateIdentifier(t *testing.T) {
	for raw, want := range map[string]IdentifierReason{
		"https://api.example.com":             "",
		"https://api.example.com/mcp":         "",
		"https://api.example.com:8443/v1/":    "",
		"https://api.example.com/a%2Fb":       "",
		"https://[2001:db8::1]/api":           "",
		"http://127.0.0.1:8080/mcp":           "",
		"http://localhost:5173/api":           "",
		"http://[::1]:9000":                   "",
		"":                                    IdentifierUnparsable,
		"api.example.com":                     IdentifierUnparsable,
		"urn:openwa:api":                      IdentifierUnparsable,
		"https://api.example.com/a b":         IdentifierUnparsable,
		"https://%zz":                         IdentifierUnparsable,
		"http://api.example.com":              IdentifierHTTPSRequired,
		"http://127.0.0.2":                    IdentifierHTTPSRequired,
		"ftp://api.example.com":               IdentifierHTTPSRequired,
		"https://api.example.com/?x=1":        IdentifierQuery,
		"https://api.example.com/?":           IdentifierQuery,
		"https://api.example.com/#top":        IdentifierFragment,
		"https://api.example.com/#":           IdentifierFragment,
		"https://user:pw@api.example.com":     IdentifierUserinfo,
		"https://API.example.com":             IdentifierNotCanonical,
		"HTTPS://api.example.com":             IdentifierNotCanonical,
		"https://api.example.com/":            IdentifierNotCanonical,
		"https://api.example.com:443/mcp":     IdentifierNotCanonical,
		"https://api.example.com./mcp":        IdentifierNotCanonical,
		"https://api.example.com/a/../mcp":    IdentifierNotCanonical,
		"https://api.example.com/%7euser":     IdentifierNotCanonical,
		"https://api.example.com/a%2fb":       IdentifierNotCanonical,
		"https://" + strings.Repeat("a", 512): IdentifierTooLong,
	} {
		err := ValidateIdentifier(raw)
		if want == "" {
			assert.NoError(t, err, raw)
			continue
		}
		e, ok := errors.AsType[*InvalidIdentifierError](err)
		require.True(t, ok, "%s: %v", raw, err)
		assert.Equal(t, want, e.Reason, raw)
	}
}

func TestValidateIdentifier_NamesTheCanonicalForm(t *testing.T) {
	for raw, want := range map[string]string{
		"https://API.Example.com:443/":     "https://api.example.com",
		"https://api.example.com/a/../mcp": "https://api.example.com/mcp",
		"https://api.example.com/%7e%2f":   "https://api.example.com/~%2F",
		"HTTP://LOCALHOST:80/api":          "http://localhost/api",
	} {
		err := ValidateIdentifier(raw)
		e, ok := errors.AsType[*InvalidIdentifierError](err)
		require.True(t, ok, "%s: %v", raw, err)
		assert.Equal(t, want, e.Canonical, raw)
		assert.NoError(t, ValidateIdentifier(want), "the canonical form of %s is valid", raw)
	}
}

func TestIsLoopbackHTTP(t *testing.T) {
	assert.True(t, IsLoopbackHTTP("http://127.0.0.1:8080/mcp"))
	assert.True(t, IsLoopbackHTTP("http://[::1]"))
	assert.False(t, IsLoopbackHTTP("https://127.0.0.1"))
	assert.False(t, IsLoopbackHTTP("http://api.example.com"))
}

func TestMetadataURL(t *testing.T) {
	assert.Equal(t, "https://api.example.com/.well-known/oauth-protected-resource", MetadataURL("https://api.example.com"))
	assert.Equal(t, "https://api.example.com:8443/.well-known/oauth-protected-resource/v1/mcp", MetadataURL("https://api.example.com:8443/v1/mcp/"))
	assert.Empty(t, MetadataURL("::"))
}

func TestValidateScopes(t *testing.T) {
	assert.NoError(t, ValidateScopes([]string{"devices:read", "cache:purge", "A~z!#[]"}))
	many := make([]string, MaxScopes+1)
	for i := range many {
		many[i] = "s" + strings.Repeat("x", i)
	}
	for name, tc := range map[string]struct {
		scopes []string
		reason ScopeReason
		scope  string
	}{
		"empty list":       {nil, ScopeFormat, ""},
		"too many":         {many, ScopeTooMany, ""},
		"empty name":       {[]string{""}, ScopeFormat, ""},
		"space":            {[]string{"a b"}, ScopeFormat, "a b"},
		"quote":            {[]string{`a"b`}, ScopeFormat, `a"b`},
		"backslash":        {[]string{`a\b`}, ScopeFormat, `a\b`},
		"non-ascii":        {[]string{"é"}, ScopeFormat, "é"},
		"too long":         {[]string{strings.Repeat("a", MaxScopeLength+1)}, ScopeFormat, strings.Repeat("a", MaxScopeLength+1)},
		"reserved openid":  {[]string{"read", "openid"}, ScopeReserved, "openid"},
		"reserved offline": {[]string{"offline_access"}, ScopeReserved, "offline_access"},
		"duplicate":        {[]string{"read", "read"}, ScopeDuplicate, "read"},
	} {
		err := ValidateScopes(tc.scopes)
		e, ok := errors.AsType[*InvalidScopeError](err)
		require.True(t, ok, "%s: %v", name, err)
		assert.Equal(t, tc.reason, e.Reason, name)
		assert.Equal(t, tc.scope, e.Scope, name)
	}
}

func TestLinkScopes(t *testing.T) {
	r := &ResourceServer{Scopes: []string{"read", "write"}}
	got, err := r.LinkScopes([]string{"write", "read", "write"})
	require.NoError(t, err)
	assert.Equal(t, []string{"write", "read"}, got)

	_, err = r.LinkScopes(nil)
	assert.Equal(t, &InvalidLinkScopesError{Reason: LinkScopesEmpty}, err)
	_, err = r.LinkScopes([]string{"read", "admin"})
	assert.Equal(t, &InvalidLinkScopesError{Scope: "admin", Reason: LinkScopesNotInCatalog}, err)
}

func TestRemoved(t *testing.T) {
	r := &ResourceServer{Scopes: []string{"read", "write", "admin"}}
	assert.Equal(t, []string{"write", "admin"}, r.Removed([]string{"read", "new"}))
	assert.Empty(t, r.Removed([]string{"admin", "write", "read"}))
}

func TestPassAndFail(t *testing.T) {
	now := time.Date(2026, 10, 20, 9, 0, 0, 0, time.UTC)
	r := &ResourceServer{State: StatePending}
	r.Fail(Failure{Reason: FailureNotFound, CheckedAt: now})
	r.Fail(Failure{Reason: FailureNotFound, CheckedAt: now})
	r.Fail(Failure{Reason: FailureNotFound, CheckedAt: now})
	assert.Equal(t, StatePending, r.State, "a pending server stays pending")
	assert.Equal(t, int32(3), r.ConsecutiveFailures)

	r.Pass(now)
	assert.Equal(t, StateVerified, r.State)
	assert.Zero(t, r.ConsecutiveFailures)
	assert.Nil(t, r.LastError)
	assert.Equal(t, now, *r.VerifiedAt)

	r.Fail(Failure{Reason: FailureTimeout, CheckedAt: now.Add(time.Hour)})
	r.Fail(Failure{Reason: FailureTimeout, CheckedAt: now.Add(2 * time.Hour)})
	assert.Equal(t, StateVerified, r.State, "two failures leave it verified")
	r.Fail(Failure{Reason: FailureStatus, Status: 503, CheckedAt: now.Add(3 * time.Hour)})
	assert.Equal(t, StateFailed, r.State)
	assert.Equal(t, now.Add(3*time.Hour), *r.LastCheckedAt)
	assert.Equal(t, int32(503), r.LastError.Status)

	r.Pass(now.Add(4 * time.Hour))
	assert.Equal(t, StateVerified, r.State, "a pass restores a failed server")
}

func TestTrustAndUntrust(t *testing.T) {
	now := time.Date(2026, 10, 20, 9, 0, 0, 0, time.UTC)
	user := uuid.New()
	r := &ResourceServer{Identifier: "https://api.example.com", State: StateFailed, VerifiedAt: &now, ConsecutiveFailures: 3}
	r.Trust(user, now)
	assert.True(t, r.Trusted)
	assert.Equal(t, user, r.TrustedBy)
	require.NoError(t, r.Untrust())
	assert.False(t, r.Trusted)
	assert.Nil(t, r.TrustedAt)
	assert.Equal(t, StatePending, r.State)
	assert.Nil(t, r.VerifiedAt)
	assert.Zero(t, r.ConsecutiveFailures)

	loop := &ResourceServer{Identifier: "http://127.0.0.1:8080"}
	loop.Trust(user, now)
	err := loop.Untrust()
	assert.Equal(t, IdentifierNeedsTrust, asError[*InvalidIdentifierError](t, err).Reason)
	assert.True(t, loop.Trusted)
}

// one admission check for /authorize, /token and CIMD admission.
func TestAdmissible(t *testing.T) {
	for _, tc := range []struct {
		state          State
		trusted, avail bool
		want           bool
	}{
		{StateVerified, false, false, true},
		{StateVerified, false, true, true},
		{StatePending, false, true, false},
		{StateFailed, false, true, false},
		{StatePending, true, true, true},
		{StateFailed, true, true, true},
		{StatePending, true, false, false},
	} {
		r := &ResourceServer{State: tc.state, Trusted: tc.trusted}
		assert.Equal(t, tc.want, r.Admissible(tc.avail), "%+v", tc)
	}
}

// RFC 9728 §3.3: a document that names another resource or omits the issuer fails a server at once; other reasons take three strikes.
func TestFail_Policy(t *testing.T) {
	now := time.Date(2026, 10, 20, 9, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		from    State
		reason  FailureReason
		strikes int
		want    State
	}{
		{StatePending, FailureResourceMismatch, 1, StateFailed},
		{StatePending, FailureIssuerMissing, 1, StateFailed},
		{StateVerified, FailureResourceMismatch, 1, StateFailed},
		{StateVerified, FailureIssuerMissing, 1, StateFailed},
		{StatePending, FailureNetwork, 3, StatePending},
		{StatePending, FailureStatus, 5, StatePending},
		{StateVerified, FailureTimeout, 2, StateVerified},
		{StateVerified, FailureTimeout, 3, StateFailed},
		{StateVerified, FailureNotFound, 3, StateFailed},
	} {
		r := &ResourceServer{State: tc.from}
		for range tc.strikes {
			r.Fail(Failure{Reason: tc.reason, CheckedAt: now})
		}
		assert.Equal(t, tc.want, r.State, "%+v", tc)
		assert.Equal(t, int32(tc.strikes), r.ConsecutiveFailures, "%+v", tc)
	}
}

// untrusting clears the last check, so the next scheduled run checks the server.
func TestUntrust_ClearsTheLastCheck(t *testing.T) {
	now := time.Date(2026, 10, 20, 9, 0, 0, 0, time.UTC)
	r := &ResourceServer{Identifier: "https://api.example.com", LastCheckedAt: &now}
	r.Trust(uuid.New(), now)
	require.NoError(t, r.Untrust())
	assert.Nil(t, r.LastCheckedAt)
}
