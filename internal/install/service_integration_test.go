//go:build integration

package install_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/altalune-id/othra/apperror"
	"github.com/altalune-id/othra/internal/client"
	"github.com/altalune-id/othra/internal/install"
	"github.com/altalune-id/othra/internal/kernel/db"
	"github.com/altalune-id/othra/internal/legal"
	"github.com/altalune-id/othra/internal/org"
	"github.com/altalune-id/othra/internal/org/invite"
	"github.com/altalune-id/othra/internal/org/member"
	"github.com/altalune-id/othra/internal/project"
	"github.com/altalune-id/othra/internal/testutil/fakes"
	"github.com/altalune-id/othra/internal/testutil/pgtest"
	"github.com/altalune-id/othra/internal/user"
	"github.com/altalune-id/othra/password"
)

type installFixture struct {
	pool     db.Pool
	svc      *install.Service
	users    *user.Service
	orgs     *org.Service
	members  *member.Service
	projects *project.Service
	clients  *client.Service
	invites  *ownerInvites
	terms    *legal.Service
}

var console = install.Console{RedirectURI: "http://localhost:5150/oauth/callback", PostLogoutURI: "http://localhost:5150/console/"}

type ownerInvites struct {
	mu      sync.Mutex
	emails  []string
	pending bool
	err     error
}

func (o *ownerInvites) InviteOwner(_ context.Context, _ uuid.UUID, email string) (*invite.Invite, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.err != nil {
		return nil, o.err
	}
	o.emails, o.pending = append(o.emails, email), true
	return &invite.Invite{Email: email, Role: member.RoleOwner}, nil
}

func (o *ownerInvites) PendingOwner(context.Context, uuid.UUID) (bool, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.pending, o.err
}

func (f *installFixture) deps() install.Deps {
	return install.Deps{
		Transactor: db.NewTransactor(f.pool), Users: f.users, Orgs: f.orgs, Members: f.members, Projects: f.projects,
		Clients: f.clients, Invites: f.invites, Terms: f.terms, Console: console, Passwords: true,
	}
}

func newInstallFixture(t *testing.T) *installFixture {
	t.Helper()
	return newInstallFixtureWithConns(t, 0)
}

func newInstallFixtureWithConns(t *testing.T, maxOpen int) *installFixture {
	t.Helper()
	pool, _, schemaName := pgtest.MigratedRLS(t)
	pool.W.SetMaxOpenConns(maxOpen)
	unexpected := apperror.NewReporter(nil, false).Unexpected
	f := &installFixture{
		pool:     pool,
		users:    user.NewService(user.NewStore(pool, schemaName, ""), unexpected),
		orgs:     org.NewService(org.NewStore(pool, schemaName, ""), db.NewTransactor(pool), fakes.NoMemberRoles{}, nil, unexpected).WithCreation(false),
		members:  member.NewService(member.NewStore(pool, schemaName, ""), db.NewTransactor(pool), unexpected),
		projects: project.NewService(project.NewStore(pool, schemaName, ""), fakes.Transactor{}, fakes.NoMemberships{}, nil, "", unexpected).WithOrgSlugs(fakes.OrgSlugs("")),
	}
	f.terms = legal.NewService(f.users, legal.NewStore(pool, schemaName, ""), db.NewTransactor(pool))
	f.clients = client.NewService(client.NewStore(pool, schemaName, ""), db.NewTransactor(pool), f.projects, nil, client.MetadataConfig{}, unexpected)
	f.invites = &ownerInvites{}
	f.svc = install.NewService(f.deps())
	return f
}

func (f *installFixture) orgCount(t *testing.T) int {
	t.Helper()
	all, err := f.orgs.IDs(t.Context())
	require.NoError(t, err)
	return len(all)
}

func TestInstall_Fresh(t *testing.T) {
	f := newInstallFixture(t)
	ctx := t.Context()

	required, err := f.svc.Required(ctx, false)
	require.NoError(t, err)
	assert.True(t, required)

	u, err := f.svc.Install(ctx, install.Request{Name: "Ada", Email: "Ada@Example.com", Password: "a-good-password", OrgName: "Acme"})
	require.NoError(t, err)
	assert.Equal(t, "ada@example.com", u.Email)

	sys, err := f.orgs.System(ctx)
	require.NoError(t, err)
	assert.True(t, sys.IsSystem)
	assert.Equal(t, "Acme", sys.Name)
	assert.Equal(t, "default", sys.Slug)
	assert.Equal(t, u.ID, sys.CreatedBy.UUID)

	m, err := f.members.Find(ctx, sys.ID, u.ID)
	require.NoError(t, err)
	assert.Equal(t, member.RoleOwner, m.Role)
	assert.Nil(t, u.TermsAcceptedAt, "no box ticked, no acceptance")
	_, err = f.projects.BySlug(ctx, sys.ID, project.DefaultSlug)
	require.NoError(t, err)

	required, err = f.svc.Required(ctx, false)
	require.NoError(t, err)
	assert.False(t, required)

	_, err = f.svc.Install(ctx, install.Request{Email: "eve@example.com", Password: "a-good-password"})
	assert.True(t, install.IsAlreadyInstalledError(err))
	_, err = f.users.ByEmail(ctx, "eve@example.com")
	assert.True(t, user.IsNotFoundError(err), "a refused install writes nothing")
}

