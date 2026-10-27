//go:build integration

package boot

import (
	"net/http"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"altalune.id/othra/config"
	orgv1 "altalune.id/othra/gen/go/org/v1"
	"altalune.id/othra/internal/auth"
	"altalune.id/othra/internal/install"
	"altalune.id/othra/internal/kernel/session"
	"altalune.id/othra/internal/kernel/tenant"
	"altalune.id/othra/internal/org"
	"altalune.id/othra/internal/org/member"
)

// The console signs in through /p/system over the in-process transport: the owner's verified email links once, then the sub does.
func TestConsoleSignIn_ThroughSystem(t *testing.T) {
	f := newSignInFixture(t)
	browser := f.signInDevice(t, "/p/system", "root@example.com")

	start := f.do(browser, http.MethodGet, "/login/oidc?return_to=/console/", nil)
	require.Equal(t, http.StatusFound, start.status, start.body)
	authorize := start.header.Get("Location")
	require.True(t, strings.HasPrefix(authorize, f.srv.URL+"/p/system/oauth2/authorize?"), authorize)

	granted := f.do(browser, http.MethodGet, strings.TrimPrefix(authorize, f.srv.URL), nil)
	callback := granted.header.Get("Location")
	require.True(t, strings.HasPrefix(callback, f.srv.URL+"/oauth/callback?"), "%d %s", granted.status, granted.body)

	done := f.do(browser, http.MethodGet, strings.TrimPrefix(callback, f.srv.URL), nil)
	require.Equal(t, http.StatusSeeOther, done.status, done.body)
	assert.Equal(t, "/console/", done.header.Get("Location"))

	root, err := f.svcs.Users.ByEmail(t.Context(), "root@example.com")
	require.NoError(t, err)
	assert.Equal(t, auth.ConsoleIssuer, root.IDPIssuer, "linked under the fixed issuer, so a public-URL change keeps it")
	assert.NotEmpty(t, root.IDPSubject)

	replay := f.do(browser, http.MethodGet, strings.TrimPrefix(callback, f.srv.URL), nil)
	assert.Equal(t, http.StatusBadRequest, replay.status, "the state cookie is single use")

	cb, err := url.Parse(callback)
	require.NoError(t, err)
	for name, code := range map[string]string{"replayed": cb.Query().Get("code"), "unknown": "not-a-code"} {
		again := f.do(browser, http.MethodGet, "/login/oidc?return_to=/console/", nil)
		require.Equal(t, http.StatusFound, again.status, again.body)
		authorize, err := url.Parse(again.header.Get("Location"))
		require.NoError(t, err)
		q := url.Values{"code": {code}, "state": {authorize.Query().Get("state")}, "iss": {cb.Query().Get("iss")}}
		refused := f.do(browser, http.MethodGet, "/oauth/callback?"+q.Encode(), nil)
		assert.Equal(t, http.StatusBadRequest, refused.status, "%s code: %s", name, refused.body)
		assert.Contains(t, refused.body, "This sign-in expired", name)
	}
}

// NOTE: the console's "Continue with Othra" on browser, through /p/system, back to /console/.
func consoleSignIn(t *testing.T, f *signInFixture, browser *http.Client) {
	t.Helper()
	start := f.do(browser, http.MethodGet, "/login/oidc?return_to=/console/", nil)
	require.Equal(t, http.StatusFound, start.status, start.body)
	granted := f.do(browser, http.MethodGet, strings.TrimPrefix(start.header.Get("Location"), f.srv.URL), nil)
	callback := granted.header.Get("Location")
	require.True(t, strings.HasPrefix(callback, f.srv.URL+"/oauth/callback?"), "%d %s", granted.status, granted.body)
	done := f.do(browser, http.MethodGet, strings.TrimPrefix(callback, f.srv.URL), nil)
	require.Equal(t, http.StatusSeeOther, done.status, done.body)
}

