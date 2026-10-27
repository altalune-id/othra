package enduser

import (
	"context"
	"errors"
	"log/slog"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/altalune-id/othra/apperror"
	"github.com/altalune-id/othra/emailaddr"
	"github.com/altalune-id/othra/internal/kernel/db"
	ksecret "github.com/altalune-id/othra/internal/kernel/secret"
	"github.com/altalune-id/othra/internal/kernel/session"
	"github.com/altalune-id/othra/internal/kernel/tenant"
	"github.com/altalune-id/othra/nanoid"
)

const touchEvery = time.Minute

// Service signs end users in by magic link and keeps their browser and user sessions and terms consent.
type Service struct {
	store      Store
	uow        db.Transactor
	mail       MailQueue
	cfg        Config
	unexpected apperror.UnexpectedFunc
	now        func() time.Time
	newSecret  func() (string, error)
	log        *slog.Logger
	signIn     SignInSwitch
	grants     Grants
}

// NewService builds the Service over store; mail enqueues sign-in emails in the same unit of work as their link.
func NewService(
	store Store, uow db.Transactor, mail MailQueue, grants Grants, signIn SignInSwitch, cfg Config, unexpected apperror.UnexpectedFunc,
) *Service {
	return &Service{
		store: store, uow: uow, mail: mail, grants: grants, signIn: signIn, cfg: cfg, unexpected: unexpected,
		now: func() time.Time { return time.Now().UTC() }, newSecret: session.NewID, log: slog.New(slog.DiscardHandler),
	}
}

// WithLogger sets where sign-ins and terms acceptances are logged.
func (s *Service) WithLogger(log *slog.Logger) *Service {
	s.log = log
	return s
}

// WithClock sets the clock that issues and checks links and sessions.
func (s *Service) WithClock(now func() time.Time) *Service {
	s.now = now
	return s
}

func scope(ctx context.Context, p Project) context.Context {
	return tenant.WithProject(tenant.WithOrg(ctx, p.OrgID), p.ID)
}

// IssueLink returns a link for req without mailing it, or nothing inside the cooldown. NOTE: the caller checks req.ReturnTo.
func (s *Service) IssueLink(ctx context.Context, p Project, req LinkRequest) (IssuedLink, error) {
	return s.issue(ctx, p, req, nil)
}

// SendLink issues a link for req and enqueues the email compose builds, in one unit of work; inside the cooldown it sends nothing.
func (s *Service) SendLink(ctx context.Context, p Project, req LinkRequest, compose ComposeFunc) (IssuedLink, error) {
	return s.issue(ctx, p, req, func(ctx context.Context, l IssuedLink) error {
		m, err := compose(ctx, l)
		if err != nil {
			return err
		}
		_, err = s.mail.Enqueue(ctx, m)
		return err
	})
}

