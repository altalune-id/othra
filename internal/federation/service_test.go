package federation

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"altalune.id/othra/apperror"
	"altalune.id/othra/internal/testutil/fakes"
	"altalune.id/othra/internal/upstream"
	"altalune.id/othra/oidcrp"
	"altalune.id/othra/oidcrp/oidcrptest"
)

const redirectURI = "https://othra.example/p/acme/federation/google/callback"

type harness struct {
	svc       *Service
	store     *fakeStore
	upstreams *fakeUpstreams
	users     *fakeUsers
	switches  *fakeSwitch
	idp       *oidcrptest.IDP
	p         Project
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	idp := oidcrptest.New(t)
	h := &harness{
		store: newFakeStore(), idp: idp, users: &fakeUsers{byEmail: map[string]uuid.UUID{}}, switches: &fakeSwitch{on: true},
		p: Project{OrgID: uuid.New(), ID: uuid.New(), PublicID: "prj1"},
		upstreams: &fakeUpstreams{creds: &upstream.Credentials{
			Upstream:     upstream.Upstream{ID: uuid.New(), Kind: upstream.KindGoogle, ClientID: "cid.apps", Enabled: true},
			ClientSecret: "s3cret",
		}},
	}
	h.svc = NewService(h.store, fakes.Transactor{}, h.upstreams, h.users, h.switches, Config{
		GoogleIssuerOverride: idp.Issuer(), HTTPClient: idp.Client(),
	}, apperror.NewReporter(nil, false).Unexpected)
	return h
}

func (h *harness) roundTrip(t *testing.T, hint string) (Identity, error) {
	t.Helper()
	ctx := context.Background()
	req, err := h.svc.Start(ctx, h.p, upstream.KindGoogle, hint)
	require.NoError(t, err)
	authURL, err := h.svc.AuthURL(ctx, h.p, upstream.KindGoogle, redirectURI, req)
	require.NoError(t, err)
	return h.svc.Complete(ctx, h.p, upstream.KindGoogle, redirectURI, oidcrp.ParseCallback(h.idp.Authorize(authURL)), req)
}

func codeOf(err error) apperror.Code {
	if app, ok := apperror.AsAppError(err); ok {
		return app.Code()
	}
	return ""
}

func TestEnabled(t *testing.T) {
	h := newHarness(t)
	on, err := h.svc.Enabled(context.Background(), h.p, upstream.KindGoogle)
	require.NoError(t, err)
	assert.True(t, on)

	h.upstreams.creds.Upstream.Enabled = false
	on, err = h.svc.Enabled(context.Background(), h.p, upstream.KindGoogle)
	require.NoError(t, err)
	assert.False(t, on)

	h.upstreams.err = errBoom
	_, err = h.svc.Enabled(context.Background(), h.p, upstream.KindGoogle)
	require.Error(t, err)
}

// every start asks Google to choose the account, and only the given linked email goes as login_hint.
func TestStart(t *testing.T) {
	h := newHarness(t)
	req, err := h.svc.Start(context.Background(), h.p, upstream.KindGoogle, "")
	require.NoError(t, err)
	assert.Equal(t, "select_account", req.Prompt, "a plain start too")
	assert.Empty(t, req.LoginHint)
	assert.NotEmpty(t, req.State)

	req, err = h.svc.Start(context.Background(), h.p, upstream.KindGoogle, "ana.g@gmail.com")
	require.NoError(t, err)
	raw, err := h.svc.AuthURL(context.Background(), h.p, upstream.KindGoogle, redirectURI, req)
	require.NoError(t, err)
	u, err := url.Parse(raw)
	require.NoError(t, err)
	assert.Equal(t, "select_account", u.Query().Get("prompt"))
	assert.Equal(t, "ana.g@gmail.com", u.Query().Get("login_hint"))

	h.upstreams.creds.Upstream.Enabled = false
	_, err = h.svc.Start(context.Background(), h.p, upstream.KindGoogle, "")
	assert.Equal(t, apperror.CodeSignInUpstreamDisabled, codeOf(err))
}