// SECURITY: console sign-out ends the /p/system session in process, so the next Continue with Othra asks for the email again.
func TestConsoleSignOut_EndsTheSystemSession(t *testing.T) {
	f := newSignInFixture(t, func(c *config.Config) { c.Auth.SessionTTL = time.Hour })
	browser := f.signInDevice(t, "/p/system", "root@example.com")
	consoleSignIn(t, f, browser)

	out := f.do(browser, http.MethodPost, "/logout", url.Values{})
	require.Equal(t, http.StatusSeeOther, out.status, out.body)
	assert.True(t, strings.HasPrefix(out.header.Get("Location"), f.srv.URL+"/p/system/oauth2/end-session?"), out.header.Get("Location"))

	start := f.do(browser, http.MethodGet, "/login/oidc?return_to=/console/", nil)
	require.Equal(t, http.StatusFound, start.status, start.body)
	next := strings.TrimPrefix(start.header.Get("Location"), f.srv.URL)
	var page signInPage
	for range 5 {
		page = f.do(browser, http.MethodGet, next, nil)
		loc := page.header.Get("Location")
		require.False(t, strings.HasPrefix(loc, f.srv.URL+"/oauth/callback"), "signed straight back in: %s", loc)
		if page.status != http.StatusFound && page.status != http.StatusSeeOther {
			break
		}
		next = strings.TrimPrefix(loc, f.srv.URL)
	}
	require.Equal(t, http.StatusOK, page.status, page.body)
	assert.Contains(t, page.body, `name="email"`, "the email is asked again")
	assert.NotContains(t, page.body, "Choose an account", "no picker shortcut back into the account")
}

func cloudServices(t *testing.T) (*Services, config.Config) {
	t.Helper()
	cfg := devLocalConfig()
	cfg.Mode = config.ModeCloud
	cfg.HTTP.BaseURL = "https://console.example.com"
	return migratedServices(t, cfg)
}

// SECURITY: cloud signup's one org per person holds under concurrency: the creator's row lock lets one creation through.
func TestCreateOwnOrg_OneOrgPerPerson(t *testing.T) {
	svcs, _ := cloudServices(t)
	ctx := t.Context()
	ana, err := svcs.Users.CreateLinked(ctx, "ana@shop.test", "Ana", auth.ConsoleIssuer, "sub-ana")
	require.NoError(t, err)

	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i := range errs {
		wg.Go(func() { _, errs[i] = svcs.Auth.CreateOwnOrg(ctx, ana.ID, "Ana Org", "") })
	}
	wg.Wait()
	wins := 0
	for _, err := range errs {
		if err == nil {
			wins++
			continue
		}
		assert.True(t, org.IsAlreadyOwnedError(err), "%v", err)
	}
	assert.Equal(t, 1, wins)

	ms, err := svcs.Members.ListByUser(ctx, ana.ID)
	require.NoError(t, err)
	require.Len(t, ms, 1)
	assert.Equal(t, member.RoleOwner, ms[0].Role, "the creator owns it, in the org's own unit of work")
	projects, err := svcs.Projects.List(tenant.Into(ctx, tenant.Context{OrgID: ms[0].OrgID, UserID: ana.ID}), ms[0].OrgID)
	require.NoError(t, err)
	assert.Len(t, projects, 1, "with its default project")

	_, err = svcs.Auth.CreateOwnOrg(ctx, ana.ID, "Second", "")
	assert.True(t, org.IsAlreadyOwnedError(err))
}

// SECURITY: a cloud boot puts the console client's URIs back to the ones http.baseURL derives, like selfhosted.
func TestFirstBoot_CloudReconcilesTheConsoleClient(t *testing.T) {
	svcs, cfg := cloudServices(t)
	ctx := t.Context()
	_, err := svcs.Install.Install(ctx, install.Request{Email: "op@altalune.test"})
	require.NoError(t, err)
	require.NoError(t, firstBoot(ctx, cfg, svcs, testLogger(), true))
	sys, p, err := consoleProject(ctx, svcs)
	require.NoError(t, err)
	require.NotNil(t, p)
	redirects := func() []string {
		c, err := svcs.Clients.System(ctx, sys.ID, p.ID)
		require.NoError(t, err)
		return c.RedirectURIs
	}

	cli := cfg
	cli.HTTP.BaseURL = "http://localhost:5150"
	require.NoError(t, firstBoot(ctx, cli, svcs, testLogger(), false))
	assert.Equal(t, []string{"https://console.example.com/oauth/callback"}, redirects(), "an operator command never rewrites the console client")

	cfg.HTTP.BaseURL = "https://moved.example.com"
	require.NoError(t, firstBoot(ctx, cfg, svcs, testLogger(), true))
	assert.Equal(t, []string{"https://moved.example.com/oauth/callback"}, redirects())
}

