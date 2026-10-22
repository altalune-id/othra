package hosted

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"sync"
	"time"

	"github.com/google/uuid"

	"altalune.id/othra/apperror"
	"altalune.id/othra/internal/oauth/oidc"
	"altalune.id/othra/internal/project"
	"altalune.id/othra/mailer"
)

var errFake = errors.New("fake failure")

type fakeLink struct {
	project  Project
	email    string
	returnTo string
	used     bool
}

type fakeBrowser struct {
	project  Project
	accounts []Account
}

type fakeUsers struct {
	mu       sync.Mutex
	links    map[string]*fakeLink
	browsers map[string]*fakeBrowser
	users    map[string]string
	accepted map[string][]int32
	mails    []mailer.Message
	requests []IssueRequest
	issued   []IssueRequest
	last     map[string]time.Time
	now      func() time.Time
	stamp    *tickClock
	redeems  int
	failOn   string
	seq      int
}

const fakeCooldown = time.Minute

type tickClock struct {
	mu   sync.Mutex
	last time.Time
	skew time.Duration
}

func (c *tickClock) advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.skew += d
}

func (c *tickClock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	t := time.Now().Add(c.skew).Truncate(time.Millisecond)
	if !t.After(c.last) {
		t = c.last.Add(time.Millisecond)
	}
	c.last = t
	return t
}

func newFakeUsers() *fakeUsers {
	return &fakeUsers{
		links: map[string]*fakeLink{}, browsers: map[string]*fakeBrowser{}, users: map[string]string{}, accepted: map[string][]int32{},
		last: map[string]time.Time{}, now: time.Now, stamp: &tickClock{},
	}
}

func notUsable() error {
	return apperror.New(apperror.CodeSignInLinkNotUsable, "This sign-in link can't be used")
}

func gone() error { return apperror.New(apperror.CodeSignInSessionGone, "Session gone") }

func (f *fakeUsers) next(prefix string) string {
	f.seq++
	return prefix + uuid.NewString()[:8]
}

func (f *fakeUsers) IssueLink(ctx context.Context, req IssueRequest) (Link, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.requests = append(f.requests, req)
	if f.failOn == "issue" {
		return Link{}, false, errFake
	}
	k, now := key(req.Project, req.Email), f.now()
	if last, ok := f.last[k]; ok && now.Sub(last) < fakeCooldown {
		return Link{}, false, nil
	}
	if req.Allow != nil {
		ok, err := req.Allow(ctx)
		if err != nil || !ok {
			return Link{}, false, err
		}
	}
	f.last[k] = now
	f.issued = append(f.issued, req)
	token := f.next("tok")
	_, known := f.users[key(req.Project, req.Email)]
	l := Link{Email: req.Email, URL: req.Project.IssuerURL + "/login/verify?t=" + token, SignUp: !known}
	f.links[token] = &fakeLink{project: req.Project, email: req.Email, returnTo: req.ReturnTo}
	if req.Compose != nil {
		m, err := req.Compose(l)
		if err != nil {
			return Link{}, false, err
		}
		f.mails = append(f.mails, m)
	}
	return l, true, nil
}

func key(p Project, s string) string { return p.ID.String() + "/" + s }

func (f *fakeUsers) LinkByToken(_ context.Context, p Project, token string) (LinkRef, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failOn == "link" {
		return LinkRef{}, errFake
	}
	l, ok := f.links[token]
	if !ok || l.used || l.project.ID != p.ID {
		return LinkRef{}, notUsable()
	}
	return LinkRef{ID: "id-" + token, Email: l.email}, nil
}

func (f *fakeUsers) Redeem(_ context.Context, req RedeemRequest) (*Redeemed, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failOn == "redeem" {
		return nil, errFake
	}
	l, ok := f.links[req.Token]
	if !ok || l.used || l.project.ID != req.Project.ID {
		return nil, notUsable()
	}
	l.used = true
	f.redeems++
	sub, ok := f.users[key(req.Project, l.email)]
	if !ok {
		sub = f.next("sub")
		f.users[key(req.Project, l.email)] = sub
	}
	b := f.browsers[req.BrowserSecret]
	if b == nil || b.project.ID != req.Project.ID {
		b = &fakeBrowser{project: req.Project}
	}
	delete(f.browsers, req.BrowserSecret)
	secret := f.next("sec")
	f.browsers[secret] = b
	now := f.stamp.now()
	a := Account{UserSession: f.next("us"), EndUser: sub, Email: l.email, AuthTime: now, SelectedAt: now}
	b.accounts = append([]Account{a}, b.accounts...)
	return &Redeemed{BrowserSecret: secret, ExpiresAt: time.Now().Add(90 * 24 * time.Hour), Account: a, ReturnTo: l.returnTo}, nil
}

func (f *fakeUsers) browser(p Project, secret string) *fakeBrowser {
	b := f.browsers[secret]
	if b == nil || b.project.ID != p.ID {
		return nil
	}
	return b
}

func (f *fakeUsers) Browser(_ context.Context, p Project, secret string) (*Browser, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failOn == "browser" {
		return nil, errFake
	}
	b := f.browser(p, secret)
	if b == nil {
		return &Browser{}, nil
	}
	return &Browser{Accounts: slices.Clone(b.accounts)}, nil
}

func (f *fakeUsers) Select(_ context.Context, p Project, secret, userSession string) (Account, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	b := f.browser(p, secret)
	if b == nil {
		return Account{}, gone()
	}
	for i, a := range b.accounts {
		if a.UserSession == userSession {
			a.SelectedAt = f.stamp.now()
			b.accounts = append([]Account{a}, slices.Delete(b.accounts, i, i+1)...)
			return a, nil
		}
	}
	return Account{}, gone()
}

func (f *fakeUsers) SignOut(_ context.Context, p Project, secret, userSession string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failOn == "signout" {
		return errFake
	}
	if b := f.browser(p, secret); b != nil {
		b.accounts = slices.DeleteFunc(b.accounts, func(a Account) bool { return a.UserSession == userSession })
	}
	return nil
}

func (f *fakeUsers) TermsState(_ context.Context, p Project, secret, userSession string) (TermsState, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failOn == "terms" {
		return TermsState{}, errFake
	}
	b := f.browser(p, secret)
	if b == nil {
		return TermsState{}, gone()
	}
	for _, a := range b.accounts {
		if a.UserSession != userSession {
			continue
		}
		got := f.accepted[key(p, a.EndUser)]
		if p.TermsVersion == 0 || slices.Contains(got, p.TermsVersion) {
			return TermsState{}, nil
		}
		return TermsState{Needed: true, Updated: len(got) > 0}, nil
	}
	return TermsState{}, gone()
}

func (f *fakeUsers) AcceptTerms(_ context.Context, a TermsAcceptance) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if a.Seen != a.Project.TermsVersion {
		return ErrTermsChanged
	}
	b := f.browser(a.Project, a.BrowserSecret)
	if b == nil {
		return gone()
	}
	for _, acc := range b.accounts {
		if acc.UserSession == a.UserSession {
			k := key(a.Project, acc.EndUser)
			f.accepted[k] = append(f.accepted[k], a.Project.TermsVersion)
			return nil
		}
	}
	return gone()
}

type fakeIssuers struct {
	byHandle map[string]*project.Issuer
	err      error
}

func (f *fakeIssuers) Resolve(r *http.Request) (*project.Issuer, error) {
	if f.err != nil {
		return nil, f.err
	}
	i, ok := f.byHandle[r.PathValue("handle")]
	if !ok {
		return nil, oidc.ErrUnknownIssuer
	}
	return i, nil
}