func TestAuthURL(t *testing.T) {
	h := newHarness(t)
	req, err := h.svc.Start(context.Background(), h.p, upstream.KindGoogle, "")
	require.NoError(t, err)
	raw, err := h.svc.AuthURL(context.Background(), h.p, upstream.KindGoogle, redirectURI, req)
	require.NoError(t, err)
	u, err := url.Parse(raw)
	require.NoError(t, err)
	q := u.Query()
	assert.Equal(t, "cid.apps", q.Get("client_id"))
	assert.Equal(t, redirectURI, q.Get("redirect_uri"))
	assert.Equal(t, "openid email profile", q.Get("scope"))
	assert.Equal(t, "select_account", q.Get("prompt"))

	again, _, err := h.svc.client(context.Background(), h.p, upstream.KindGoogle, redirectURI)
	require.NoError(t, err)
	first, _, err := h.svc.client(context.Background(), h.p, upstream.KindGoogle, redirectURI)
	require.NoError(t, err)
	assert.Same(t, first, again, "one oidcrp client per upstream configuration, so discovery and JWKS stay cached")

	h.upstreams.creds.ClientSecret = "rotated"
	rotated, _, err := h.svc.client(context.Background(), h.p, upstream.KindGoogle, redirectURI)
	require.NoError(t, err)
	assert.NotSame(t, first, rotated, "a new secret builds a new client")
	assert.Len(t, h.svc.clients, 1, "the client with the old secret is evicted")

	h.upstreams.creds.Upstream.ClientID = "cid2.apps"
	renamed, _, err := h.svc.client(context.Background(), h.p, upstream.KindGoogle, redirectURI)
	require.NoError(t, err)
	assert.NotSame(t, rotated, renamed, "a new client id builds a new client")
	assert.Len(t, h.svc.clients, 1)
}

func TestAuthURL_UpstreamDown(t *testing.T) {
	h := newHarness(t)
	h.svc.cfg.GoogleIssuerOverride = "https://127.0.0.1:1"
	req, err := h.svc.Start(context.Background(), h.p, upstream.KindGoogle, "")
	require.NoError(t, err)
	_, err = h.svc.AuthURL(context.Background(), h.p, upstream.KindGoogle, redirectURI, req)
	assert.Equal(t, apperror.CodeSignInUpstreamUnavailable, codeOf(err))
}

func TestComplete(t *testing.T) {
	h := newHarness(t)
	h.idp.User = oidcrptest.User{Subject: "sub-1", Email: " Ana@Acme.test ", EmailVerified: true, Name: "  Ana  "}
	h.idp.Claims = map[string]any{"hd": "acme.test"}
	id, err := h.roundTrip(t, "")
	require.NoError(t, err)
	assert.Equal(t, Identity{
		UpstreamID: h.upstreams.creds.Upstream.ID, Issuer: h.idp.Issuer(), Subject: "sub-1", Email: "ana@acme.test", EmailVerified: true,
		HostedDomain: "acme.test", Name: "Ana", Kind: upstream.KindGoogle,
	}, id, "hd rides along to decide whether Google is authoritative for the email")
}

func TestComplete_Refused(t *testing.T) {
	ctx := context.Background()
	t.Run("cancelled", func(t *testing.T) {
		h := newHarness(t)
		req, err := h.svc.Start(ctx, h.p, upstream.KindGoogle, "")
		require.NoError(t, err)
		authURL, err := h.svc.AuthURL(ctx, h.p, upstream.KindGoogle, redirectURI, req)
		require.NoError(t, err)
		_, err = h.svc.Complete(ctx, h.p, upstream.KindGoogle, redirectURI, oidcrp.ParseCallback(h.idp.Deny(authURL)), req)
		assert.Equal(t, apperror.CodeSignInUpstreamCancelled, codeOf(err))
	})
	t.Run("bad nonce", func(t *testing.T) {
		h := newHarness(t)
		h.idp.Claims = map[string]any{"nonce": "another"}
		_, err := h.roundTrip(t, "")
		assert.Equal(t, apperror.CodeSignInUpstreamResponseInvalid, codeOf(err))
	})
	t.Run("token endpoint down", func(t *testing.T) {
		h := newHarness(t)
		h.idp.TokenStatus = 503
		_, err := h.roundTrip(t, "")
		assert.Equal(t, apperror.CodeSignInUpstreamUnavailable, codeOf(err))
	})
	t.Run("disabled meanwhile", func(t *testing.T) {
		h := newHarness(t)
		req, err := h.svc.Start(ctx, h.p, upstream.KindGoogle, "")
		require.NoError(t, err)
		h.upstreams.creds.Upstream.Enabled = false
		_, err = h.svc.Complete(ctx, h.p, upstream.KindGoogle, redirectURI, oidcrp.Callback{}, req)
		assert.Equal(t, apperror.CodeSignInUpstreamDisabled, codeOf(err))
	})
	t.Run("bad email", func(t *testing.T) {
		h := newHarness(t)
		h.idp.User.Email = "not an email"
		id, err := h.roundTrip(t, "")
		require.NoError(t, err)
		assert.Empty(t, id.Email)
		assert.False(t, id.EmailVerified, "no usable email is never a verified one")
	})
}

