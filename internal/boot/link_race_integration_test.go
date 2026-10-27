//go:build integration

package boot

import (
	"context"
	"encoding/json"
	"net/url"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"altalune.id/othra/apperror"
	"altalune.id/othra/internal/enduser"
	"altalune.id/othra/internal/federation"
	"altalune.id/othra/internal/kernel/tenant"
	"altalune.id/othra/internal/project"
	"altalune.id/othra/internal/upstream"
)

type linkRace struct {
	f   *signInFixture
	ctx context.Context
	p   *project.Project
	eu  enduser.Project
	fp  federation.Project
	up  uuid.UUID
}

func newLinkRace(t *testing.T) *linkRace {
	t.Helper()
	f := newSignInFixture(t)
	p := f.project("race", "Race")
	ctx := tenant.Into(t.Context(), tenant.Context{OrgID: p.OrgID, ProjectID: p.ID})
	require.NoError(t, f.svcs.Upstreams.Seed(ctx, p.OrgID, p.ID, upstream.KindGoogle, "cid.apps", "s3cret"))
	ups, err := f.svcs.Upstreams.List(ctx, p.OrgID, p.ID)
	require.NoError(t, err)
	require.Len(t, ups, 1)
	return &linkRace{
		f: f, ctx: ctx, p: p, up: ups[0].ID,
		eu: enduser.Project{OrgID: p.OrgID, ID: p.ID, PublicID: p.PublicID, IssuerURL: "https://id.example.com/p/race", EmailLink: true},
		fp: federation.Project{OrgID: p.OrgID, ID: p.ID, PublicID: p.PublicID},
	}
}

func (r *linkRace) token(t *testing.T, email string, pending *federation.Identity) string {
	t.Helper()
	req := enduser.LinkRequest{Email: email, ReturnTo: "/p/race/account/"}
	if pending != nil {
		raw, err := json.Marshal(pending)
		require.NoError(t, err)
		req.Pending = raw
	}
	l, err := r.f.svcs.EndUsers.IssueLink(r.ctx, r.eu, req)
	require.NoError(t, err)
	require.True(t, l.Issued)
	u, err := url.Parse(l.URL)
	require.NoError(t, err)
	return u.Query().Get("t")
}

func (r *linkRace) existing(t *testing.T, email string) {
	t.Helper()
	_, err := r.f.svcs.EndUsers.Redeem(r.ctx, r.eu, enduser.RedeemRequest{Token: r.token(t, email, nil)})
	require.NoError(t, err)
}

func (r *linkRace) identity(email, subject string) federation.Identity {
	return federation.Identity{
		UpstreamID: r.up, Issuer: "https://accounts.google.com", Subject: subject, Email: email, EmailVerified: true, Kind: upstream.KindGoogle,
	}
}

func (r *linkRace) confirm(id federation.Identity) func(ctx context.Context, _ *enduser.Link, endUserID uuid.UUID, _ bool) (bool, error) {
	return func(ctx context.Context, _ *enduser.Link, endUserID uuid.UUID, _ bool) (bool, error) {
		return true, r.f.svcs.Federation.Link(ctx, r.fp, endUserID, id)
	}
}

func (r *linkRace) count(t *testing.T, query string) int {
	t.Helper()
	var n int
	require.NoError(t, r.f.db.QueryRowContext(t.Context(), query, r.p.ID).Scan(&n))
	return n
}

// concurrent confirms of one collision link spend it once and link one identity.
func TestLinkRace_ConcurrentConfirmsLinkOnce(t *testing.T) {
	r := newLinkRace(t)
	r.existing(t, "ana@example.com")
	id := r.identity("ana@example.com", "g-ana")
	token := r.token(t, "ana@example.com", &id)

	var wg sync.WaitGroup
	errs := make([]error, 4)
	for i := range errs {
		wg.Go(func() {
			_, errs[i] = r.f.svcs.EndUsers.Redeem(r.ctx, r.eu, enduser.RedeemRequest{Token: token, Confirm: r.confirm(id)})
		})
	}
	wg.Wait()
	wins := 0
	for _, err := range errs {
		if err == nil {
			wins++
			continue
		}
		assert.True(t, enduser.IsLinkNotUsableError(err), "%v", err)
	}
	assert.Equal(t, 1, wins)
	assert.Equal(t, 1, r.count(t, `SELECT count(*) FROM end_user_identities WHERE project_id = $1`))
	assert.Equal(t, 1, r.count(t, `SELECT count(*) FROM end_users WHERE project_id = $1`))
	assert.Equal(t, 1, r.count(t, `SELECT count(*) FROM magic_links WHERE project_id = $1 AND pending_identity IS NOT NULL AND used_at IS NOT NULL`))
}