func (s *Service) issue(
	ctx context.Context, p Project, req LinkRequest, after func(context.Context, IssuedLink) error,
) (IssuedLink, error) {
	if !p.EmailLink && req.Pending == nil {
		return IssuedLink{}, &EmailDisabledError{}
	}
	email, err := emailaddr.Normalize(req.Email)
	if err != nil {
		return IssuedLink{}, &InvalidEmailError{}
	}
	ctx = scope(ctx, p)
	now := s.now()
	out := IssuedLink{Email: email}
	var allowErr error
	err = s.uow.RunInTx(ctx, func(ctx context.Context) error {
		since := now.Add(-s.cfg.ResendAfter)
		if req.Pending != nil || req.Operator {
			since = now
		}
		recent, err := s.store.HasRecentLink(ctx, p.ID, email, since)
		if err != nil || recent {
			return err
		}
		if req.Allow != nil {
			ok, err := req.Allow(ctx)
			if err != nil {
				allowErr = err
				return err
			}
			if !ok {
				return nil
			}
		}
		// NOTE: only links outside the cooldown go, so a concurrent request's fresh link stays and the one-live index refuses this one.
		if err := s.store.DeleteUnusedLinks(ctx, p.ID, email, since); err != nil {
			return err
		}
		token, err := s.newSecret()
		if err != nil {
			return err
		}
		publicID, err := nanoid.New(publicIDLen)
		if err != nil {
			return err
		}
		l := &Link{
			ID: uuid.New(), PublicID: publicID, Email: email, ReturnTo: req.ReturnTo, CreatedAt: now, ExpiresAt: now.Add(s.cfg.LinkTTL),
			Pending: req.Pending,
		}
		if err := s.store.InsertLink(ctx, p.OrgID, p.ID, l, ksecret.Hex(token), req.IP); err != nil {
			return err
		}
		known, err := s.store.EndUserExists(ctx, p.ID, email)
		if err != nil {
			return err
		}
		out = IssuedLink{
			PublicID: publicID, URL: p.IssuerURL + "/login/verify?t=" + url.QueryEscape(token), Email: email, ExpiresAt: l.ExpiresAt,
			Issued: true, SignUp: !known,
		}
		if after == nil {
			return nil
		}
		return after(ctx, out)
	})
	if allowErr != nil {
		return IssuedLink{}, allowErr
	}
	if errors.Is(err, ErrLiveLink) {
		return IssuedLink{Email: email}, nil
	}
	if err != nil {
		return IssuedLink{}, s.errOrReport(ctx, "enduser: issue link", err)
	}
	return out, nil
}

// LinkByToken returns the usable link a raw token names, or *LinkNotUsableError.
func (s *Service) LinkByToken(ctx context.Context, p Project, token string) (*Link, error) {
	if token == "" {
		return nil, &LinkNotUsableError{}
	}
	ctx = scope(ctx, p)
	l, err := s.store.LinkByTokenHash(ctx, p.ID, ksecret.Hex(token))
	return s.usable(ctx, l, err)
}

// LinkByPublicID returns the usable link publicID, or *LinkNotUsableError.
func (s *Service) LinkByPublicID(ctx context.Context, p Project, publicID string) (*Link, error) {
	ctx = scope(ctx, p)
	l, err := s.store.LinkByPublicID(ctx, p.ID, publicID)
	return s.usable(ctx, l, err)
}

func (s *Service) usable(ctx context.Context, l *Link, err error) (*Link, error) {
	if err != nil {
		return nil, s.errOrReport(ctx, "enduser: link", err)
	}
	if !l.Usable(s.now()) {
		return nil, &LinkNotUsableError{}
	}
	return l, nil
}

// Redeem uses the link once and signs its email in on the browser, creating the end user, browser and user session as needed.
func (s *Service) Redeem(ctx context.Context, p Project, req RedeemRequest) (*Redeemed, error) {
	if req.Token == "" {
		return nil, &LinkNotUsableError{}
	}
	ctx = scope(ctx, p)
	now := s.now()
	hash := ksecret.Hex(req.Token)
	var out *Redeemed
	repeated := false
	err := s.uow.RunInTx(ctx, func(ctx context.Context) error {
		current, err := s.browser(ctx, p, req.BrowserSecret, now)
		if err != nil {
			return err
		}
		if current != nil {
			if repeat, err := s.repeatRedeem(ctx, p, hash, req.BrowserSecret, current); err != nil || repeat != nil {
				out, repeated = repeat, repeat != nil
				return err
			}
		}
		secret, b, err := s.startBrowser(ctx, p, current, req.UserAgent, now)
		if err != nil {
			return err
		}
		l, err := s.store.UseLink(ctx, p.ID, hash, b.ID, now)
		if err != nil {
			return err
		}
		pending := req.Confirm != nil && l.Pending != nil
		known, gated, err := s.signUpGated(ctx, p, l.Email)
		if err != nil {
			return err
		}
		if gated && !pending {
			return &EmailDisabledError{}
		}
		endUserID, err := s.store.SaveEndUser(ctx, p.OrgID, p.ID, l.Email, now)
		if err != nil {
			return err
		}
		if pending {
			linked, err := req.Confirm(ctx, l, endUserID, !known)
			if err != nil {
				return err
			}
			if gated && !linked {
				return &EmailDisabledError{}
			}
		}
		out, err = s.signInBrowser(ctx, p, secret, b, endUserID, MethodMagicLink, now)
		if out != nil {
			out.ReturnTo = l.ReturnTo
		}
		return err
	})
	if err != nil {
		return nil, s.errOrReport(ctx, "enduser: redeem", err)
	}
	s.log.LogAttrs(ctx, slog.LevelInfo, "enduser: signed in", slog.String("project", p.PublicID),
		slog.String("end_user", out.Account.Subject), slog.String("user_session", out.Account.PublicID),
		slog.String("method", MethodMagicLink), slog.Bool("repeat", repeated))
	return out, nil
}

