package grant

import (
	"context"
	"log/slog"
	"slices"
	"time"

	"github.com/google/uuid"

	"altalune.id/othra/apperror"
	"altalune.id/othra/internal/kernel/db"
	"altalune.id/othra/internal/kernel/session"
	"altalune.id/othra/internal/kernel/tenant"
	"altalune.id/othra/nanoid"
)

// Service issues authorization codes and exchanges them for rotating token grants.
type Service struct {
	store      Store
	uow        db.Transactor
	cfg        Config
	unexpected apperror.UnexpectedFunc
	now        func() time.Time
	newSecret  func() (string, error)
	log        *slog.Logger
}

// NewService builds the Service over store.
func NewService(store Store, uow db.Transactor, cfg Config, unexpected apperror.UnexpectedFunc) *Service {
	return &Service{
		store: store, uow: uow, cfg: cfg, unexpected: unexpected,
		now: func() time.Time { return time.Now().UTC() }, newSecret: session.NewID, log: slog.New(slog.DiscardHandler),
	}
}

// WithLogger sets where revocations are logged.
func (s *Service) WithLogger(log *slog.Logger) *Service {
	s.log = log
	return s
}

// WithClock sets the clock that issues and checks codes and tokens.
func (s *Service) WithClock(now func() time.Time) *Service {
	s.now = now
	return s
}

func scope(ctx context.Context, p Project) context.Context {
	return tenant.WithProject(tenant.WithOrg(ctx, p.OrgID), p.ID)
}

// CodeRequest is an authorization code for a signed-in account, named by public ids.
type CodeRequest struct {
	Client        string
	RedirectURI   string
	Scopes        []string
	CodeChallenge string
	Nonce         string
	UserSession   string
	EndUser       string
	AuthTime      time.Time
}

// IssueCode stores a code for req and returns its plaintext once; *SessionGoneError when the account or client is gone.
func (s *Service) IssueCode(ctx context.Context, p Project, req CodeRequest) (string, error) {
	ctx = scope(ctx, p)
	code, err := s.newSecret()
	if err != nil {
		return "", s.errOrReport(ctx, "grant: issue code", err)
	}
	now := s.now()
	err = s.store.InsertCode(ctx, p.ID, NewCode{
		ID: uuid.New(), Hash: nanoid.HashToken(code), Client: req.Client, UserSession: req.UserSession, EndUser: req.EndUser,
		RedirectURI: req.RedirectURI, Scopes: req.Scopes, CodeChallenge: req.CodeChallenge, Nonce: req.Nonce,
		AuthTime: req.AuthTime.UTC(), CreatedAt: now, ExpiresAt: now.Add(s.cfg.CodeTTL),
	})
	if err != nil {
		return "", s.errOrReport(ctx, "grant: issue code", err)
	}
	return code, nil
}

// ExchangeRequest redeems Code for the authenticated Client; RedirectURI is "" when the request left it out.
type ExchangeRequest struct {
	Client       Client
	Code         string
	RedirectURI  string
	CodeVerifier string
}

