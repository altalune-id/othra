package install

import (
	"context"
	"net/http"
	"time"

	"github.com/google/uuid"

	"altalune.id/othra/internal/auth"
	"altalune.id/othra/internal/client"
	"altalune.id/othra/internal/kernel/session"
	"altalune.id/othra/internal/legal"
	"altalune.id/othra/internal/org"
	"altalune.id/othra/internal/org/invite"
	"altalune.id/othra/internal/org/member"
	"altalune.id/othra/internal/project"
	"altalune.id/othra/internal/user"
)

// UserCreator creates the first owner.
type UserCreator interface {
	Create(ctx context.Context, req user.CreateRequest) (*user.User, error)
}

// Terms records that a person accepted the current terms; it joins the transaction on ctx.
type Terms interface {
	Accept(ctx context.Context, userID uuid.UUID, source legal.Source) (time.Time, error)
}

// SystemOrgs finds or creates the deployment's system org.
type SystemOrgs interface {
	System(ctx context.Context) (*org.Org, error)
	CreateSystem(ctx context.Context, req org.CreateRequest) (*org.Org, error)
}

// Owners adds the owner membership and counts owners.
type Owners interface {
	Add(ctx context.Context, orgID, userID uuid.UUID, role member.Role) (*member.Membership, error)
	CountOwners(ctx context.Context, orgID uuid.UUID) (int, error)
}

// Projects finds or creates the system org's default and console projects.
type Projects interface {
	BySlug(ctx context.Context, orgID uuid.UUID, slug string) (*project.Project, error)
	Create(ctx context.Context, req project.CreateRequest) (*project.Project, error)
	CreateSystem(ctx context.Context, orgID uuid.UUID) (*project.Project, error)
}

// ConsoleClients seeds the console project's OAuth client.
type ConsoleClients interface {
	CreateSystem(ctx context.Context, orgID, projectID uuid.UUID, redirectURI, postLogoutURI string) (*client.Client, error)
}

// OwnerInvites invites the first owner by email when console passwords are off; the first verified sign-in claims it.
type OwnerInvites interface {
	InviteOwner(ctx context.Context, orgID uuid.UUID, email string) (*invite.Invite, error)
	PendingOwner(ctx context.Context, orgID uuid.UUID) (bool, error)
}

// SignIn signs the new owner in after install.
type SignIn interface {
	LoginLocal(ctx context.Context, c auth.Credentials) (session.Principal, error)
}

// SessionStarter starts the new owner's browser session.
type SessionStarter interface {
	Start(ctx context.Context, w http.ResponseWriter, p session.Principal) (string, time.Time, error)
}