// SECURITY: with the email link off, a redeem creates an end user only if it links the pending Google identity it carries.
func (s *Service) signUpGated(ctx context.Context, p Project, email string) (known, gated bool, err error) {
	_, known, err = s.store.EndUserIDByEmail(ctx, p.ID, email)
	if err != nil || known {
		return known, false, err
	}
	on, err := s.signIn.LockEmailLink(ctx, p.ID)
	if err != nil {
		return false, false, err
	}
	return false, !on, nil
}

func (s *Service) signInBrowser(
	ctx context.Context, p Project, secret string, b *Browser, endUserID uuid.UUID, method string, now time.Time,
) (*Redeemed, error) {
	if err := s.store.SignIn(ctx, p.OrgID, p.ID, b.ID, endUserID, method, now); err != nil {
		return nil, err
	}
	if err := s.store.KeepNewest(ctx, p.ID, b.ID, s.cfg.MaxAccounts, now); err != nil {
		return nil, err
	}
	accounts, err := s.store.Accounts(ctx, p.ID, b.ID)
	if err != nil {
		return nil, err
	}
	if len(accounts) == 0 {
		return nil, &SessionGoneError{}
	}
	return &Redeemed{Secret: secret, ExpiresAt: b.ExpiresAt(s.cfg.Idle), Account: accounts[0]}, nil
}

// FederatedSignIn signs EndUserID in on the browser of BrowserSecret after an upstream IdP vouched for them; Method is the upstream kind.
type FederatedSignIn struct {
	EndUserID     uuid.UUID
	Method        string
	BrowserSecret string
	UserAgent     string
}

// SignInFederated starts the user session the way Redeem does, in the unit of work on ctx, rotating the browser secret.
func (s *Service) SignInFederated(ctx context.Context, p Project, req FederatedSignIn) (*Redeemed, error) {
	ctx = scope(ctx, p)
	now := s.now()
	var out *Redeemed
	err := s.uow.RunInTx(ctx, func(ctx context.Context) error {
		current, err := s.browser(ctx, p, req.BrowserSecret, now)
		if err != nil {
			return err
		}
		secret, b, err := s.startBrowser(ctx, p, current, req.UserAgent, now)
		if err != nil {
			return err
		}
		out, err = s.signInBrowser(ctx, p, secret, b, req.EndUserID, req.Method, now)
		return err
	})
	if err != nil {
		return nil, s.errOrReport(ctx, "enduser: federated sign-in", err)
	}
	s.log.LogAttrs(ctx, slog.LevelInfo, "enduser: signed in", slog.String("project", p.PublicID),
		slog.String("end_user", out.Account.Subject), slog.String("user_session", out.Account.PublicID), slog.String("method", req.Method))
	return out, nil
}

// EndUserIDByEmail returns the id of the project's end user with email; ok is false when there is none.
func (s *Service) EndUserIDByEmail(ctx context.Context, p Project, email string) (uuid.UUID, bool, error) {
	ctx = scope(ctx, p)
	id, ok, err := s.store.EndUserIDByEmail(ctx, p.ID, email)
	if err != nil {
		return uuid.Nil, false, s.errOrReport(ctx, "enduser: by email", err)
	}
	return id, ok, nil
}

