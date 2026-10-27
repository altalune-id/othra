package views

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/a-h/templ"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"altalune.id/othra/internal/web/layout"
)

func render(t *testing.T, c templ.Component) string {
	t.Helper()
	var b bytes.Buffer
	require.NoError(t, c.Render(context.Background(), &b))
	return b.String()
}

func TestLogin_PasswordOnly(t *testing.T) {
	out := render(t, Login(layout.LayoutData{Title: "Sign in"}, LoginData{ForgotURL: "/forgot-password"}))
	assert.NotContains(t, out, "Continue with Othra", "no button until the console's OIDC side is there")
	assert.NotContains(t, out, "divider")
	assert.Contains(t, out, `<button class="btn mt-2 btn-primary" type="submit">Sign in</button>`)
	assert.Contains(t, out, "Forgot password?")
	assert.Contains(t, out, "required autofocus>", "the email field takes focus when it is the only way in")
}

func TestLogin_ContinueWithOthraLeads(t *testing.T) {
	out := render(t, Login(layout.LayoutData{Title: "Sign in"}, LoginData{OthraURL: "/login/oidc?return_to=%2Fconsole%2F", ForgotURL: "/forgot-password"}))
	button := strings.Index(out, ">Continue with Othra</a>")
	divider := strings.Index(out, "or sign in with a password")
	form := strings.Index(out, `name="password"`)
	require.True(t, button > 0 && divider > button && form > divider, "the button, then the divider, then the password form")
	assert.Contains(t, out, `<a class="btn btn-primary btn-block" href="/login/oidc?return_to=%2Fconsole%2F">`)
	assert.Contains(t, out, `<button class="btn mt-2 btn-outline" type="submit">Sign in</button>`, "one primary button per page")
	assert.NotContains(t, out, "New here?")
	assert.NotContains(t, out, "autofocus", "focus stays off the password form when Continue with Othra leads")
	locked := render(t, Login(layout.LayoutData{Title: "Sign in"}, LoginData{OthraURL: "/login/oidc", Email: "ana@acme.test", EmailLocked: true}))
	assert.NotContains(t, locked, "autofocus")
}

func TestLogin_PasswordOff(t *testing.T) {
	out := render(t, Login(layout.LayoutData{Title: "Sign in"}, LoginData{OthraURL: "/login/oidc", PasswordOff: true, NewHere: true, ForgotURL: "/forgot-password"}))
	assert.Contains(t, out, "Continue with Othra")
	assert.Contains(t, out, "New here? Continue with Othra to create your account.")
	for _, gone := range []string{`name="password"`, "or sign in with a password", "Forgot password?", "<form"} {
		assert.NotContains(t, out, gone)
	}
}

func TestSignInError(t *testing.T) {
	d := layout.LayoutData{Title: "Sign in"}
	for name, tc := range map[string]struct {
		v       SignInErrorData
		title   string
		body    string
		another bool
	}{
		"not invited": {SignInErrorData{Code: "AUT005", Reason: "not_invited", Email: "ana@acme.test"}, "You need an invite", "has no account on this console", true},
		"removed":     {SignInErrorData{Code: "AUT005", Reason: "removed", Email: "ana@acme.test"}, "You can&#39;t sign in to this console", "Ask an admin to check your account.", false},
		"unverified":  {SignInErrorData{Code: "AUT005", Reason: "email_unverified", Email: "ana@acme.test"}, "Verify your Othra email", "isn&#39;t verified in Othra yet", true},
		"expired":     {SignInErrorData{Code: "AUT006"}, "This sign-in expired", "Make sure this site may use cookies.", false},
		"conflict":    {SignInErrorData{Code: "AUT007", Email: "ana@acme.test"}, "This account uses another Othra sign-in", "is linked to a different Othra identity", true},
	} {
		tc.v.AnotherURL, tc.v.BackURL = "/login/oidc?prompt=select_account", "/login"
		out := render(t, SignInError(d, tc.v))
		assert.Contains(t, out, tc.title, name)
		assert.Contains(t, out, tc.body, name)
		assert.Contains(t, out, `role="alert"`, name)
		assert.Contains(t, out, `href="/login">Back to sign in</a>`, name)
		assert.Equal(t, tc.another, strings.Contains(out, "Sign in with another account"), name)
		assert.Contains(t, out, "Error code: "+tc.v.Code, "%s: the labelled code, for support", name)
		if tc.v.Email != "" && tc.v.Reason != "removed" {
			assert.Contains(t, out, "ana@acme.test", name)
		}
	}
	out := render(t, SignInError(d, SignInErrorData{Code: "AUT005", Email: `<script>x</script>`, BackURL: "/login"}))
	assert.NotContains(t, out, "<script>x", "the email is escaped")
	assert.NotContains(t, out, "Sign in with another account", "no picker link without its URL")

	out = render(t, SignInError(d, SignInErrorData{Code: "AUT007", Email: "ana@acme.test", BackURL: "/login"}))
	assert.Contains(t, out, "or with your password")
	out = render(t, SignInError(d, SignInErrorData{Code: "AUT007", Email: "ana@acme.test", BackURL: "/login", PasswordOff: true}))
	assert.Contains(t, out, "Sign in with that one.")
	assert.NotContains(t, out, "password", "no password to offer when passwords are off")
}
