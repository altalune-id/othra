package hosted

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/altalune-id/othra/apperror"
	"github.com/altalune-id/othra/internal/enduser"
	"github.com/altalune-id/othra/internal/oauth/oidc"
)

type resolvedEndUser struct {
	Subject, UserSession, SignOut, Link string
	Answered                            bool
	Code                                string
}

func (f *webFixture) withAccountProbe() {
	mux := http.NewServeMux()
	f.web.Register(mux)
	mux.HandleFunc("POST /p/{handle}/api/", func(w http.ResponseWriter, r *http.Request) {
		eu, answered, err := f.web.EndUser(w, r)
		if answered {
			return
		}
		out := resolvedEndUser{Subject: eu.Subject, UserSession: eu.UserSession, SignOut: eu.SignOutToken, Link: eu.LinkToken}
		if ae, ok := apperror.AsAppError(err); ok {
			out.Code = string(ae.Code())
		} else if err != nil {
			out.Code = "error"
		}
		_ = json.NewEncoder(w).Encode(out)
	})
	f.srv.Config.Handler = mux
}

func (f *webFixture) resolve(c *http.Client, handle string) (resolvedEndUser, int) {
	f.t.Helper()
	p := f.do(c, http.MethodPost, "/p/"+handle+"/api/account.v1.AccountService/GetProfile", url.Values{})
	var out resolvedEndUser
	if p.status == http.StatusOK {
		require.NoError(f.t, json.NewDecoder(strings.NewReader(p.body)).Decode(&out))
	}
	return out, p.status
}

// the account plane runs for the browser's active account, with its hosted form tokens; anything else is refused.
func TestEndUser_ResolvesTheActiveAccount(t *testing.T) {
	f := newWebFixture(t)
	f.withAccountProbe()
	got, status := f.resolve(f.client, "acme")
	require.Equal(t, http.StatusOK, status)
	assert.Equal(t, string(apperror.CodeAccountSignedOut), got.Code, "no browser cookie")

	f.signIn(f.client, "acme", "ana@example.com")
	got, _ = f.resolve(f.client, "acme")
	assert.Empty(t, got.Code)
	assert.NotEmpty(t, got.Subject)
	assert.NotEmpty(t, got.UserSession)
	assert.NotEmpty(t, got.SignOut)
	assert.NotEmpty(t, got.Link)
	assert.NotEqual(t, got.SignOut, got.Link, "one token per hosted form")

	f.acme.TermsVersion, f.acme.TermsURL = 1, "https://acme.example/terms"
	got, _ = f.resolve(f.client, "acme")
	assert.Equal(t, string(apperror.CodeAccountTermsRequired), got.Code)
	f.acme.TermsVersion = 0

	got, _ = f.resolve(f.client, "beta")
	assert.Equal(t, string(apperror.CodeAccountSignedOut), got.Code, "the acme cookie never reaches beta")
	_, status = f.resolve(f.client, "nope")
	assert.Equal(t, http.StatusNotFound, status)

	f.users.failOn = "terms"
	got, _ = f.resolve(f.client, "acme")
	assert.Equal(t, "error", got.Code)
	f.users.failOn = "browser"
	got, _ = f.resolve(f.client, "acme")
	assert.NotEmpty(t, got.Code)
	f.users.failOn = ""

	f.issuers.err = &oidc.LookupLimitedError{RetryAfter: 7}
	p := f.do(f.client, http.MethodPost, "/p/acme/api/x", url.Values{})
	assert.Equal(t, http.StatusTooManyRequests, p.status)
	assert.Equal(t, "7", p.header.Get("Retry-After"))
	f.issuers.err = assert.AnError
	got, _ = f.resolve(f.client, "acme")
	assert.Equal(t, "error", got.Code)
	assert.False(t, enduser.IsSignedOutError(nil))
}

// /login?prompt=login back to a portal page asks this account to sign in again instead of showing the picker.
func TestLogin_PortalReauth(t *testing.T) {
	f := newWebFixture(t)
	f.signIn(f.client, "acme", "ana@example.com")
	picker := f.get(f.client, "/p/acme/login?return_to=%2Fp%2Facme%2Faccount%2Ftokens%2Fnew")
	assert.Contains(t, picker.body, `action="/p/acme/login/select"`, "no prompt: the picker")

	reauth := f.get(f.client, "/p/acme/login?prompt=login&return_to=%2Fp%2Facme%2Faccount%2Ftokens%2Fnew")
	require.Equal(t, http.StatusOK, reauth.status, reauth.body)
	assert.Contains(t, reauth.body, "Confirm it&#39;s you")
	assert.Contains(t, reauth.body, `value="ana@example.com"`, "the active account's email")
	assert.NotContains(t, reauth.body, `action="/p/acme/login/select"`)
}