// CreateFederated creates a verified end user from an upstream IdP's verified email, or returns ErrEmailTaken.
func (s *Service) CreateFederated(ctx context.Context, p Project, email, name string) (uuid.UUID, error) {
	ctx = scope(ctx, p)
	id, err := s.store.CreateEndUser(ctx, p.OrgID, p.ID, email, clip(name, nameMax), s.now())
	if errors.Is(err, ErrEmailTaken) {
		return uuid.Nil, err
	}
	if err != nil {
		return uuid.Nil, s.errOrReport(ctx, "enduser: create federated", err)
	}
	return id, nil
}

// FillName gives the end user an upstream IdP's name, in the unit of work on ctx, only while they have none.
func (s *Service) FillName(ctx context.Context, p Project, endUserID uuid.UUID, name string) error {
	name = clip(strings.TrimSpace(name), nameMax)
	if name == "" {
		return nil
	}
	ctx = scope(ctx, p)
	if err := s.store.FillName(ctx, p.ID, endUserID, name, s.now()); err != nil {
		return s.errOrReport(ctx, "enduser: fill name", err)
	}
	return nil
}

// NOTE: a second Continue from the browser that already redeemed the link signs nobody new in but succeeds.
func (s *Service) repeatRedeem(ctx context.Context, p Project, hash, secret string, b *Browser) (*Redeemed, error) {
	l, err := s.store.LinkByTokenHash(ctx, p.ID, hash)
	if IsLinkNotUsableError(err) {
		return nil, nil
	}
	if err != nil || l.UsedAt == nil || l.BrowserSessionID == nil || *l.BrowserSessionID != b.ID {
		return nil, err
	}
	accounts, err := s.store.Accounts(ctx, p.ID, b.ID)
	if err != nil {
		return nil, err
	}
	for _, a := range accounts {
		if a.Email == l.Email {
			return &Redeemed{Secret: secret, ExpiresAt: b.ExpiresAt(s.cfg.Idle), Account: a, ReturnTo: l.ReturnTo}, nil
		}
	}
	return nil, nil
}

// SECURITY: every sign-in gets a new cookie secret, so a secret planted before sign-in never names the signed-in browser.
func (s *Service) startBrowser(ctx context.Context, p Project, b *Browser, userAgent string, now time.Time) (string, *Browser, error) {
	secret, err := s.newSecret()
	if err != nil {
		return "", nil, err
	}
	if b != nil {
		if err := s.store.RotateBrowser(ctx, p.ID, b.ID, ksecret.Hex(secret), now); err != nil {
			return "", nil, err
		}
		b.LastSeenAt = now
		return secret, b, nil
	}
	b = &Browser{ID: uuid.New(), CreatedAt: now, LastSeenAt: now, AbsoluteEnd: now.Add(s.cfg.Absolute)}
	if err := s.store.InsertBrowser(ctx, p.OrgID, p.ID, b, ksecret.Hex(secret), clip(userAgent, userAgentMax)); err != nil {
		return "", nil, err
	}
	return secret, b, nil
}

func (s *Service) browser(ctx context.Context, p Project, secret string, now time.Time) (*Browser, error) {
	if secret == "" {
		return nil, nil
	}
	b, err := s.store.BrowserBySecretHash(ctx, p.ID, ksecret.Hex(secret))
	if err != nil || b == nil || !b.Live(now, s.cfg.Idle) {
		return nil, err
	}
	return b, nil
}

