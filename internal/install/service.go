package install

import (
	"cmp"
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"

	"altalune.id/othra/internal/kernel/db"
	"altalune.id/othra/internal/kernel/tenant"
	"altalune.id/othra/internal/legal"
	"altalune.id/othra/internal/org"
	"altalune.id/othra/internal/org/member"
	"altalune.id/othra/internal/project"
	"altalune.id/othra/internal/user"
)

const (
	lockName       = "othra.install"
	defaultOrgName = "Default"
)

// Service installs the instance and reports whether it still needs installing.
type Service struct {
	uow      db.Transactor
	users    UserCreator
	orgs     SystemOrgs
	members  Owners
	projects DefaultProjects
	terms    Terms
	seed     func(ctx context.Context, orgID, projectID uuid.UUID) error
}

// NewService builds a Service over the domain services it writes through.
func NewService(uow db.Transactor, users UserCreator, orgs SystemOrgs, members Owners, projects DefaultProjects) *Service {
	return &Service{uow: uow, users: users, orgs: orgs, members: members, projects: projects}
}

// WithProjectSeed runs seed once, on the project install creates, such as the upstream IdP from config (M7 F13).
func (s *Service) WithProjectSeed(seed func(ctx context.Context, orgID, projectID uuid.UUID) error) *Service {
	s.seed = seed
	return s
}

// WithTerms records the owner's acceptance of the terms through t.
func (s *Service) WithTerms(t Terms) *Service {
	s.terms = t
	return s
}

// Required reports whether the instance still needs installing: no system org, or in local mode no owner of it.
func (s *Service) Required(ctx context.Context, basic bool) (bool, error) {
	sys, err := s.orgs.System(ctx)
	if org.IsNotFoundError(err) {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	if basic {
		return false, nil
	}
	owners, err := s.members.CountOwners(ctx, sys.ID)
	if err != nil {
		return false, err
	}
	return owners == 0, nil
}

// Install creates the first owner and claims or creates the system org and its default project, all in one transaction.
func (s *Service) Install(ctx context.Context, r Request) (*user.User, error) {
	sysID, err := s.systemOrgID(ctx)
	if err != nil {
		return nil, err
	}
	var created *user.User
	err = s.uow.RunInTx(tenant.Into(ctx, tenant.Context{OrgID: sysID}), func(ctx context.Context) error {
		if err := db.LockInTx(ctx, lockName); err != nil {
			return err
		}
		sys, err := s.ownerlessSystemOrg(ctx, sysID)
		if err != nil {
			return err
		}
		u, err := s.users.Create(ctx, user.CreateRequest{Email: r.Email, Name: r.Name, Password: r.Password})
		if err != nil {
			return err
		}
		if r.AcceptTerms {
			if _, err := s.terms.Accept(ctx, u.ID, legal.SourceInstall); err != nil {
				return err
			}
		}
		if sys == nil {
			name := cmp.Or(strings.TrimSpace(r.OrgName), defaultOrgName)
			if sys, err = s.orgs.CreateSystem(ctx, org.CreateRequest{ID: sysID, Name: name, CreatedBy: u.ID}); err != nil {
				return err
			}
		}
		if _, err := s.members.Add(ctx, sys.ID, u.ID, member.RoleOwner); err != nil {
			return err
		}
		if err := s.ensureDefaultProject(ctx, sys.ID); err != nil {
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
	return s.uow.RunInTx(tenant.Into(ctx, tenant.Context{OrgID: sysID}), func(ctx context.Context) error {
		if err := db.LockInTx(ctx, lockName); err != nil {
			return err
		}
		sys, err := s.orgs.System(ctx)
		if org.IsNotFoundError(err) {
			sys, err = s.orgs.CreateSystem(ctx, org.CreateRequest{ID: sysID, Name: defaultOrgName})
		}
		if err != nil {
			return err
		}
		if sys.ID != sysID {
			return errSystemOrgMoved
		}
		return s.ensureDefaultProject(ctx, sys.ID)
	})
}

// SECURITY: the system org is scoped by its own id, chosen before the transaction opens; a new one gets a fresh id.
func (s *Service) systemOrgID(ctx context.Context) (uuid.UUID, error) {
	sys, err := s.orgs.System(ctx)
	switch {
	case org.IsNotFoundError(err):
		return uuid.New(), nil
	case err != nil:
		return uuid.Nil, err
	}
	return sys.ID, nil
}

func (s *Service) ownerlessSystemOrg(ctx context.Context, scoped uuid.UUID) (*org.Org, error) {
	sys, err := s.orgs.System(ctx)
	if org.IsNotFoundError(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if sys.ID != scoped {
		return nil, &AlreadyInstalledError{}
	}
	owners, err := s.members.CountOwners(ctx, sys.ID)
	if err != nil {
		return nil, err
	}
	if owners > 0 {
		return nil, &AlreadyInstalledError{}
	}
	return sys, nil
}

func (s *Service) ensureDefaultProject(ctx context.Context, orgID uuid.UUID) error {
	_, err := s.projects.BySlug(ctx, orgID, project.DefaultSlug)
	if err == nil || !project.IsNotFoundError(err) {
		return err
	}
	p, err := s.projects.Create(ctx, project.CreateRequest{OrgID: orgID, Slug: project.DefaultSlug, Name: project.DefaultName})
	if err != nil || s.seed == nil {
		return err
	}
	return s.seed(ctx, orgID, p.ID)
}