func TestInstall_ConcurrentOnlyOneWins(t *testing.T) {
	f := newInstallFixture(t)
	ctx := t.Context()

	const n = 4
	errs := make([]error, n)
	var wg sync.WaitGroup
	for i := range n {
		wg.Go(func() {
			_, errs[i] = f.svc.Install(ctx, install.Request{Email: "user" + string(rune('a'+i)) + "@example.com", Password: "a-good-password"})
		})
	}
	wg.Wait()

	wins := 0
	for _, err := range errs {
		if err == nil {
			wins++
			continue
		}
		assert.True(t, install.IsAlreadyInstalledError(err), err)
	}
	assert.Equal(t, 1, wins)
	sys, err := f.orgs.System(ctx)
	require.NoError(t, err)
	owners, err := f.members.CountOwners(ctx, sys.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, owners)
	assert.Equal(t, 1, f.orgCount(t))
}

func TestInstall_ConcurrentOnASmallPoolDoesNotDeadlock(t *testing.T) {
	f := newInstallFixtureWithConns(t, 2)
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()

	const n = 3
	errs := make([]error, n)
	var wg sync.WaitGroup
	for i := range n {
		wg.Go(func() {
			_, errs[i] = f.svc.Install(ctx, install.Request{Email: fmt.Sprintf("user%d@example.com", i), Password: "a-good-password"})
		})
	}
	wg.Wait()

	wins := 0
	for _, err := range errs {
		if err == nil {
			wins++
			continue
		}
		assert.True(t, install.IsAlreadyInstalledError(err), err)
	}
	assert.Equal(t, 1, wins)
}

func TestInstall_ClaimsBasicSystemOrg(t *testing.T) {
	f := newInstallFixture(t)
	ctx := t.Context()

	require.NoError(t, f.svc.EnsureBasicSystemOrg(ctx))
	require.NoError(t, f.svc.EnsureBasicSystemOrg(ctx))
	assert.Equal(t, 1, f.orgCount(t))
	sys, err := f.orgs.System(ctx)
	require.NoError(t, err)
	assert.False(t, sys.CreatedBy.Valid)
	owners, err := f.members.CountOwners(ctx, sys.ID)
	require.NoError(t, err)
	assert.Zero(t, owners)
	_, err = f.projects.BySlug(ctx, sys.ID, project.DefaultSlug)
	require.NoError(t, err)

	required, err := f.svc.Required(ctx, true)
	require.NoError(t, err)
	assert.False(t, required)
	required, err = f.svc.Required(ctx, false)
	require.NoError(t, err)
	assert.True(t, required, "switching basic to local still needs an owner")

	u, err := f.svc.Install(ctx, install.Request{Email: "ada@example.com", Password: "a-good-password", OrgName: "Ignored"})
	require.NoError(t, err)
	claimed, err := f.orgs.System(ctx)
	require.NoError(t, err)
	assert.Equal(t, sys.ID, claimed.ID)
	assert.Equal(t, 1, f.orgCount(t))
	m, err := f.members.Find(ctx, sys.ID, u.ID)
	require.NoError(t, err)
	assert.Equal(t, member.RoleOwner, m.Role)
	all, err := f.projects.List(ctx, sys.ID)
	require.NoError(t, err)
	assert.Len(t, all, 2, "the default and console projects are not duplicated")
}

func TestInstall_WeakPasswordWritesNothing(t *testing.T) {
	f := newInstallFixture(t)
	ctx := t.Context()

	_, err := f.svc.Install(ctx, install.Request{Email: "ada@example.com", Password: "short"})
	assert.True(t, password.IsPolicyError(err))
	assert.Zero(t, f.orgCount(t))
	_, err = f.users.ByEmail(ctx, "ada@example.com")
	assert.True(t, user.IsNotFoundError(err))
}