// Session returns the browser's live accounts, or nil when the secret names no live browser session; /authorize calls it too.
func (s *Service) Session(ctx context.Context, p Project, secret string) (*Session, error) {
	ctx = scope(ctx, p)
	now := s.now()
	b, err := s.browser(ctx, p, secret, now)
	if err != nil {
		return nil, s.errOrReport(ctx, "enduser: session", err)
	}
	if b == nil {
		return nil, nil
	}
	if now.Sub(b.LastSeenAt) >= touchEvery {
		if err := s.store.TouchBrowser(ctx, p.ID, b.ID, now); err != nil {
			return nil, s.errOrReport(ctx, "enduser: session", err)
		}
		b.LastSeenAt = now
	}
	accounts, err := s.store.Accounts(ctx, p.ID, b.ID)
	if err != nil {
		return nil, s.errOrReport(ctx, "enduser: session", err)
	}
	return &Session{BrowserSessionID: b.ID, ExpiresAt: b.ExpiresAt(s.cfg.Idle), Accounts: accounts}, nil
}

// Select makes the account whose user session is publicID the browser's active one, or returns *SessionGoneError.
func (s *Service) Select(ctx context.Context, p Project, secret, publicID string) (*Account, error) {
	ctx = scope(ctx, p)
	now := s.now()
	var out *Account
	err := s.uow.RunInTx(ctx, func(ctx context.Context) error {
		b, err := s.browser(ctx, p, secret, now)
		if err != nil {
			return err
		}
		if b == nil {
			return &SessionGoneError{}
		}
		live, err := s.store.Select(ctx, p.ID, b.ID, publicID, now)
		if err != nil {
			return err
		}
		if !live {
			return &SessionGoneError{}
		}
		accounts, err := s.store.Accounts(ctx, p.ID, b.ID)
		if err != nil {
			return err
		}
		sess := &Session{Accounts: accounts}
		a, ok := sess.Account(publicID)
		if !ok {
			return &SessionGoneError{}
		}
		out = a
		return nil
	})
	if err != nil {
		return nil, s.errOrReport(ctx, "enduser: select", err)
	}
	return out, nil
}

// SignOut ends the one user session publicID on this browser; the browser's other accounts stay signed in.
func (s *Service) SignOut(ctx context.Context, p Project, secret, publicID string) error {
	ctx = scope(ctx, p)
	now := s.now()
	err := s.uow.RunInTx(ctx, func(ctx context.Context) error {
		b, err := s.browser(ctx, p, secret, now)
		if err != nil || b == nil {
			return err
		}
		if err := s.store.EndUserSession(ctx, p.ID, b.ID, publicID, now); err != nil {
			return err
		}
		return s.revoke(ctx, p, publicID, EndSignOut)
	})
	if err != nil {
		return s.errOrReport(ctx, "enduser: sign out", err)
	}
	return nil
}

// NOTE: every way a user session ends revokes its grants in the same unit of work; Grants keeps offline_access ones.
func (s *Service) revoke(ctx context.Context, p Project, publicID string, reason EndReason) error {
	return s.grants.RevokeUserSession(ctx, p, publicID, reason)
}

// EndUserSession ends the end user's live session publicID with its grants in one unit of work, or returns *SessionNotFoundError.
func (s *Service) EndUserSession(ctx context.Context, p Project, endUserID uuid.UUID, publicID string) error {
	ctx = scope(ctx, p)
	err := s.uow.RunInTx(ctx, func(ctx context.Context) error { return s.endOwned(ctx, p, endUserID, publicID) })
	if err != nil {
		return s.errOrReport(ctx, "enduser: end user session", err)
	}
	s.log.LogAttrs(ctx, slog.LevelInfo, "enduser: session ended", slog.String("project", p.PublicID), slog.String("user_session", publicID))
	return nil
}

func (s *Service) endOwned(ctx context.Context, p Project, endUserID uuid.UUID, publicID string) error {
	ended, err := s.store.EndOwnedSession(ctx, p.ID, endUserID, publicID, s.now())
	switch {
	case err != nil:
		return err
	case !ended:
		return &SessionNotFoundError{}
	}
	return s.revoke(ctx, p, publicID, EndSignOut)
}