// Exchange redeems a code once and starts its grant family; a replay by its own client and verifier revokes the family it started.
func (s *Service) Exchange(ctx context.Context, p Project, req ExchangeRequest) (*Tokens, error) {
	ctx = scope(ctx, p)
	now := s.now()
	var out *Tokens
	var replayed string
	err := s.uow.RunInTx(ctx, func(ctx context.Context) error {
		c, err := s.store.CodeByHash(ctx, p.ID, nanoid.HashToken(req.Code))
		switch {
		case err != nil:
			return err
		case c == nil:
			return &InvalidGrantError{}
		case c.UsedAt != nil:
			if c.FamilyID == nil || c.ClientID != req.Client.ID || !verifyPKCE(c.CodeChallenge, req.CodeVerifier) {
				return nil
			}
			replayed = c.FamilySID
			return s.store.RevokeFamily(ctx, p.ID, *c.FamilyID, ReasonCodeReplay, now)
		case !now.Before(c.ExpiresAt), c.ClientID != req.Client.ID, c.UserSession == "",
			req.RedirectURI != "" && req.RedirectURI != c.RedirectURI:
			return &InvalidGrantError{}
		case c.CodeChallenge == "" && req.CodeVerifier != "":
			return &InvalidRequestError{}
		case !verifyPKCE(c.CodeChallenge, req.CodeVerifier):
			return &InvalidGrantError{}
		}
		sid, err := nanoid.New(publicIDLen)
		if err != nil {
			return err
		}
		f := &Family{
			ID: uuid.New(), PublicID: sid, ClientID: c.ClientID, EndUserID: c.EndUserID, UserSessionID: c.UserSessionID,
			UserSession: c.UserSession, Scopes: c.Scopes, AuthTime: c.AuthTime, CreatedAt: now, LastUsedAt: now,
			AbsoluteExpiresAt: now.Add(s.cfg.RefreshAbsolute),
		}
		if err := s.store.InsertFamily(ctx, p.OrgID, p.ID, f); err != nil {
			return err
		}
		if err := s.store.UseCode(ctx, p.ID, c.ID, f.ID, now); err != nil {
			return err
		}
		out, err = s.issue(ctx, p, f, c.Scopes, req.Client.Refresh && slices.Contains(c.Scopes, ScopeOfflineAccess), now)
		if out != nil {
			out.Nonce = c.Nonce
		}
		return err
	})
	if err != nil {
		return nil, s.errOrReport(ctx, "grant: exchange", err)
	}
	if out == nil {
		if replayed != "" {
			s.revoked(ctx, replayed, ReasonCodeReplay)
		}
		return nil, &InvalidGrantError{}
	}
	return out, nil
}

// RefreshRequest rotates RefreshToken for the authenticated Client; Scopes narrows this access token, nil keeps the grant's.
type RefreshRequest struct {
	Client       Client
	RefreshToken string
	Scopes       []string
}

// Refresh rotates a refresh token; a used token revokes its family, and so does another client presenting it.
func (s *Service) Refresh(ctx context.Context, p Project, req RefreshRequest) (*Tokens, error) {
	ctx = scope(ctx, p)
	now := s.now()
	var out *Tokens
	var revoked *Family
	var reason string
	err := s.uow.RunInTx(ctx, func(ctx context.Context) error {
		rt, f, err := s.store.RefreshByHash(ctx, p.ID, nanoid.HashToken(req.RefreshToken))
		switch {
		case err != nil:
			return err
		case rt == nil:
			return &InvalidGrantError{}
		case rt.UsedAt != nil, f.ClientID != req.Client.ID:
			reason = ReasonReuse
			if rt.UsedAt == nil {
				reason = ReasonClientMismatch
			}
			if f.RevokedAt != nil {
				return &InvalidGrantError{}
			}
			revoked = f
			return s.store.RevokeFamily(ctx, p.ID, f.ID, reason, now)
		case !f.Live(now), !now.Before(rt.ExpiresAt):
			return &InvalidGrantError{}
		case req.Scopes != nil && !subset(req.Scopes, f.Scopes):
			return &InvalidScopeError{}
		}
		if err := s.store.UseRefreshToken(ctx, p.ID, rt.ID, now); err != nil {
			return err
		}
		if err := s.store.TouchFamily(ctx, p.ID, f.ID, now); err != nil {
			return err
		}
		f.LastUsedAt = now
		scopes := f.Scopes
		if req.Scopes != nil {
			scopes = req.Scopes
		}
		out, err = s.issue(ctx, p, f, scopes, true, now)
		return err
	})
	if err != nil {
		return nil, s.errOrReport(ctx, "grant: refresh", err)
	}
	if out == nil {
		if revoked != nil {
			s.revoked(ctx, revoked.PublicID, reason)
		}
		return nil, &InvalidGrantError{}
	}
	return out, nil
}

func (s *Service) issue(ctx context.Context, p Project, f *Family, scopes []string, refresh bool, now time.Time) (*Tokens, error) {
	access, err := s.newSecret()
	if err != nil {
		return nil, err
	}
	out := &Tokens{Family: f, AccessToken: access, ExpiresAt: now.Add(s.cfg.AccessTokenTTL), Scopes: scopes}
	if err := s.store.InsertAccessToken(ctx, p.OrgID, p.ID, f.ID, nanoid.HashToken(access), scopes, now, out.ExpiresAt); err != nil {
		return nil, err
	}
	if !refresh {
		return out, nil
	}
	if out.RefreshToken, err = s.newSecret(); err != nil {
		return nil, err
	}
	expires := now.Add(s.cfg.RefreshIdle)
	if f.AbsoluteExpiresAt.Before(expires) {
		expires = f.AbsoluteExpiresAt
	}
	if err := s.store.InsertRefreshToken(ctx, p.OrgID, p.ID, f.ID, nanoid.HashToken(out.RefreshToken), now, expires); err != nil {
		return nil, err
	}
	return out, nil
}