func TestInstall_FailureAfterUserRollsBack(t *testing.T) {
	f := newInstallFixture(t)
	ctx := t.Context()

	_, err := f.svc.Install(ctx, install.Request{Email: "ada@example.com", Password: "a-good-password", OrgName: strings.Repeat("x", 500)})
	assert.True(t, org.IsInvalidNameError(err))
	_, err = f.users.ByEmail(ctx, "ada@example.com")
	assert.True(t, user.IsNotFoundError(err), "the user insert is rolled back with the failed org")
	assert.Zero(t, f.orgCount(t))
}

var errInjected = errors.New("injected failure")

type failSystem struct{ install.SystemOrgs }

func (failSystem) System(context.Context) (*org.Org, error) { return nil, errInjected }

type failCount struct{ install.Owners }

func (failCount) CountOwners(context.Context, uuid.UUID) (int, error) { return 0, errInjected }

type failAdd struct{ install.Owners }

func (failAdd) Add(context.Context, uuid.UUID, uuid.UUID, member.Role) (*member.Membership, error) {
	return nil, errInjected
}

type failProjectCreate struct{ install.Projects }

func (failProjectCreate) Create(context.Context, project.CreateRequest) (*project.Project, error) {
	return nil, errInjected
}

func TestInstall_DependencyFailuresRollBack(t *testing.T) {
	cases := []struct {
		name  string
		basic bool
		wrap  func(f *installFixture, pool db.Pool) *install.Service
	}{
		{"system org lookup", false, func(f *installFixture, pool db.Pool) *install.Service {
			d := f.deps()
			d.Orgs = failSystem{f.orgs}
			return install.NewService(d)
		}},
		{"owner count", true, func(f *installFixture, pool db.Pool) *install.Service {
			d := f.deps()
			d.Members = failCount{f.members}
			return install.NewService(d)
		}},
		{"owner membership", false, func(f *installFixture, pool db.Pool) *install.Service {
			d := f.deps()
			d.Members = failAdd{f.members}
			return install.NewService(d)
		}},
		{"default project", false, func(f *installFixture, pool db.Pool) *install.Service {
			d := f.deps()
			d.Projects = failProjectCreate{f.projects}
			return install.NewService(d)
		}},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			f := newInstallFixture(t)
			ctx := t.Context()
			if tt.basic {
				require.NoError(t, f.svc.EnsureBasicSystemOrg(ctx))
			}
			before := f.orgCount(t)

			_, err := tt.wrap(f, f.pool).Install(ctx, install.Request{Email: "ada@example.com", Password: "a-good-password"})

			require.ErrorIs(t, err, errInjected)
			_, err = f.users.ByEmail(ctx, "ada@example.com")
			assert.True(t, user.IsNotFoundError(err), "the user insert is rolled back")
			assert.Equal(t, before, f.orgCount(t))
		})
	}
}

func TestEnsureBasicSystemOrg_DependencyFailures(t *testing.T) {
	f := newInstallFixture(t)
	ctx := t.Context()

	d := f.deps()
	d.Orgs = failSystem{f.orgs}
	err := install.NewService(d).EnsureBasicSystemOrg(ctx)
	require.ErrorIs(t, err, errInjected)

	d = f.deps()
	d.Projects = failProjectCreate{f.projects}
	err = install.NewService(d).EnsureBasicSystemOrg(ctx)
	require.ErrorIs(t, err, errInjected)
	assert.Zero(t, f.orgCount(t), "the system org insert is rolled back with the failed project")
}

func TestInstall_RecordsAcceptedTerms(t *testing.T) {
	f := newInstallFixture(t)
	ctx := t.Context()
	before := time.Now().Add(-time.Second)
	u, err := f.svc.Install(ctx, install.Request{Email: "ada@example.com", Password: "a-good-password", AcceptTerms: true})
	require.NoError(t, err)
	stored, err := f.users.ByID(ctx, u.ID)
	require.NoError(t, err)
	require.NotNil(t, stored.TermsAcceptedAt)
	assert.True(t, stored.TermsAcceptedAt.After(before))
}

// The console project install creates is seeded once (the upstream IdP from config); a refused install seeds nothing.
func TestInstall_SeedsTheConsoleProjectOnce(t *testing.T) {
	f := newInstallFixture(t)
	ctx := t.Context()
	var seeded []uuid.UUID
	d := f.deps()
	d.Seed = func(_ context.Context, _, projectID uuid.UUID) error {
		seeded = append(seeded, projectID)
		return nil
	}
	f.svc = install.NewService(d)
	_, err := f.svc.Install(ctx, install.Request{Email: "ada@example.com", Password: "a-good-password", OrgName: "Acme"})
	require.NoError(t, err)
	sys, err := f.orgs.System(ctx)
	require.NoError(t, err)
	p, err := f.projects.BySlug(ctx, sys.ID, project.SystemSlug)
	require.NoError(t, err)
	assert.Equal(t, []uuid.UUID{p.ID}, seeded)
	_, err = f.svc.Install(ctx, install.Request{Email: "eve@example.com", Password: "a-good-password"})
	assert.True(t, install.IsAlreadyInstalledError(err))
	assert.Len(t, seeded, 1)
}