// EndOtherSessions ends every live user session of the end user but keep, with their grants, in one unit of work, and counts them.
func (s *Service) EndOtherSessions(ctx context.Context, p Project, endUserID uuid.UUID, keep string) (int, error) {
	ctx = scope(ctx, p)
	ended := 0
	err := s.uow.RunInTx(ctx, func(ctx context.Context) error {
		live, err := s.store.LiveSessions(ctx, p.ID, endUserID)
		if err != nil {
			return err
		}
		for _, ls := range live {
			if ls.PublicID == keep {
				continue
			}
			if err := s.endOwned(ctx, p, endUserID, ls.PublicID); err != nil && !IsSessionNotFoundError(err) {
				return err
			}
			ended++
		}
		return nil
	})
	if err != nil {
		return 0, s.errOrReport(ctx, "enduser: end other sessions", err)
	}
	s.log.LogAttrs(ctx, slog.LevelInfo, "enduser: other sessions ended", slog.String("project", p.PublicID), slog.Int("ended", ended))
	return ended, nil
}

// Sessions returns the end user's live user sessions on every browser, the most recently seen first.
func (s *Service) Sessions(ctx context.Context, p Project, endUserID uuid.UUID) ([]LiveSession, error) {
	ctx = scope(ctx, p)
	out, err := s.store.LiveSessions(ctx, p.ID, endUserID)
	if err != nil {
		return nil, s.errOrReport(ctx, "enduser: sessions", err)
	}
	return out, nil
}

// Profile returns the end user, or *EndUserNotFoundError when they are gone.
func (s *Service) Profile(ctx context.Context, p Project, endUserID uuid.UUID) (*EndUser, error) {
	u, err := s.EndUser(ctx, p, endUserID)
	switch {
	case err != nil:
		return nil, err
	case u == nil:
		return nil, &EndUserNotFoundError{}
	}
	return u, nil
}

// ByPublicID returns the project's end user publicID, or *EndUserNotFoundError.
func (s *Service) ByPublicID(ctx context.Context, p Project, publicID string) (*EndUser, error) {
	ctx = scope(ctx, p)
	u, err := s.store.EndUserByPublicID(ctx, p.ID, publicID)
	switch {
	case err != nil:
		return nil, s.errOrReport(ctx, "enduser: by public id", err)
	case u == nil:
		return nil, &EndUserNotFoundError{}
	}
	return u, nil
}

// UpdateName sets the name the end user goes by, trimmed and clipped; an empty name clears it.
func (s *Service) UpdateName(ctx context.Context, p Project, endUserID uuid.UUID, name string) (*EndUser, error) {
	ctx = scope(ctx, p)
	found, err := s.store.UpdateName(ctx, p.ID, endUserID, clip(strings.TrimSpace(name), nameMax), s.now())
	switch {
	case err != nil:
		return nil, s.errOrReport(ctx, "enduser: update name", err)
	case !found:
		return nil, &EndUserNotFoundError{}
	}
	u, err := s.Profile(ctx, p, endUserID)
	if err != nil {
		return nil, err
	}
	s.log.LogAttrs(ctx, slog.LevelInfo, "enduser: profile updated", slog.String("project", p.PublicID), slog.String("end_user", u.Subject))
	return u, nil
}

// ListEndUsers pages the project's end users newest first by email prefix query after the end user after; next is set on a full page.
func (s *Service) ListEndUsers(ctx context.Context, p Project, query, after string, limit int) (users []*EndUser, next string, err error) {
	ctx = scope(ctx, p)
	if limit <= 0 || limit > MaxListLimit {
		limit = MaxListLimit
	}
	users, err = s.store.ListEndUsers(ctx, p.ID, strings.ToLower(strings.TrimSpace(query)), after, limit)
	if err != nil {
		return nil, "", s.errOrReport(ctx, "enduser: list", err)
	}
	if len(users) == limit {
		next = users[len(users)-1].Subject
	}
	return users, next, nil
}