func identity(h *harness) Identity {
	return Identity{
		UpstreamID: h.upstreams.creds.Upstream.ID, Issuer: "https://accounts.google.com", Subject: "sub-1", Email: "ana@gmail.com",
		EmailVerified: true, Name: "Ana", Kind: upstream.KindGoogle,
	}
}

func TestSignIn_NewEndUser(t *testing.T) {
	h := newHarness(t)
	got, err := h.svc.SignIn(context.Background(), h.p, identity(h), SessionRequest{BrowserSecret: "old", UserAgent: "ua"})
	require.NoError(t, err)
	assert.Equal(t, "secret", got.BrowserSecret)
	require.Len(t, h.store.rows, 1)
	assert.Equal(t, h.users.byEmail["ana@gmail.com"], h.store.rows[0].EndUserID)
	assert.Len(t, h.store.rows[0].PublicID, publicIDLen)
	assert.Equal(t, []SessionRequest{{EndUserID: got.EndUserID, Method: "google", BrowserSecret: "old", UserAgent: "ua"}}, h.users.signedIn)
}

func TestSignIn_KnownIdentity(t *testing.T) {
	h := newHarness(t)
	id := identity(h)
	_, err := h.svc.SignIn(context.Background(), h.p, id, SessionRequest{})
	require.NoError(t, err)
	id.Email = "ana.new@gmail.com"
	got, err := h.svc.SignIn(context.Background(), h.p, id, SessionRequest{})
	require.NoError(t, err, "a known (iss, sub) signs in whatever its email is now")
	assert.Equal(t, h.store.rows[0].EndUserID, got.EndUserID)
	assert.Equal(t, "ana.new@gmail.com", h.store.rows[0].Email, "the stored email follows the upstream")
	assert.Len(t, h.store.rows, 1)
}

func TestSignIn_Collision(t *testing.T) {
	h := newHarness(t)
	h.users.byEmail["ana@gmail.com"] = uuid.New()
	_, err := h.svc.SignIn(context.Background(), h.p, identity(h), SessionRequest{})
	cerr, ok := errors.AsType[*CollisionError](err)
	require.True(t, ok, "never auto-link by email: %v", err)
	assert.Equal(t, "ana@gmail.com", cerr.Email)
	assert.Equal(t, "sub-1", cerr.Identity.Subject)
	assert.Empty(t, h.store.rows)
	assert.Empty(t, h.users.signedIn)
	assert.Equal(t, apperror.CodeSignInUpstreamEmailTaken, codeOf(err))
}