// Install seeds the console project /p/system and its client in the same unit of work as the system org and the first owner.
func TestInstall_SeedsTheConsole(t *testing.T) {
	f := newInstallFixture(t)
	ctx := t.Context()
	_, err := f.svc.Install(ctx, install.Request{Email: "ada@example.com", Password: "a-good-password", OrgName: "Acme"})
	require.NoError(t, err)
	sys, err := f.orgs.System(ctx)
	require.NoError(t, err)
	p, err := f.projects.BySlug(ctx, sys.ID, project.SystemSlug)
	require.NoError(t, err)
	assert.True(t, p.IsSystem())
	assert.Equal(t, project.SystemHandle, p.IssuerHandle)
	assert.Equal(t, "Othra", p.DisplayName)
	assert.NotNil(t, p.IssuerHandleLockedAt, "the console client locks the handle")

	c, err := f.clients.System(ctx, sys.ID, p.ID)
	require.NoError(t, err)
	assert.True(t, c.System)
	assert.Equal(t, []string{console.RedirectURI}, c.RedirectURIs)
	assert.Equal(t, []string{console.PostLogoutURI}, c.PostLogoutRedirectURIs)
	assert.Equal(t, client.AuthNone, c.TokenEndpointAuthMethod)

	changed, err := f.clients.ReconcileSystem(ctx, sys.ID, p.ID, console.RedirectURI, console.PostLogoutURI)
	require.NoError(t, err)
	assert.False(t, changed, "a second boot changes nothing")
	changed, err = f.clients.ReconcileSystem(ctx, sys.ID, p.ID, "http://127.0.0.1:5150/oauth/callback", "http://127.0.0.1:5150/console/")
	require.NoError(t, err)
	assert.True(t, changed, "a new public URL")

	bad := f.deps()
	bad.Console.RedirectURI = "not a url"
	f2 := newInstallFixture(t)
	bad.Transactor, bad.Users, bad.Orgs, bad.Members, bad.Projects, bad.Clients = db.NewTransactor(f2.pool), f2.users, f2.orgs, f2.members, f2.projects, f2.clients
	_, err = install.NewService(bad).Install(ctx, install.Request{Email: "ada@example.com", Password: "a-good-password"})
	require.Error(t, err)
	assert.Zero(t, f2.orgCount(t), "a failed console client rolls back the whole install")
	_, err = f2.users.ByEmail(ctx, "ada@example.com")
	assert.True(t, user.IsNotFoundError(err))
}

// With passwords off, install creates the ownerless system org, its projects and the console client, and invites the owner.
func TestInstall_EmailOnly(t *testing.T) {
	f := newInstallFixture(t)
	ctx := t.Context()
	d := f.deps()
	d.Passwords = false
	f.svc = install.NewService(d)
	assert.False(t, f.svc.Passwords())

	u, err := f.svc.Install(ctx, install.Request{Email: "ada@example.com", OrgName: "Acme"})
	require.NoError(t, err)
	assert.Nil(t, u, "no console user until the invite is claimed")
	assert.Equal(t, []string{"ada@example.com"}, f.invites.emails)
	sys, err := f.orgs.System(ctx)
	require.NoError(t, err)
	assert.Equal(t, "Acme", sys.Name)
	assert.False(t, sys.CreatedBy.Valid)
	owners, err := f.members.CountOwners(ctx, sys.ID)
	require.NoError(t, err)
	assert.Zero(t, owners)
	p, err := f.projects.BySlug(ctx, sys.ID, project.SystemSlug)
	require.NoError(t, err)
	_, err = f.clients.System(ctx, sys.ID, p.ID)
	require.NoError(t, err)

	required, err := f.svc.Required(ctx, false)
	require.NoError(t, err)
	assert.False(t, required, "the pending owner invite closes /install")
	_, err = f.svc.Install(ctx, install.Request{Email: "eve@example.com"})
	assert.True(t, install.IsAlreadyInstalledError(err))

	g := newInstallFixture(t)
	gd := g.deps()
	gd.Passwords = false
	g.invites.err = errInjected
	_, err = install.NewService(gd).Install(ctx, install.Request{Email: "ada@example.com"})
	require.ErrorIs(t, err, errInjected)
	assert.Zero(t, g.orgCount(t), "a failed invite rolls back the system org")
}
