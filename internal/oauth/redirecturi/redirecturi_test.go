package redirecturi

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func reasonOf(t *testing.T, err error) Reason {
	t.Helper()
	if err == nil {
		return ""
	}
	e, ok := errors.AsType[*Error](err)
	require.True(t, ok, "%v", err)
	return e.Reason
}

func TestValidate(t *testing.T) {
	for _, tt := range []struct {
		kind ApplicationType
		uri  string
		want Reason
	}{
		{Web, "https://app.example/callback", ""},
		{Web, "https://app.example/cb?tenant=a", ""},
		{Web, "https://app.example:8443/cb", ""},
		{Web, "https://app.example/", ""},
		{Web, "http://127.0.0.1:3000/cb", ""},
		{Web, "http://[::1]:3000/cb", ""},
		{Web, "http://localhost:3000/cb", ""},
		{Web, "http://127.0.0.1:0/cb", Unparsable},
		{Web, "http://127.0.0.1:99999/cb", Unparsable},
		{Web, "http://127.0.0.1:03000/cb", Unparsable},
		{Web, "https://app.example:0/cb", Unparsable},
		{Web, "", Required},
		{Web, "https://app.example/" + strings.Repeat("a", MaxLength), TooLong},
		{Web, "https://*.app.example/cb", Wildcard},
		{Web, "https://app.example/cb#x", Fragment},
		{Web, "https://app.example/cb#", Fragment},
		{Web, "app.example/cb", Unparsable},
		{Web, "/cb", Unparsable},
		{Web, "https:///cb", Unparsable},
		{Web, "https://app.example/a b", Unparsable},
		{Web, "mailto:a@b.example", Unparsable},
		{Web, "https://user:pass@app.example/cb", Userinfo},
		{Web, "https://app.example/a/../cb", DotSegments},
		{Web, "https://app.example/./cb", DotSegments},
		{Web, "https://app.example/%2E%2E/cb", DotSegments},
		{Web, "http://app.example/cb", HTTPSRequired},
		{Web, "http://127.0.0.1/cb", HTTPSRequired},
		{Web, "http://127.0.0.1:80/cb", HTTPSRequired},
		{Web, "http://127.0.0.2:3000/cb", HTTPSRequired},
		{Web, "http://0.0.0.0:3000/cb", HTTPSRequired},
		{Web, "http://app.localhost:3000/cb", HTTPSRequired},
		{Web, "myapp://cb", HTTPSRequired},
		{Web, "https://App.Example/cb", NotCanonical},
		{Web, "HTTPS://app.example/cb", NotCanonical},
		{Web, "https://app.example:443/cb", NotCanonical},
		{Web, "https://app.example", NotCanonical},
		{Native, "http://127.0.0.1/callback", ""},
		{Native, "http://[::1]/callback", ""},
		{Native, "http://localhost/callback", ""},
		{Native, "http://127.0.0.1/", ""},
		{Native, "http://127.0.0.1:8080/callback", PortIgnored},
		{Native, "http://localhost:80/callback", PortIgnored},
		{Native, "https://127.0.0.1/callback", LoopbackOnly},
		{Native, "https://localhost/callback", LoopbackOnly},
		{Native, "http://127.0.0.2/callback", LoopbackOnly},
		{Native, "http://0.0.0.0/callback", LoopbackOnly},
		{Native, "http://app.localhost/callback", LoopbackOnly},
		{Native, "http://app.example/callback", LoopbackOnly},
		{Native, "https://app.example/callback", LoopbackOnly},
		{Native, "myapp:/callback", Unparsable},
		{Native, "http://LOCALHOST/callback", NotCanonical},
		{Native, "http://127.0.0.1", NotCanonical},
		{Native, "http://127.0.0.1/cb#x", Fragment},
		{Native, "http://u@127.0.0.1/cb", Userinfo},
	} {
		t.Run(string(tt.kind)+" "+tt.uri[:min(len(tt.uri), 60)], func(t *testing.T) {
			assert.Equal(t, tt.want, reasonOf(t, Validate(tt.kind, tt.uri)))
		})
	}
}

func TestValidate_NamesTheCanonicalForm(t *testing.T) {
	for uri, want := range map[string]string{
		"https://App.Example:443/cb?x=1": "https://app.example/cb?x=1",
		"https://app.example":            "https://app.example/",
		"http://[::1]":                   "http://[::1]/",
		"http://LocalHost:3000/cb":       "http://localhost:3000/cb",
	} {
		kind := Web
		if !strings.Contains(uri, ":3000") && strings.HasPrefix(uri, "http://") {
			kind = Native
		}
		e, ok := errors.AsType[*Error](Validate(kind, uri))
		require.True(t, ok, uri)
		assert.Equal(t, NotCanonical, e.Reason, uri)
		assert.Equal(t, want, e.Canonical, uri)
		assert.NoError(t, Validate(kind, want), "the canonical form of %s passes", uri)
	}
}