func TestSignIn_Races(t *testing.T) {
	t.Run("the email was just taken", func(t *testing.T) {
		h := newHarness(t)
		h.users.createErr = ErrEmailTaken
		_, err := h.svc.SignIn(context.Background(), h.p, identity(h), SessionRequest{})
		assert.True(t, IsCollisionError(err))
	})
	t.Run("the subject was just linked, then found", func(t *testing.T) {
		h := newHarness(t)
		winner := uuid.New()
		h.store.onInsert = func() {
			delete(h.users.byEmail, "ana@gmail.com")
			require.NoError(t, h.store.Insert(context.Background(), h.p.OrgID, h.p.ID, winner, identity(h), "idn-winner", time.Now()))
		}
		got, err := h.svc.SignIn(context.Background(), h.p, identity(h), SessionRequest{})
		require.NoError(t, err, "the retry finds the row the other request inserted")
		assert.Equal(t, winner, got.EndUserID)
	})
	t.Run("the same new account won the email first", func(t *testing.T) {
		h := newHarness(t)
		winner := uuid.New()
		h.users.onCreate = func() error {
			require.NoError(t, h.store.Insert(context.Background(), h.p.OrgID, h.p.ID, winner, identity(h), "idn-winner", time.Now()))
			return ErrEmailTaken
		}
		got, err := h.svc.SignIn(context.Background(), h.p, identity(h), SessionRequest{})
		require.NoError(t, err, "the loser re-reads by (iss, sub) and signs in, never the collision page")
		assert.Equal(t, winner, got.EndUserID)
	})
	t.Run("the same new account committed between the two lookups", func(t *testing.T) {
		h := newHarness(t)
		winner := uuid.New()
		h.users.onLookup = func() {
			h.users.byEmail["ana@gmail.com"] = winner
			require.NoError(t, h.store.Insert(context.Background(), h.p.OrgID, h.p.ID, winner, identity(h), "idn-winner", time.Now()))
		}
		got, err := h.svc.SignIn(context.Background(), h.p, identity(h), SessionRequest{})
		require.NoError(t, err, "the email hit re-reads (iss, sub) and finds the winner")
		assert.Equal(t, winner, got.EndUserID)
		assert.Zero(t, h.users.created)
	})
	t.Run("the subject stays taken", func(t *testing.T) {
		h := newHarness(t)
		h.store.insertErr = []error{errSubjectTaken, errSubjectTaken}
		_, err := h.svc.SignIn(context.Background(), h.p, identity(h), SessionRequest{})
		assert.True(t, IsCollisionError(err))
	})
}

func TestSignIn_Refused(t *testing.T) {
	h := newHarness(t)
	id := identity(h)
	id.EmailVerified = false
	_, err := h.svc.SignIn(context.Background(), h.p, id, SessionRequest{})
	assert.Equal(t, apperror.CodeSignInUpstreamEmailUnverified, codeOf(err))

	id = identity(h)
	id.Email = ""
	_, err = h.svc.SignIn(context.Background(), h.p, id, SessionRequest{})
	assert.Equal(t, apperror.CodeSignInUpstreamEmailUnverified, codeOf(err))

	h.store.err = errBoom
	_, err = h.svc.SignIn(context.Background(), h.p, identity(h), SessionRequest{})
	assert.Equal(t, apperror.CodeUnexpected, codeOf(err))

	h.store.err = nil
	h.users.signInErr = &apperror.AppError{}
	_, err = h.svc.SignIn(context.Background(), h.p, identity(h), SessionRequest{})
	require.Error(t, err)
}

func TestLink(t *testing.T) {
	ctx := context.Background()
	h := newHarness(t)
	ana, budi := uuid.New(), uuid.New()
	require.NoError(t, h.svc.Link(ctx, h.p, ana, identity(h)))
	require.NoError(t, h.svc.Link(ctx, h.p, ana, identity(h)), "linking again to the same end user is a no-op")
	assert.Len(t, h.store.rows, 1)

	err := h.svc.Link(ctx, h.p, budi, identity(h))
	assert.Equal(t, apperror.CodeSignInUpstreamLinkedElsewhere, codeOf(err))

	other := identity(h)
	other.Subject = "sub-2"
	err = h.svc.Link(ctx, h.p, ana, other)
	assert.Equal(t, apperror.CodeSignInUpstreamLinkedElsewhere, codeOf(err), "one identity per upstream per end user")

	h.store.err = errBoom
	err = h.svc.Link(ctx, h.p, ana, identity(h))
	assert.Equal(t, apperror.CodeUnexpected, codeOf(err))
}

