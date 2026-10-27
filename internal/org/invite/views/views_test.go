package views

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/altalune-id/othra/internal/web/layout"
)

func TestLanding_OnePrimaryButton(t *testing.T) {
	render := func(v LandingData) string {
		var b bytes.Buffer
		require.NoError(t, Landing(layout.LayoutData{Title: "Join Acme"}, v).Render(context.Background(), &b))
		return b.String()
	}
	v := LandingData{State: StateJoin, Org: "Acme", Inviter: "Ana", Role: "member", Email: "bo@acme.test", Password: true}

	out := render(v)
	assert.Contains(t, out, `class="btn mt-2 btn-primary" type="submit">Join Acme`)
	assert.NotContains(t, out, "Continue with Othra")

	v.OthraURL = "/login/oidc?invite=x"
	out = render(v)
	assert.Contains(t, out, `class="btn btn-primary btn-block"`, "Continue with Othra leads")
	assert.Contains(t, out, `class="btn mt-2 btn-outline" type="submit">Join Acme`)
	assert.Contains(t, out, "or join with a password")
	assert.NotContains(t, out, "Ask your admin")

	v.OthraURL, v.Password = "", false
	out = render(v)
	assert.Contains(t, out, "Ask your admin to turn on sign-in for this console.", "the join page is never without a way forward")
	assert.NotContains(t, out, "<form")
}
