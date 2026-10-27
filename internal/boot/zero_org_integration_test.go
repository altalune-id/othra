//go:build integration

package boot

import (
	"context"
	"net/http"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	orgv1 "github.com/altalune-id/othra/gen/go/org/v1"
	orgv1connect "github.com/altalune-id/othra/gen/go/org/v1/orgv1connect"
	"github.com/altalune-id/othra/internal/kernel/session"
	"github.com/altalune-id/othra/internal/org/member"
	"github.com/altalune-id/othra/internal/user"
)

func (f *apiFixture) orglessCookie(t *testing.T, email string) string {
	t.Helper()
	ctx := t.Context()
	_, err := f.svcs.Users.Create(ctx, user.CreateRequest{Email: email, Name: email, Password: "quiet-forest-lamp-9"})
	require.NoError(t, err)
	p, err := f.svcs.Auth.LoginLocal(ctx, authCredentials(email, "quiet-forest-lamp-9"))
	require.NoError(t, err)
	sid, err := session.NewID()
	require.NoError(t, err)
	require.NoError(t, f.svcs.Sessions.Save(ctx, sid, p, time.Now().Add(time.Hour)))
	return session.CookieName + "=" + session.Sign(f.svcs.StateSecret, sid)
}

func (f *apiFixture) listOrgs(t *testing.T, cookie string) ([]*orgv1.Org, error) {
	t.Helper()
	resp, err := orgv1connect.NewOrgServiceClient(http.DefaultClient, f.url).
		ListOrgs(context.Background(), withCookie(connect.NewRequest(&orgv1.ListOrgsRequest{}), cookie))
	if err != nil {
		return nil, err
	}
	return resp.Msg.GetOrgs(), nil
}

func TestAPI_APersonInNoOrgListsAnEmptySet(t *testing.T) {
	f := newAPIFixture(t)

	orgs, err := f.listOrgs(t, f.orglessCookie(t, "nobody@example.com"))
	require.NoError(t, err, "listing your own orgs needs no org permission")
	assert.Empty(t, orgs)

	cookie, _, _ := f.personCookie(t, "member@example.com", member.RoleMember)
	orgs, err = f.listOrgs(t, cookie)
	require.NoError(t, err)
	require.Len(t, orgs, 1, "a member sees only their own org")
	assert.Equal(t, f.orgID, orgs[0].GetId())
}