// EndUser returns the project's end user id for an ID token or userinfo, or nil when it is gone.
func (s *Service) EndUser(ctx context.Context, p Project, id uuid.UUID) (*EndUser, error) {
	ctx = scope(ctx, p)
	u, err := s.store.EndUserByID(ctx, p.ID, id)
	if err != nil {
		return nil, s.errOrReport(ctx, "enduser: end user", err)
	}
	return u, nil
}

// EndSession ends the user session publicID on any browser with its grants, for an RP-initiated logout; an ended one is no error.
func (s *Service) EndSession(ctx context.Context, p Project, publicID string) error {
	ctx = scope(ctx, p)
	err := s.uow.RunInTx(ctx, func(ctx context.Context) error {
		if err := s.store.EndUserSessionAnywhere(ctx, p.ID, publicID, s.now()); err != nil {
			return err
		}
		return s.revoke(ctx, p, publicID, EndLogout)
	})
	if err != nil {
		return s.errOrReport(ctx, "enduser: end session", err)
	}
	return nil
}

// NeedsTerms reports whether a owes the project's current terms and whether it accepted an older one; /authorize calls it too.
func (s *Service) NeedsTerms(ctx context.Context, p Project, a Account) (needed, updated bool, err error) {
	if !p.TermsRequired() {
		return false, false, nil
	}
	ctx = scope(ctx, p)
	latest, err := s.store.LatestTerms(ctx, p.ID, a.EndUserID)
	if err != nil {
		return false, false, s.errOrReport(ctx, "enduser: terms", err)
	}
	return latest < p.TermsVersion, latest > 0 && latest < p.TermsVersion, nil
}

// AcceptTerms records that the account of user session publicID on this browser accepted terms version seen, or returns *TermsChangedError.
func (s *Service) AcceptTerms(ctx context.Context, p Project, secret, publicID string, seen int, ip, userAgent string) (*Account, error) {
	if p.TermsRequired() && seen != p.TermsVersion {
		return nil, &TermsChangedError{Seen: seen, Current: p.TermsVersion}
	}
	sess, err := s.Session(ctx, p, secret)
	if err != nil {
		return nil, err
	}
	a, ok := sess.Account(publicID)
	if !ok {
		return nil, &SessionGoneError{}
	}
	if !p.TermsRequired() {
		return a, nil
	}
	ctx = scope(ctx, p)
	err = s.store.InsertTerms(ctx, p.OrgID, p.ID, TermsAcceptance{
		EndUserID: a.EndUserID, Version: p.TermsVersion, TermsURL: p.TermsURL, PrivacyURL: p.PrivacyURL,
		AcceptedAt: s.now(), UserSessionID: a.UserSessionID, UserSessionPublicID: a.PublicID, IP: ip,
		UserAgent: clip(userAgent, userAgentMax),
	})
	if err != nil {
		return nil, s.errOrReport(ctx, "enduser: accept terms", err)
	}
	s.log.LogAttrs(ctx, slog.LevelInfo, "enduser: accepted terms", slog.String("project", p.PublicID),
		slog.String("end_user", a.Subject), slog.String("user_session", a.PublicID), slog.Int("version", p.TermsVersion))
	return a, nil
}

// Prune deletes a project's expired links and sessions; the scheduler runs it with ctx already scoped.
func (s *Service) Prune(ctx context.Context, orgID, projectID uuid.UUID) (int, error) {
	ctx = tenant.WithProject(tenant.WithOrg(ctx, orgID), projectID)
	n, err := s.store.Prune(ctx, projectID, s.now(), s.cfg.Idle)
	if err != nil {
		return 0, s.errOrReport(ctx, "enduser: prune", err)
	}
	return n, nil
}

func (s *Service) errOrReport(ctx context.Context, message string, err error) error {
	if _, ok := apperror.AsAppError(err); ok {
		return err
	}
	return s.unexpected(ctx, message, err)
}