// a link confirm racing "email link off" never leaves an end user without a sign-in method.
func TestLinkRace_ConfirmVersusEmailLinkOff(t *testing.T) {
	for range 3 {
		r := newLinkRace(t)
		r.existing(t, "ana@example.com")
		id := r.identity("ana@example.com", "g-ana")
		token := r.token(t, "ana@example.com", &id)

		var wg sync.WaitGroup
		var redeemErr, switchErr error
		wg.Go(func() {
			_, redeemErr = r.f.svcs.EndUsers.Redeem(r.ctx, r.eu, enduser.RedeemRequest{Token: token, Confirm: r.confirm(id)})
		})
		wg.Go(func() {
			_, switchErr = r.f.svcs.Projects.SetSignInEmailLink(r.ctx, r.p.ID, false)
		})
		wg.Wait()
		require.NoError(t, redeemErr, "the confirm links an existing end user either way")
		assert.Equal(t, 1, r.count(t, `SELECT count(*) FROM end_user_identities WHERE project_id = $1`))
		var on bool
		require.NoError(t, r.f.db.QueryRowContext(t.Context(), `SELECT signin_email_link FROM projects WHERE id = $1`, r.p.ID).Scan(&on))
		if switchErr != nil {
			assert.True(t, on, "a refused switch-off leaves the email link on: %v", switchErr)
			continue
		}
		assert.False(t, on)
		assert.Zero(t, r.count(t, `SELECT count(*) FROM end_users e WHERE e.project_id = $1
			AND NOT EXISTS (SELECT 1 FROM end_user_identities i WHERE i.end_user_id = e.id)`), "no stranded end user")
	}
}

// with the email link off, a Google sign-up link creates the end user and links the identity together, or neither.
func TestSignUpLink_CreatesAndLinksInOneUnitOfWork(t *testing.T) {
	r := newLinkRace(t)
	_, err := r.f.svcs.Projects.SetSignInEmailLink(r.ctx, r.p.ID, false)
	require.NoError(t, err)
	r.eu.EmailLink = false
	users := `SELECT count(*) FROM end_users WHERE project_id = $1`
	identities := `SELECT count(*) FROM end_user_identities WHERE project_id = $1`

	budi := r.identity("budi@acme.test", "g-budi")
	_, err = r.f.svcs.EndUsers.Redeem(r.ctx, r.eu, enduser.RedeemRequest{Token: r.token(t, "budi@acme.test", &budi)})
	assert.True(t, enduser.IsEmailDisabledError(err), "another browser with the email link off: %v", err)
	assert.Zero(t, r.count(t, users))

	clash := r.identity("cara@acme.test", "g-budi")
	token := r.token(t, "budi@acme.test", &budi)
	_, err = r.f.svcs.EndUsers.Redeem(r.ctx, r.eu, enduser.RedeemRequest{Token: token, Confirm: r.confirm(budi)})
	require.NoError(t, err)
	assert.Equal(t, 1, r.count(t, users))
	assert.Equal(t, 1, r.count(t, identities))
	assert.Equal(t, 1, r.count(t, `SELECT count(*) FROM user_sessions WHERE project_id = $1`))

	clashToken := r.token(t, "cara@acme.test", &clash)
	_, err = r.f.svcs.EndUsers.Redeem(r.ctx, r.eu, enduser.RedeemRequest{Token: clashToken, Confirm: r.confirm(clash)})
	app, ok := apperror.AsAppError(err)
	require.True(t, ok, "%v", err)
	assert.Equal(t, apperror.CodeSignInUpstreamLinkedElsewhere, app.Code(), "the sub is budi's")
	assert.Equal(t, 1, r.count(t, users), "cara was rolled back with the failed link")
	_, err = r.f.svcs.EndUsers.LinkByToken(r.ctx, r.eu, clashToken)
	require.NoError(t, err, "the link is still unused")
}
