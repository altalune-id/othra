package hosted

import (
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"altalune.id/othra/internal/project"
)

const testIssuer = "https://id.example.com/auth/p/acme"

func TestReturnTo(t *testing.T) {
	const account = "/auth/p/acme/account/"
	for raw, want := range map[string]string{
		"":                              account,
		"/auth/p/acme/account/":         account,
		"/auth/p/acme/account/sessions": "/auth/p/acme/account/sessions",
		"/auth/p/acme/oauth2/authorize?client_id=abc&state=x%20y":                    "/auth/p/acme/oauth2/authorize?client_id=abc&state=x+y",
		"/auth/p/acme/oauth2/authorize?state=" + strings.Repeat("s", maxReturnToLen): account,
		"/auth/p/acme/oauth2/authorize?redirect_uri=https%3A%2F%2Fapp%2Fcb":          "/auth/p/acme/oauth2/authorize?redirect_uri=https%3A%2F%2Fapp%2Fcb",
		"/auth/p/acme/account/#frag":                                                 account,
		"https://id.example.com/auth/p/acme/account/":                                account,
		"https://evil.example/auth/p/acme/account/x":                                 account,
		"//evil.example/auth/p/acme/account/x":                                       account,
		"/\\evil.example/auth/p/acme/account/x":                                      account,
		"\\\\evil.example":                                                           account,
		"/auth/p/acme/account/\\..\\..":                                              account,
		"https://user:pw@id.example.com/auth/p/acme/account/":                        account,
		"javascript:alert(1)":                                                        account,
		"/auth/p/acme/account/../../other/account/":                                  account,
		"/auth/p/acme/account/./x":                                                   account,
		"/auth/p/acme/account/%2e%2e/%2e%2e/other/account/":                          account,
		"/auth/p/acme/account%2F..%2F..%2Fother":                                     account,
		"/auth/p/acme/account/%5c..":                                                 account,
		"/auth/p/acme/account/\x00":                                                  account,
		"/auth/p/acme/account/\nSet-Cookie: x":                                       account,
		"/auth/p/acme/console/":                                                      account,
		"/auth/p/acmex/account/":                                                     account,
		"/auth/p/other/account/":                                                     account,
		"/auth/p/acme/oauth2/token":                                                  account,
		"/auth/p/acme/oauth2/authorize/../token":                                     account,
		"/auth/p/acme/accounts/":                                                     account,
		"/p/acme/account/":                                                           account,
	} {
		assert.Equal(t, want, ReturnTo(testIssuer, raw), "%q", raw)
	}
	assert.Equal(t, "/p/acme/account/", ReturnTo("https://id.example.com/p/acme/", ""), "no base path")
	assert.Equal(t, "/account/", ReturnTo("::", "/x"), "an unparsable issuer still falls back")
}

// RFC 9700 §4.11: whatever comes in, the result stays under this issuer, is a plain path, and is stable.
func FuzzReturnTo(f *testing.F) {
	for _, seed := range []string{
		"/auth/p/acme/account/", "/auth/p/acme/oauth2/authorize?a=b", "//evil", "/\\evil", "https://evil/x",
		"/auth/p/acme/account/%2e%2e/x", "/auth/p/acme/account/..%2f", "\t/auth", "/auth/p/acme/account/?x=%00",
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, raw string) {
		got := ReturnTo(testIssuer, raw)
		u, err := url.Parse(got)
		if err != nil {
			t.Fatalf("%q gave an unparsable %q", raw, got)
		}
		if u.Scheme != "" || u.Host != "" || u.User != nil || u.Fragment != "" || strings.HasPrefix(got, "//") {
			t.Fatalf("%q gave %q, which leaves the origin", raw, got)
		}
		if u.Path != "/auth/p/acme/oauth2/authorize" && !strings.HasPrefix(u.Path, "/auth/p/acme/account/") {
			t.Fatalf("%q gave %q, outside this issuer", raw, got)
		}
		if dotSegment(u.Path) || strings.ContainsAny(got, "\\\x00\r\n") {
			t.Fatalf("%q gave %q", raw, got)
		}
		if again := ReturnTo(testIssuer, got); again != got {
			t.Fatalf("%q gave %q, then %q", raw, got, again)
		}
	})
}

// SECURITY: /p/system may hand off to the console's sign-in start, and nothing else; no other project may.
func TestConsoleHandOff(t *testing.T) {
	h := &Web{cfg: Config{BasePath: "/auth"}}
	system := &site{issuer: &project.Issuer{Handle: project.SystemHandle}, issuerURL: "https://id.example.com/auth/p/system"}
	acme := &site{issuer: &project.Issuer{Handle: "acme"}, issuerURL: "https://id.example.com/auth/p/acme"}
	assert.Equal(t, "/auth/login/oidc", h.returnTo(system, "/auth/login/oidc"))
	assert.Equal(t, "/auth/p/system/account/", h.returnTo(system, "/auth/login/oidc?return_to=https://evil.test"), "exact path only")
	assert.Equal(t, "/auth/p/system/account/", h.returnTo(system, "https://evil.test/auth/login/oidc"))
	assert.Equal(t, "/auth/p/acme/account/", h.returnTo(acme, "/auth/login/oidc"))
}
