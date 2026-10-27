package boot

import (
	"context"
	"errors"
	"log/slog"

	"github.com/google/uuid"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"altalune.id/othra/internal/auth"
	"altalune.id/othra/internal/enduser"
	"altalune.id/othra/internal/kernel"
	"altalune.id/othra/internal/kernel/session"
	"altalune.id/othra/internal/org/member"
	"altalune.id/othra/internal/user"
	rootmcp "altalune.id/othra/mcp"
	"altalune.id/othra/worker"
)

// App is the wired runtime graph.
type App struct {
	Kernel     kernel.Kernel
	Services   *Services
	Supervisor *worker.Supervisor
	MCP        *rootmcp.Server

	shutdownTelemetry func(context.Context) error
	consoleSignIn     string
	log               *slog.Logger
}

// Run starts every supervised worker until ctx is cancelled or one fails.
func (a *App) Run(ctx context.Context) error { return a.Supervisor.Run(ctx) }

// RunMCP runs the MCP server over stdio until ctx is cancelled or the client disconnects.
func (a *App) RunMCP(ctx context.Context) error {
	return a.runMCP(ctx, &sdkmcp.StdioTransport{})
}

// SECURITY: stdio is trusted (whoever runs it holds the DB DSN), so it acts as the explicit system Principal, never as "no Principal".
func (a *App) runMCP(ctx context.Context, t sdkmcp.Transport) error {
	if a.MCP == nil {
		return errors.New("boot: mcp is disabled (set mcp.enabled)")
	}
	return a.MCP.SDK().Run(session.PrincipalInto(ctx, session.System()), t)
}

// SetPassword replaces the password of the user with email and signs out every session of theirs.
func (a *App) SetPassword(ctx context.Context, email, password string) error {
	return a.Services.Auth.SetPassword(ctx, email, password)
}

// SignInLink issues a one-time /p/system sign-in link for a console account, or the unclaimed owner seat, with email; never mailed.
func (a *App) SignInLink(ctx context.Context, email string) (enduser.IssuedLink, error) {
	sys, p, err := consoleProject(ctx, a.Services)
	switch {
	case err != nil:
		return enduser.IssuedLink{}, err
	case p == nil:
		return enduser.IssuedLink{}, errNotInstalled
	}
	to, err := linkTarget(ctx, a.Services.Users, consoleInvites{invites: a.Services.Invites}, sys.ID, email)
	if err != nil {
		return enduser.IssuedLink{}, err
	}
	l, err := a.Services.EndUsers.IssueLink(ctx, enduser.Project{
		OrgID: sys.ID, ID: p.ID, PublicID: p.PublicID, IssuerURL: a.Services.Projects.IssuerURL(p.IssuerHandle), EmailLink: p.SignInEmailLink,
	}, enduser.LinkRequest{Email: to.email, ReturnTo: a.consoleSignIn, Operator: true})
	if err != nil {
		return enduser.IssuedLink{}, err
	}
	// SECURITY: the link is a bearer secret, so the log names only its public id and the account, never the URL.
	a.log.Info("admin: printed a /p/system sign-in link", slog.String("link", l.PublicID), slog.String("user", to.user),
		slog.Bool("owner_seat", to.user == ""))
	return l, nil
}

type signInTarget struct {
	email string
	user  string
}

type accountsByEmail interface {
	FindByEmail(ctx context.Context, raw string) (*user.User, error)
}

type ownerSeats interface {
	OwnerSeat(ctx context.Context, orgID uuid.UUID, email string) (*auth.Seat, error)
}

// NOTE: an existing account, else the system org's unclaimed email-only owner seat, so an install with passwords and mail off can start.
func linkTarget(ctx context.Context, users accountsByEmail, seats ownerSeats, sysID uuid.UUID, email string) (signInTarget, error) {
	u, err := users.FindByEmail(ctx, email)
	if err == nil {
		return signInTarget{email: u.Email, user: u.PublicID}, nil
	}
	if !user.IsUnknownEmailError(err) {
		return signInTarget{}, err
	}
	seat, seatErr := seats.OwnerSeat(ctx, sysID, email)
	switch {
	case seatErr != nil:
		return signInTarget{}, seatErr
	case seat == nil:
		return signInTarget{}, err
	}
	return signInTarget{email: seat.Email}, nil
}

// AddMember makes the existing user with email a member of the org orgPublicID, acting as the operator.
func (a *App) AddMember(ctx context.Context, email, orgPublicID string, role member.Role) (*member.Member, error) {
	return a.Services.Members.AddByEmail(ctx, member.Requester{Operator: true}, email, orgPublicID, role)
}

// Close drains telemetry then closes the kernel's resources.
func (a *App) Close(ctx context.Context) error {
	err := a.shutdownTelemetry(ctx)
	if cerr := a.Kernel.Close(); cerr != nil && err == nil {
		err = cerr
	}
	return err
}
