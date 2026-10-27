//go:build integration

package enduser_test

import (
	"context"
	"net/url"
	"testing"

	"github.com/go-jet/jet/v2/qrm"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/altalune-id/othra/internal/enduser"
	"github.com/altalune-id/othra/internal/kernel/db"
)

func TestPostgres_PendingIdentityRoundTrip(t *testing.T) {
	f := newPGFixture(t)
	p := f.project(t)
	f.link(t, p, "ada@example.com")
	issued, err := f.svc.IssueLink(t.Context(), p, enduser.LinkRequest{
		Email: "ada@example.com", ReturnTo: "/p/h/account/", Pending: []byte(`{"Subject": "g-1"}`),
	})
	require.NoError(t, err)
	require.True(t, issued.Issued, "the collision link replaces the live one inside the cooldown")
	u, err := url.Parse(issued.URL)
	require.NoError(t, err)
	token := u.Query().Get("t")

	l, err := f.svc.LinkByToken(t.Context(), p, token)
	require.NoError(t, err)
	assert.JSONEq(t, `{"Subject":"g-1"}`, string(l.Pending))

	confirmed := false
	r, err := f.svc.Redeem(t.Context(), p, enduser.RedeemRequest{
		Token: token,
		Confirm: func(_ context.Context, cl *enduser.Link, endUserID uuid.UUID, _ bool) (bool, error) {
			confirmed = cl.PublicID == l.PublicID && endUserID != uuid.Nil
			return true, nil
		},
	})
	require.NoError(t, err)
	assert.True(t, confirmed)
	assert.Equal(t, enduser.MethodMagicLink, r.Account.Method)

	plainToken, plain := f.link(t, p, "bob@example.com")
	assert.Nil(t, plain.Pending)
	_, err = f.svc.Redeem(t.Context(), p, enduser.RedeemRequest{Token: plainToken})
	require.NoError(t, err)
}

type rowSwitch struct{ pool db.Pool }

func (s rowSwitch) LockEmailLink(ctx context.Context, projectID uuid.UUID) (bool, error) {
	var on bool
	err := db.Write(ctx, s.pool, func(q qrm.DB) error {
		rows, err := q.QueryContext(ctx, `SELECT signin_email_link FROM projects WHERE id = $1 FOR NO KEY UPDATE`, projectID)
		if err != nil {
			return err
		}
		defer rows.Close()
		if rows.Next() {
			if err := rows.Scan(&on); err != nil {
				return err
			}
		}
		return rows.Err()
	})
	return on, err
}

// the switch is read in the redeem's unit of work, and a refused sign-up rolls back, leaving the link unused.
func TestPostgres_RedeemRereadsTheEmailLinkSwitch(t *testing.T) {
	f := newPGFixture(t)
	p := f.project(t)
	known, _ := f.link(t, p, "ada@example.com")
	_, err := f.svc.Redeem(t.Context(), p, enduser.RedeemRequest{Token: known})
	require.NoError(t, err)
	newcomer, _ := f.link(t, p, "bob@example.com")
	again, _ := f.link(t, p, "ada@example.com")

	_, err = f.db.ExecContext(t.Context(), `UPDATE projects SET signin_email_link = false WHERE id = $1`, p.ID)
	require.NoError(t, err)
	_, err = f.svc.Redeem(t.Context(), p, enduser.RedeemRequest{Token: newcomer})
	assert.True(t, enduser.IsEmailDisabledError(err), "%v", err)
	assert.Equal(t, 1, f.count(t, "end_users", p))
	_, err = f.svc.LinkByToken(t.Context(), p, newcomer)
	require.NoError(t, err, "the refused redeem rolled back")

	_, err = f.svc.Redeem(t.Context(), p, enduser.RedeemRequest{Token: again})
	require.NoError(t, err, "an existing end user still signs in")
}

// FillName writes only an empty name.
func TestPostgres_FillNameOnlyWhenEmpty(t *testing.T) {
	f := newPGFixture(t)
	p := f.project(t)
	id, err := f.svc.CreateFederated(t.Context(), p, "budi@acme.test", "")
	require.NoError(t, err)
	require.NoError(t, f.svc.FillName(t.Context(), p, id, "Budi"))
	require.NoError(t, f.svc.FillName(t.Context(), p, id, "Someone else"))
	var name string
	require.NoError(t, f.db.QueryRowContext(t.Context(), `SELECT name FROM end_users WHERE id = $1`, id).Scan(&name))
	assert.Equal(t, "Budi", name)
}
