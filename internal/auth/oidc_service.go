package auth

import (
	"context"
	"strings"

	"altalune.id/othra/internal/kernel/session"
	"altalune.id/othra/internal/kernel/tenant"
	"altalune.id/othra/internal/org"
	"altalune.id/othra/internal/user"
)

// ConsoleIssuer is the issuer a console account's /p/system link is stored under; the issuer URL follows http.baseURL.
const ConsoleIssuer = "othra:system"

// OIDCSignIn is a /p/system sign-in whose ID token the console relying party already validated.
type OIDCSignIn struct {
	Subject       string
	Email         string
	EmailVerified bool
	Name          string
	IDToken       string
	// Invite is the public id of the invite the sign-in started from, or empty.
	Invite string
}

// LoginOIDC turns a /p/system sign-in into the console Principal: the linked account, else one email links once, else an invite.
func (s *Service) LoginOIDC(ctx context.Context, in OIDCSignIn) (session.Principal, error) {
	switch {
	case in.Subject == "":
		return session.Principal{}, &NoConsoleAccountError{Reason: NoConsoleNotInvited}
	case !in.EmailVerified:
		return session.Principal{}, &NoConsoleAccountError{Reason: NoConsoleUnverified}
	}
	u, err := s.consoleUser(ctx, in)
	if err != nil {
		return session.Principal{}, err
	}
	if !s.deps.OpenSignup {
		ms, err := s.deps.Members.ListByUser(ctx, u.ID)
		if err != nil {
			return session.Principal{}, err
		}
		if len(ms) == 0 {
			return session.Principal{}, &NoConsoleAccountError{Reason: NoConsoleRemoved}
		}
	}
	p := principalOf(u, s.now())
	p.Source, p.IDPIssuer, p.IDPSubject, p.IDToken = session.SourceOIDC, ConsoleIssuer, in.Subject, in.IDToken
	return s.Refresh(ctx, p)
}

// SECURITY: an email links only an unlinked account, once; a link is never re-pointed, so a console account can't be taken over.
func (s *Service) consoleUser(ctx context.Context, in OIDCSignIn) (*user.User, error) {
	if u, err := s.existing(ctx, in); err != nil || u != nil {
		return u, err
	}
	seat, err := s.seat(ctx, in)
	if err != nil {
		return nil, err
	}
	if seat == nil && !s.deps.OpenSignup {
		return nil, &NoConsoleAccountError{Reason: NoConsoleNotInvited}
	}
	u, err := s.join(ctx, in, seat)
	if !user.IsAlreadyExistsError(err) {
		return u, err
	}
	if u, err = s.existing(ctx, in); err != nil || u != nil {
		return u, err
	}
	return nil, &IdentityConflictError{}
}

// NOTE: the account linked to this sub, else the unlinked one with this email, now linked; nil when neither exists.
func (s *Service) existing(ctx context.Context, in OIDCSignIn) (*user.User, error) {
	ids := s.deps.Identities
	u, err := ids.ByIDP(ctx, ConsoleIssuer, in.Subject)
	switch {
	case err == nil:
		return u, s.claim(ctx, in, u)
	case !user.IsNotFoundError(err):
		return nil, err
	}
	u, err = s.deps.Users.ByEmail(ctx, in.Email)
	switch {
	case user.IsNotFoundError(err):
		return nil, nil
	case err != nil:
		return nil, err
	}
	linked, err := ids.LinkIDP(ctx, u.ID, ConsoleIssuer, in.Subject)
	if err != nil {
		return nil, err
	}
	if !linked {
		return nil, &IdentityConflictError{}
	}
	return u, s.claim(ctx, in, u)
}

// InviteMismatch reports whether the open invite with public id invite is for another email than email.
func (s *Service) InviteMismatch(ctx context.Context, invite, email string) (bool, error) {
	seat, err := s.deps.Invites.Seat(ctx, invite)
	if err != nil || seat == nil {
		return false, err
	}
	return !strings.EqualFold(seat.Email, email), nil
}

// NOTE: the invite the sign-in started from, else the system org's email-only owner seat; only one for this very email counts.
func (s *Service) seat(ctx context.Context, in OIDCSignIn) (*Seat, error) {
	var seat *Seat
	var err error
	if in.Invite != "" {
		seat, err = s.deps.Invites.Seat(ctx, in.Invite)
	} else {
		var sys *org.Org
		if sys, err = s.deps.Orgs.System(ctx); err != nil {
			return nil, err
		}
		seat, err = s.deps.Invites.OwnerSeat(ctx, sys.ID, in.Email)
	}
	if err != nil || seat == nil || !strings.EqualFold(seat.Email, in.Email) {
		return nil, err
	}
	return seat, nil
}

func (s *Service) join(ctx context.Context, in OIDCSignIn, seat *Seat) (*user.User, error) {
	if seat == nil {
		return s.deps.Identities.CreateLinked(ctx, in.Email, in.Name, ConsoleIssuer, in.Subject)
	}
	var u *user.User
	err := s.deps.Transactor.RunInTx(tenant.Into(ctx, tenant.Context{OrgID: seat.OrgID}), func(ctx context.Context) error {
		var err error
		if u, err = s.deps.Identities.CreateLinked(ctx, in.Email, in.Name, ConsoleIssuer, in.Subject); err != nil {
			return err
		}
		claimed, err := s.deps.Invites.Claim(ctx, seat, u.ID)
		if err == nil && !claimed {
			err = &NoConsoleAccountError{Reason: NoConsoleNotInvited}
		}
		return err
	})
	return u, err
}

// NOTE: an existing account that started from an invite joins that org too; a used, expired or someone else's invite is skipped.
func (s *Service) claim(ctx context.Context, in OIDCSignIn, u *user.User) error {
	if in.Invite == "" {
		return nil
	}
	seat, err := s.seat(ctx, in)
	if err != nil || seat == nil {
		return err
	}
	_, err = s.deps.Invites.Claim(ctx, seat, u.ID)
	return err
}