// SECURITY: a verified email alone claims only the email-only owner seat; an invite someone made, or for another role, needs its link.
func TestConsoleSignIn_OnlyTheEmailOnlyOwnerSeat(t *testing.T) {
	svcs, _ := migratedServices(t, devLocalConfig())
	ctx := t.Context()
	root, err := svcs.Install.Install(ctx, install.Request{Name: "Root", Email: "root@example.com", Password: "quiet-forest-lamp-9"})
	require.NoError(t, err)
	sys, err := svcs.Orgs.System(ctx)
	require.NoError(t, err)
	seats := consoleInvites{invites: svcs.Invites}

	_, err = svcs.Invites.Create(ctx, member.Requester{UserID: root.ID}, sys.ID, "co@example.com", member.RoleOwner)
	require.NoError(t, err)
	_, err = svcs.Invites.Create(ctx, member.Requester{Operator: true}, sys.ID, "mem@example.com", member.RoleMember)
	require.NoError(t, err)
	_, err = svcs.Invites.InviteOwner(ctx, sys.ID, "seat@example.com")
	require.NoError(t, err)

	for _, email := range []string{"co@example.com", "mem@example.com"} {
		seat, err := seats.OwnerSeat(ctx, sys.ID, email)
		require.NoError(t, err)
		assert.Nil(t, seat, email)

		_, err = svcs.Auth.LoginOIDC(ctx, auth.OIDCSignIn{Subject: "sub-" + email, Email: email, EmailVerified: true, Name: "X"})
		var none *auth.NoConsoleAccountError
		require.ErrorAs(t, err, &none, email)
		assert.Equal(t, auth.NoConsoleNotInvited, none.Reason, email)
	}

	seat, err := seats.OwnerSeat(ctx, sys.ID, "seat@example.com")
	require.NoError(t, err)
	require.NotNil(t, seat)
	p, err := svcs.Auth.LoginOIDC(ctx, auth.OIDCSignIn{Subject: "sub-seat", Email: "seat@example.com", EmailVerified: true, Name: "Seat"})
	require.NoError(t, err)
	m, err := svcs.Members.Find(ctx, sys.ID, p.UserID)
	require.NoError(t, err)
	assert.Equal(t, member.RoleOwner, m.Role)
}

// A passwords-off install with no mail has only the email-only owner seat; the operator's printed link is how its owner gets in.
func TestSignInLink_RescuesTheOwnerSeat(t *testing.T) {
	svcs, _ := cloudServices(t)
	ctx := t.Context()
	_, err := svcs.Install.Install(ctx, install.Request{Email: "op@altalune.test"})
	require.NoError(t, err)
	app := &App{Services: svcs, log: testLogger(), consoleSignIn: "/login/oidc"}

	l, err := app.SignInLink(ctx, "Op@Altalune.test")
	require.NoError(t, err)
	assert.Equal(t, "op@altalune.test", l.Email)
	assert.True(t, strings.HasPrefix(l.URL, "https://console.example.com/p/system/login/verify?t="), l.URL)

	_, err = app.SignInLink(ctx, "stranger@altalune.test")
	assert.Error(t, err, "no account and no seat")
}

// SECURITY: a cloud person's CreateOrg makes them the owner, never an orphan org, and a second one is ORG006.
func TestCreateOrg_ThePersonOwnsIt(t *testing.T) {
	svcs, _ := cloudServices(t)
	ctx := t.Context()
	ana, err := svcs.Users.CreateLinked(ctx, "ana@shop.test", "Ana", auth.ConsoleIssuer, "sub-ana")
	require.NoError(t, err)
	as := session.PrincipalInto(ctx, session.Principal{Source: session.SourceOIDC, UserID: ana.ID, Email: ana.Email})
	plane := org.NewControlPlane(svcs.Orgs)

	resp, err := plane.CreateOrg(as, connect.NewRequest(&orgv1.CreateOrgRequest{Name: "Ana Shop"}))
	require.NoError(t, err)
	o, err := svcs.Orgs.ByPublicID(ctx, resp.Msg.GetOrg().GetId())
	require.NoError(t, err)
	m, err := svcs.Members.Find(ctx, o.ID, ana.ID)
	require.NoError(t, err)
	assert.Equal(t, member.RoleOwner, m.Role)

	_, err = plane.CreateOrg(as, connect.NewRequest(&orgv1.CreateOrgRequest{Name: "Second"}))
	assert.True(t, org.IsAlreadyOwnedError(err), "%v", err)
}
