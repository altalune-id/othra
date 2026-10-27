package install

import (
	"cmp"
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"

	"github.com/altalune-id/othra/internal/kernel/db"
	"github.com/altalune-id/othra/internal/kernel/tenant"
	"github.com/altalune-id/othra/internal/legal"
	"github.com/altalune-id/othra/internal/org"
	"github.com/altalune-id/othra/internal/org/member"
	"github.com/altalune-id/othra/internal/project"
	"github.com/altalune-id/othra/internal/user"
)

const (
	lockName       = "othra.install"
	defaultOrgName = "Default"
)

// Console is where the console client sends people back: its derived redirect and sign-out URIs.
type Console struct {
	RedirectURI   string
	PostLogoutURI string
}

// Deps are the services install writes through.
type Deps struct {
	Transactor db.Transactor
	Users      UserCreator
	Orgs       SystemOrgs
	Members    Owners
	Projects   Projects
	Clients    ConsoleClients
	Invites    OwnerInvites
	Terms      Terms
	Console    Console
	// Passwords is whether console passwords are on; when off, install takes an email and invites the first owner.
	Passwords bool
	// Seed, when not nil, runs once on the console project install creates.
	Seed func(ctx context.Context, orgID, projectID uuid.UUID) error
}

// Service installs the instance and reports whether it still needs installing.
type Service struct{ d Deps }

// NewService builds a Service over d.
func NewService(d Deps) *Service { return &Service{d: d} }

// Passwords reports whether install creates the first owner with a password, or invites them by email.
func (s *Service) Passwords() bool { return s.d.Passwords }

// Required reports whether the instance still needs installing: no system org, or in local mode no owner of it.
func (s *Service) Required(ctx context.Context, basic bool) (bool, error) {
	sys, err := s.d.Orgs.System(ctx)
	if org.IsNotFoundError(err) {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	if basic {
		return false, nil
	}
	return s.ownerless(ctx, sys.ID)
}

func (s *Service) ownerless(ctx context.Context, orgID uuid.UUID) (bool, error) {
	owners, err := s.d.Members.CountOwners(ctx, orgID)
	if err != nil || owners > 0 {
		return false, err
	}
	pending, err := s.d.Invites.PendingOwner(ctx, orgID)
	return !pending, err
}

// Install sets up the system org, its projects and the console client with the first owner (an owner invite when passwords are off).
func (s *Service) Install(ctx context.Context, r Request) (*user.User, error) {
	sysID, err := s.systemOrgID(ctx)
	if err != nil {
		return nil, err
	}
	var created *user.User
	err = s.d.Transactor.RunInTx(tenant.Into(ctx, tenant.Context{OrgID: sysID}), func(ctx context.Context) error {
		if err := db.LockInTx(ctx, lockName); err != nil {
			return err
		}
		sys, err := s.ownerlessSystemOrg(ctx, sysID)
		if err != nil {
			return err
		}
		if !s.d.Passwords {
			if sys, err = s.systemOrg(ctx, sys, sysID, r.OrgName, uuid.Nil); err != nil {
				return err
			}
			if _, err := s.d.Invites.InviteOwner(ctx, sys.ID, r.Email); err != nil {
				return err
			}
			return s.ensureProjects(ctx, sys.ID)
		}
		u, err := s.d.Users.Create(ctx, user.CreateRequest{Email: r.Email, Name: r.Name, Password: r.Password})
		if err != nil {
			return err
		}
		if r.AcceptTerms {
			if _, err := s.d.Terms.Accept(ctx, u.ID, legal.SourceInstall); err != nil {
				return err
			}
		}
		if sys, err = s.systemOrg(ctx, sys, sysID, r.OrgName, u.ID); err != nil {
			return err
		}
		if _, err := s.d.Members.Add(ctx, sys.ID, u.ID, member.RoleOwner); err != nil {
			return err
		}
		if err := s.ensureProjects(ctx, sys.ID); err != nil {
			return err
		}
		created = u
		return nil
	})
	if err != nil {
		return nil, err
	}
	return created, nil
}

// EnsureBasicSystemOrg creates the ownerless system org and its default project when missing.
func (s *Service) EnsureBasicSystemOrg(ctx context.Context) error {
	err := s.ensureBasicSystemOrg(ctx)
	if errors.Is(err, errSystemOrgMoved) {
		err = s.ensureBasicSystemOrg(ctx)
	}
	return err
}

func (s *Service) ensureBasicSystemOrg(ctx context.Context) error {
	sysID, err := s.systemOrgID(ctx)
	if err != nil {
		return err
	}
	return s.d.Transactor.RunInTx(tenant.Into(ctx, tenant.Context{OrgID: sysID}), func(ctx context.Context) error {
		if err := db.LockInTx(ctx, lockName); err != nil {
			return err
		}
		sys, err := s.d.Orgs.System(ctx)
		if org.IsNotFoundError(err) {
			sys, err = s.d.Orgs.CreateSystem(ctx, org.CreateRequest{ID: sysID, Name: defaultOrgName})
		}
		if err != nil {
			return err
		}
		if sys.ID != sysID {
			return errSystemOrgMoved
		}
		return s.ensureProjects(ctx, sys.ID)
	})
}

// SECURITY: the system org is scoped by its own id, chosen before the transaction opens; a new one gets a fresh id.
func (s *Service) systemOrgID(ctx context.Context) (uuid.UUID, error) {
	sys, err := s.d.Orgs.System(ctx)
	switch {
	case org.IsNotFoundError(err):
		return uuid.New(), nil
	case err != nil:
		return uuid.Nil, err
	}
	return sys.ID, nil
}

func (s *Service) ownerlessSystemOrg(ctx context.Context, scoped uuid.UUID) (*org.Org, error) {
	sys, err := s.d.Orgs.System(ctx)
	if org.IsNotFoundError(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if sys.ID != scoped {
		return nil, &AlreadyInstalledError{}
	}
	ownerless, err := s.ownerless(ctx, sys.ID)
	switch {
	case err != nil:
		return nil, err
	case !ownerless:
		return nil, &AlreadyInstalledError{}
	}
	return sys, nil
}

func (s *Service) systemOrg(ctx context.Context, sys *org.Org, id uuid.UUID, name string, createdBy uuid.UUID) (*org.Org, error) {
	if sys != nil {
		return sys, nil
	}
	return s.d.Orgs.CreateSystem(ctx, org.CreateRequest{ID: id, Name: cmp.Or(strings.TrimSpace(name), defaultOrgName), CreatedBy: createdBy})
}

// NOTE: the system org's two projects: default, the first app's, and the console project with its client and upstream IdP seed.
func (s *Service) ensureProjects(ctx context.Context, orgID uuid.UUID) error {
	_, err := s.d.Projects.BySlug(ctx, orgID, project.DefaultSlug)
	if project.IsNotFoundError(err) {
		_, err = s.d.Projects.Create(ctx, project.CreateRequest{OrgID: orgID, Slug: project.DefaultSlug, Name: project.DefaultName})
	}
	if err != nil {
		return err
	}
	_, err = s.d.Projects.BySlug(ctx, orgID, project.SystemSlug)
	if !project.IsNotFoundError(err) {
		return err
	}
	p, err := s.d.Projects.CreateSystem(ctx, orgID)
	if err != nil {
		return err
	}
	if s.d.Seed != nil {
		if err := s.d.Seed(ctx, orgID, p.ID); err != nil {
			return err
		}
	}
	_, err = s.d.Clients.CreateSystem(ctx, orgID, p.ID, s.d.Console.RedirectURI, s.d.Console.PostLogoutURI)
	return err
}