func TestUnlink(t *testing.T) {
	ctx := context.Background()
	h := newHarness(t)
	ana := uuid.New()
	require.NoError(t, h.svc.Link(ctx, h.p, ana, identity(h)))
	pub := h.store.rows[0].PublicID

	h.switches.on = false
	err := h.svc.Unlink(ctx, h.p, ana, pub)
	assert.Equal(t, apperror.CodeSignInLastMethod, codeOf(err), "SIG019: no email link and no other identity")
	assert.Equal(t, 1, h.switches.locked, "the switch is read under the project lock")

	h.switches.on = true
	err = h.svc.Unlink(ctx, h.p, ana, "nope")
	assert.IsType(t, &IdentityNotFoundError{}, err)
	err = h.svc.Unlink(ctx, h.p, uuid.New(), pub)
	assert.IsType(t, &IdentityNotFoundError{}, err)

	require.NoError(t, h.svc.Unlink(ctx, h.p, ana, pub))
	assert.Empty(t, h.store.rows)

	h.switches.err = errBoom
	err = h.svc.Unlink(ctx, h.p, ana, pub)
	assert.Equal(t, apperror.CodeUnexpected, codeOf(err))
	h.switches.err = nil

	h.store.err = errBoom
	err = h.svc.Unlink(ctx, h.p, ana, pub)
	assert.Equal(t, apperror.CodeUnexpected, codeOf(err))
}

func TestIdentitiesAndEveryEndUserLinked(t *testing.T) {
	ctx := context.Background()
	h := newHarness(t)
	ana := uuid.New()
	require.NoError(t, h.svc.Link(ctx, h.p, ana, identity(h)))
	ids, err := h.svc.Identities(ctx, h.p, ana)
	require.NoError(t, err)
	require.Len(t, ids, 1)
	assert.Equal(t, "sub-1", ids[0].Subject)

	all, err := h.svc.EveryEndUserLinked(ctx, h.p.OrgID, h.p.ID)
	require.NoError(t, err)
	assert.True(t, all)
	h.store.endUsers[uuid.New()] = true
	all, err = h.svc.EveryEndUserLinked(ctx, h.p.OrgID, h.p.ID)
	require.NoError(t, err)
	assert.False(t, all)

	h.store.err = errBoom
	_, err = h.svc.Identities(ctx, h.p, ana)
	assert.Equal(t, apperror.CodeUnexpected, codeOf(err))
	_, err = h.svc.EveryEndUserLinked(ctx, h.p.OrgID, h.p.ID)
	assert.Equal(t, apperror.CodeUnexpected, codeOf(err))
}

func TestSpec_Google(t *testing.T) {
	s := (&Service{}).spec(upstream.KindGoogle)
	assert.Equal(t, "https://accounts.google.com", s.Issuer)
	o := (&Service{cfg: Config{GoogleIssuerOverride: "https://idp.qa.example"}}).spec(upstream.KindGoogle)
	assert.Equal(t, "https://idp.qa.example", o.Issuer)
	assert.Equal(t, []string{"idp.qa.example"}, o.AcceptIssuers)
	assert.False(t, strings.HasPrefix(o.AcceptIssuers[0], "http"))
}

func TestWithConfig(t *testing.T) {
	h := newHarness(t)
	first, _, err := h.svc.client(context.Background(), h.p, upstream.KindGoogle, redirectURI)
	require.NoError(t, err)
	h.svc.WithConfig(Config{GoogleIssuerOverride: h.idp.Issuer(), HTTPClient: h.idp.Client()})
	again, _, err := h.svc.client(context.Background(), h.p, upstream.KindGoogle, redirectURI)
	require.NoError(t, err)
	assert.NotSame(t, first, again, "a new config drops the cached clients")
}

// the identity keeps the canonical issuer, whichever accepted form the ID token's iss used.
func TestComplete_CanonicalIssuer(t *testing.T) {
	h := newHarness(t)
	h.idp.Claims = map[string]any{"iss": strings.TrimPrefix(h.idp.Issuer(), "https://")}
	id, err := h.roundTrip(t, "")
	require.NoError(t, err)
	assert.Equal(t, h.idp.Issuer(), id.Issuer)
	assert.True(t, strings.HasPrefix(id.Issuer, "https://"))
}