func TestValidateList(t *testing.T) {
	ok := []string{"https://a.example/cb", "https://b.example/cb"}
	assert.NoError(t, ValidateList(Web, ok, true))
	assert.NoError(t, ValidateList(Web, nil, false))
	assert.Equal(t, Required, reasonOf(t, ValidateList(Web, nil, true)))
	assert.Equal(t, TooMany, reasonOf(t, ValidateList(Web, make([]string, MaxURIs+1), true)))
	assert.Equal(t, Duplicate, reasonOf(t, ValidateList(Web, []string{ok[0], ok[1], ok[0]}, true)))
	err := ValidateList(Native, []string{"http://127.0.0.1/cb", "http://127.0.0.1:1/cb"}, true)
	e, isErr := errors.AsType[*Error](err)
	require.True(t, isErr)
	assert.Equal(t, PortIgnored, e.Reason)
	assert.Equal(t, "http://127.0.0.1:1/cb", e.URI)
	assert.Equal(t, 1, e.Index)
	e, _ = errors.AsType[*Error](ValidateList(Web, []string{ok[0], ok[1], ok[0]}, true))
	assert.Equal(t, 2, e.Index, "a duplicate names its second place")
	e, _ = errors.AsType[*Error](ValidateList(Web, nil, true))
	assert.Equal(t, -1, e.Index, "a list-wide error names no row")
	assert.Equal(t, "redirecturi: port_ignored", err.Error())
}

func TestMatch(t *testing.T) {
	for _, tt := range []struct {
		appType               ApplicationType
		registered, requested string
		want                  bool
	}{
		{Web, "https://app.example/cb", "https://app.example/cb", true},
		{Web, "https://app.example/cb", "https://app.example/cb/", false},
		{Web, "https://app.example/cb", "https://APP.example/cb", false},
		{Web, "https://app.example/cb", "https://app.example/cb?x=1", false},
		{Web, "https://app.example/cb", "https://app.example:443/cb", false},
		{Web, "http://127.0.0.1:3000/cb", "http://127.0.0.1:3000/cb", true},
		{Web, "http://127.0.0.1:3000/cb", "http://127.0.0.1:3001/cb", false},
		{Web, "http://127.0.0.1/cb", "http://127.0.0.1:53412/cb", false},
		{Native, "http://127.0.0.1/cb", "http://127.0.0.1:53412/cb", true},
		{Native, "http://127.0.0.1/cb", "http://127.0.0.1/cb", true},
		{Native, "http://[::1]/cb", "http://[::1]:8080/cb", true},
		{Native, "http://localhost/cb", "http://localhost:8080/cb", true},
		{Native, "http://localhost/cb?a=1", "http://localhost:8080/cb?a=1", true},
		{Native, "http://localhost/cb?a=1", "http://localhost:8080/cb?a=2", false},
		{Native, "http://127.0.0.1/", "http://127.0.0.1:8080", false},
		{Native, "http://127.0.0.1/cb", "http://localhost:8080/cb", false},
		{Native, "http://127.0.0.1/cb", "http://127.0.0.1:8080/other", false},
		{Native, "http://127.0.0.1/cb", "https://127.0.0.1:8080/cb", false},
		{Native, "http://127.0.0.1/cb", "HTTP://127.0.0.1:8080/cb", false},
		{Native, "http://127.0.0.1/cb", "http://127.0.0.1:8080/cb?", false},
		{Native, "http://127.0.0.1/cb", "http://127.0.0.1:08080/cb", false},
		{Native, "http://127.0.0.1/cb", "http://127.0.0.1:0/cb", false},
		{Native, "http://127.0.0.1/cb", "http://127.0.0.1:99999/cb", false},
		{Native, "http://127.0.0.1/cb", "http://127.0.0.1:80a/cb", false},
		{Native, "http://127.0.0.1/cb", "http://127.0.0.1:/cb", false},
		{Native, "http://127.0.0.1/cb", "http://u@127.0.0.1:8080/cb", false},
		{Native, "http://127.0.0.1/cb", "http://127.0.0.1:8080/cb#x", false},
		{Native, "http://[::1]/cb", "http://[::1]/cb:8080", false},
		{Native, "http://127.0.0.2/cb", "http://127.0.0.2:8080/cb", false},
		{Native, "https://app.example/cb", "", false},
		{Native, "", "", false},
	} {
		assert.Equal(t, tt.want, Match(tt.appType, tt.registered, tt.requested), "%s: %s vs %s", tt.appType, tt.registered, tt.requested)
	}
}

func TestValidate_RefusesAnUnknownApplicationType(t *testing.T) {
	err := Validate("spa", "https://app.example/cb")
	require.Error(t, err)
	_, isRuleError := errors.AsType[*Error](err)
	assert.False(t, isRuleError, "an unknown type is a programming error, not a rule the URI broke")
}