// Access returns the live access token token with its grant, or *InvalidTokenError.
func (s *Service) Access(ctx context.Context, p Project, token string) (*Access, error) {
	ctx = scope(ctx, p)
	now := s.now()
	a, err := s.store.AccessByHash(ctx, p.ID, nanoid.HashToken(token))
	switch {
	case err != nil:
		return nil, s.errOrReport(ctx, "grant: access", err)
	case a == nil, !now.Before(a.ExpiresAt), !a.Family.Live(now):
		return nil, &InvalidTokenError{}
	}
	return a, nil
}

// Family returns the live grant family sid, or nil when it is unknown, revoked or expired.
func (s *Service) Family(ctx context.Context, p Project, sid string) (*Family, error) {
	ctx = scope(ctx, p)
	f, err := s.store.FamilyBySID(ctx, p.ID, sid)
	if err != nil {
		return nil, s.errOrReport(ctx, "grant: family", err)
	}
	if f == nil || !f.Live(s.now()) {
		return nil, nil
	}
	return f, nil
}

// AnyFamily returns the grant family sid whether live, revoked or expired, or nil when it is unknown or pruned.
func (s *Service) AnyFamily(ctx context.Context, p Project, sid string) (*Family, error) {
	ctx = scope(ctx, p)
	f, err := s.store.FamilyBySID(ctx, p.ID, sid)
	if err != nil {
		return nil, s.errOrReport(ctx, "grant: any family", err)
	}
	return f, nil
}

// RevokeFamily revokes the grant family sid for reason and returns it; an unknown or revoked sid is no error and returns nil.
func (s *Service) RevokeFamily(ctx context.Context, p Project, sid, reason string) (*Family, error) {
	ctx = scope(ctx, p)
	now := s.now()
	f, err := s.store.FamilyBySID(ctx, p.ID, sid)
	if err == nil && f != nil && f.RevokedAt == nil {
		err = s.store.RevokeFamily(ctx, p.ID, f.ID, reason, now)
	}
	if err != nil {
		return nil, s.errOrReport(ctx, "grant: revoke family", err)
	}
	if f == nil || f.RevokedAt != nil {
		return nil, nil
	}
	s.revoked(ctx, sid, reason)
	return f, nil
}

// RevokeUserSession revokes the user session's grants except offline_access ones (OIDC Core §11), and drops its unused codes.
func (s *Service) RevokeUserSession(ctx context.Context, p Project, userSession, reason string) error {
	ctx = scope(ctx, p)
	now := s.now()
	err := s.uow.RunInTx(ctx, func(ctx context.Context) error {
		return s.store.RevokeUserSession(ctx, p.ID, userSession, reason, now)
	})
	if err != nil {
		return s.errOrReport(ctx, "grant: revoke user session", err)
	}
	return nil
}

// Prune deletes the project's expired codes and tokens, used codes past a day, and families revoked a week ago or idle past refresh.
func (s *Service) Prune(ctx context.Context, orgID, projectID uuid.UUID) (int, error) {
	ctx = tenant.WithProject(tenant.WithOrg(ctx, orgID), projectID)
	now := s.now()
	n, err := s.store.Prune(ctx, projectID, Cutoffs{
		Now: now, UsedCodes: now.Add(-usedCodeKeep), Revoked: now.Add(-revokedKeep), Idle: now.Add(-s.cfg.RefreshIdle),
	})
	if err != nil {
		return 0, s.errOrReport(ctx, "grant: prune", err)
	}
	return n, nil
}

func (s *Service) revoked(ctx context.Context, sid, reason string) {
	s.log.LogAttrs(ctx, slog.LevelWarn, "grant revoked", slog.String("sid", sid), slog.String("reason", reason))
}

func (s *Service) errOrReport(ctx context.Context, message string, err error) error {
	if IsExpectedError(err) {
		return err
	}
	if _, ok := apperror.AsAppError(err); ok {
		return err
	}
	return s.unexpected(ctx, message, err)
}