func TestStart_RefusesANonHTTPSIssuer(t *testing.T) {
	h := newHarness(t)
	h.svc.WithConfig(Config{GoogleIssuerOverride: "http://idp.test", HTTPClient: h.idp.Client()})
	req, err := h.svc.Start(context.Background(), h.p, upstream.KindGoogle, "")
	require.NoError(t, err)
	_, err = h.svc.AuthURL(context.Background(), h.p, upstream.KindGoogle, redirectURI, req)
	assert.Equal(t, apperror.CodeSignInUpstreamUnavailable, codeOf(err), "never a GEN900 at the identities insert")
}

// discovery naming another issuer is the deployer's to fix: ERROR with reason misconfigured, answered as SIG015.
func TestUpstreamError_Misconfigured(t *testing.T) {
	h := newHarness(t)
	var logs bytes.Buffer
	h.svc.WithLogger(slog.New(slog.NewTextHandler(&logs, nil)))
	h.svc.WithConfig(Config{GoogleIssuerOverride: h.idp.Issuer() + "/", HTTPClient: h.idp.Client()})
	req, err := h.svc.Start(context.Background(), h.p, upstream.KindGoogle, "")
	require.NoError(t, err)
	_, err = h.svc.AuthURL(context.Background(), h.p, upstream.KindGoogle, redirectURI, req)
	assert.Equal(t, apperror.CodeSignInUpstreamUnavailable, codeOf(err))
	assert.Contains(t, logs.String(), "level=ERROR")
	assert.Contains(t, logs.String(), "reason=misconfigured")

	logs.Reset()
	h.svc.WithConfig(Config{GoogleIssuerOverride: "https://127.0.0.1:1", HTTPClient: h.idp.Client()})
	_, err = h.svc.AuthURL(context.Background(), h.p, upstream.KindGoogle, redirectURI, req)
	assert.Equal(t, apperror.CodeSignInUpstreamUnavailable, codeOf(err))
	assert.Contains(t, logs.String(), "level=WARN", "an unreachable upstream is transient")
	assert.Contains(t, logs.String(), "reason=discovery")
}

// a person's own cancel is not a fault: INFO, and the log names the upstream IdP, never a bare "provider".
func TestUpstreamError_CancelLogsInfo(t *testing.T) {
	h := newHarness(t)
	var logs bytes.Buffer
	h.svc.WithLogger(slog.New(slog.NewTextHandler(&logs, nil)))
	ctx := context.Background()
	req, err := h.svc.Start(ctx, h.p, upstream.KindGoogle, "")
	require.NoError(t, err)
	authURL, err := h.svc.AuthURL(ctx, h.p, upstream.KindGoogle, redirectURI, req)
	require.NoError(t, err)
	_, err = h.svc.Complete(ctx, h.p, upstream.KindGoogle, redirectURI, oidcrp.ParseCallback(h.idp.Deny(authURL)), req)
	assert.Equal(t, apperror.CodeSignInUpstreamCancelled, codeOf(err))
	assert.Contains(t, logs.String(), "level=INFO")
	assert.Contains(t, logs.String(), "reason=cancelled")
	assert.NotContains(t, logs.String(), "level=WARN")

	logs.Reset()
	cb := oidcrp.ParseCallback(h.idp.Deny(authURL))
	cb.Error = "server_error"
	_, err = h.svc.Complete(ctx, h.p, upstream.KindGoogle, redirectURI, cb, req)
	assert.Equal(t, apperror.CodeSignInUpstreamUnavailable, codeOf(err))
	assert.Contains(t, logs.String(), "level=WARN")
	assert.Contains(t, logs.String(), "the upstream IdP answered server_error")
	assert.NotContains(t, logs.String(), "provider")
}

// Real-Google interop: Google's cancel carries no iss though discovery advertises it; it is still a cancel (SIG012), not SIG016.
func TestComplete_GoogleCancelWithoutISS(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	req, err := h.svc.Start(ctx, h.p, upstream.KindGoogle, "")
	require.NoError(t, err)
	authURL, err := h.svc.AuthURL(ctx, h.p, upstream.KindGoogle, redirectURI, req)
	require.NoError(t, err)
	q := h.idp.Deny(authURL)
	q.Del("iss")
	_, err = h.svc.Complete(ctx, h.p, upstream.KindGoogle, redirectURI, oidcrp.ParseCallback(q), req)
	assert.Equal(t, apperror.CodeSignInUpstreamCancelled, codeOf(err))
}
