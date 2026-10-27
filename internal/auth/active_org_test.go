package auth

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/altalune-id/othra/internal/kernel/session"
	"github.com/altalune-id/othra/internal/org"
	"github.com/altalune-id/othra/internal/org/member"
	"github.com/altalune-id/othra/internal/user"
)

type switchFixture struct {
	*fixture
	person  *user.User
	home    *org.Org
	other   *org.Org
	foreign *org.Org
	sid     string
}

func newSwitchFixture(t *testing.T) *switchFixture {
	t.Helper()
	f := &switchFixture{fixture: newFixture(t, BasicCredentials{}), sid: "sid-1"}
	f.person = f.addUser(t, "ada@example.com", true)
	f.home, f.other, f.foreign = f.addOrg(t, false), f.addOrg(t, false), f.addOrg(t, false)
	f.join(t, f.home, f.person, member.RoleOwner)
	f.join(t, f.other, f.person, member.RoleMember)
	f.addProject(f.other)
	return f
}

func (f *switchFixture) signedIn(t *testing.T) session.Principal {
	t.Helper()
	p, err := f.svc.Refresh(context.Background(), principalOf(f.person, time.Now()))
	require.NoError(t, err)
	require.Equal(t, f.home.ID, p.ActiveOrgID, "a fresh session starts in the first membership")
	require.NoError(t, f.sessions.Save(context.Background(), f.sid, p, time.Now().Add(time.Hour)))
	return p
}

func (f *switchFixture) stored(t *testing.T) session.Principal {
	t.Helper()
	p, ok, err := f.sessions.Load(context.Background(), f.sid)
	require.NoError(t, err)
	require.True(t, ok)
	return p
}

func TestSetActiveOrg_AMemberSwitchesAndScopesFollowTheRole(t *testing.T) {
	f := newSwitchFixture(t)
	p := f.signedIn(t)

	got, err := f.svc.SetActiveOrg(context.Background(), f.sid, p, f.other.ID)
	require.NoError(t, err)
	assert.Equal(t, f.other.ID, got.ActiveOrgID)
	assert.Equal(t, f.projects.byOrg[f.other.ID][0].ID, got.ActiveProjectID, "the new org's first project becomes active")
	assert.Equal(t, member.BuiltinPermissions(member.RoleMember), got.Scopes, "scopes come from the role in the new org")

	assert.Equal(t, f.other.ID, f.stored(t).ActiveOrgID, "the session keeps the switch")
	again, err := f.svc.Refresh(context.Background(), f.stored(t))
	require.NoError(t, err)
	assert.Equal(t, f.other.ID, again.ActiveOrgID, "the next request stays in the new org")
}

// SECURITY: an org you do not belong to and an unknown org answer the same not-found, and the session never moves.
func TestSetActiveOrg_RefusesAnOrgYouDoNotBelongTo(t *testing.T) {
	f := newSwitchFixture(t)
	p := f.signedIn(t)

	for name, id := range map[string]uuid.UUID{"foreign": f.foreign.ID, "unknown": uuid.New()} {
		_, err := f.svc.SetActiveOrg(context.Background(), f.sid, p, id)
		assert.True(t, org.IsNotFoundError(err), "%s: %v", name, err)
		assert.Equal(t, f.home.ID, f.stored(t).ActiveOrgID, name)
	}
}

func TestSetActiveOrg_OnlyAPersonWithALiveSessionSwitches(t *testing.T) {
	f := newSwitchFixture(t)
	p := f.signedIn(t)

	_, err := f.svc.SetActiveOrg(context.Background(), "gone", p, f.other.ID)
	assert.True(t, session.IsNotFoundError(err), "%v", err)

	key := session.Principal{Source: session.SourceAPIKey, ActiveOrgID: f.home.ID}
	_, err = f.svc.SetActiveOrg(context.Background(), f.sid, key, f.other.ID)
	assert.True(t, org.IsNotFoundError(err), "a key never switches: %v", err)
}

func TestSetActiveOrg_SupportStaffMayEnterAnyOrgOnlyWithOrgSupport(t *testing.T) {
	for name, support := range map[string]bool{"cloud staff": true, "selfhosted operator": false} {
		t.Run(name, func(t *testing.T) {
			f := newSwitchFixture(t)
			sys := f.addOrg(t, true)
			f.join(t, sys, f.person, member.RoleOwner)
			f.svc.WithOperatorOrgSupport(support)
			p := f.signedIn(t)
			require.True(t, p.Operator)

			got, err := f.svc.SetActiveOrg(context.Background(), f.sid, p, f.foreign.ID)
			if !support {
				assert.True(t, org.IsNotFoundError(err), "%v", err)
				assert.Equal(t, f.home.ID, f.stored(t).ActiveOrgID)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, f.foreign.ID, got.ActiveOrgID)
			assert.Equal(t, member.BuiltinPermissions(member.RoleAdmin), got.Scopes, "staff act as an admin there")
			again, err := f.svc.Refresh(context.Background(), f.stored(t))
			require.NoError(t, err)
			assert.Equal(t, f.foreign.ID, again.ActiveOrgID, "the switch survives the next request")

			_, err = f.svc.SetActiveOrg(context.Background(), f.sid, p, uuid.New())
			assert.True(t, org.IsNotFoundError(err), "an unknown org stays not found for staff: %v", err)
		})
	}
}

// A console session from Othra sign-in switches orgs like a password one, and keeps its /p/system link for sign-out.
func TestSetActiveOrg_OIDCSession(t *testing.T) {
	f := newSwitchFixture(t)
	p := f.signedIn(t)
	p.Source, p.IDPIssuer, p.IDPSubject, p.IDToken = session.SourceOIDC, ConsoleIssuer, "sub-1", "the.id.token"

	got, err := f.svc.SetActiveOrg(context.Background(), f.sid, p, f.other.ID)
	require.NoError(t, err)
	assert.Equal(t, f.other.ID, got.ActiveOrgID)
	assert.Equal(t, session.SourceOIDC, got.Source)
	assert.Equal(t, "the.id.token", f.stored(t).IDToken)
}
