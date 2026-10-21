package enduser

import (
	"context"
	"errors"
	"slices"
	"sync"
	"time"

	"github.com/google/uuid"

	"altalune.id/othra/mailer"
)

var errFake = errors.New("fake store failure")

type fakeBrowser struct {
	b       Browser
	project uuid.UUID
	hash    string
	ended   bool
}

type fakeLink struct {
	l       Link
	project uuid.UUID
	hash    string
	ip      string
}

type fakeUser struct {
	id, project uuid.UUID
	subject     string
	email       string
	verifiedAt  *time.Time
}

type fakeSession struct {
	id, project, browser, endUser uuid.UUID
	publicID                      string
	authTime, selectedAt          time.Time
	ended                         bool
}

type fakeStore struct {
	mu         sync.Mutex
	links      []*fakeLink
	browsers   []*fakeBrowser
	users      []*fakeUser
	sessions   []*fakeSession
	terms      map[uuid.UUID]map[int]TermsAcceptance
	failOn     string
	n          int
	raceInsert bool
}

func newFakeStore() *fakeStore { return &fakeStore{terms: map[uuid.UUID]map[int]TermsAcceptance{}} }

func (f *fakeStore) fail(op string) error {
	if f.failOn == op {
		return errFake
	}
	return nil
}

func (f *fakeStore) HasRecentLink(_ context.Context, projectID uuid.UUID, email string, since time.Time) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, l := range f.links {
		if l.project == projectID && l.l.Email == email && l.l.UsedAt == nil && l.l.CreatedAt.After(since) {
			return true, f.fail("HasRecentLink")
		}
	}
	return false, f.fail("HasRecentLink")
}

func (f *fakeStore) DeleteUnusedLinks(_ context.Context, projectID uuid.UUID, email string, before time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.links = slices.DeleteFunc(f.links, func(l *fakeLink) bool {
		return l.project == projectID && l.l.Email == email && l.l.UsedAt == nil && !l.l.CreatedAt.After(before)
	})
	return f.fail("DeleteUnusedLinks")
}

func (f *fakeStore) InsertLink(_ context.Context, _, projectID uuid.UUID, l *Link, tokenHash, ip string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.fail("InsertLink"); err != nil {
		return err
	}
	if f.raceInsert {
		return ErrLiveLink
	}
	f.links = append(f.links, &fakeLink{l: *l, project: projectID, hash: tokenHash, ip: ip})
	return nil
}

func (f *fakeStore) findLink(projectID uuid.UUID, match func(*fakeLink) bool) (*Link, error) {
	for _, l := range f.links {
		if l.project == projectID && match(l) {
			cp := l.l
			return &cp, nil
		}
	}
	return nil, &LinkNotUsableError{}
}

func (f *fakeStore) LinkByTokenHash(_ context.Context, projectID uuid.UUID, tokenHash string) (*Link, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.fail("LinkByTokenHash"); err != nil {
		return nil, err
	}
	return f.findLink(projectID, func(l *fakeLink) bool { return l.hash == tokenHash })
}

func (f *fakeStore) LinkByPublicID(_ context.Context, projectID uuid.UUID, publicID string) (*Link, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.findLink(projectID, func(l *fakeLink) bool { return l.l.PublicID == publicID })
}

func (f *fakeStore) UseLink(_ context.Context, projectID uuid.UUID, tokenHash string, browserID uuid.UUID, now time.Time) (*Link, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, l := range f.links {
		if l.project == projectID && l.hash == tokenHash && l.l.UsedAt == nil && now.Before(l.l.ExpiresAt) {
			l.l.UsedAt, l.l.BrowserSessionID = &now, &browserID
			cp := l.l
			return &cp, nil
		}
	}
	return nil, &LinkNotUsableError{}
}

func (f *fakeStore) BrowserBySecretHash(_ context.Context, projectID uuid.UUID, secretHash string) (*Browser, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.fail("BrowserBySecretHash"); err != nil {
		return nil, err
	}
	for _, b := range f.browsers {
		if b.project == projectID && b.hash == secretHash && !b.ended {
			cp := b.b
			return &cp, nil
		}
	}
	return nil, nil
}

func (f *fakeStore) InsertBrowser(_ context.Context, _, projectID uuid.UUID, b *Browser, secretHash, _ string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.browsers = append(f.browsers, &fakeBrowser{b: *b, project: projectID, hash: secretHash})
	return f.fail("InsertBrowser")
}

func (f *fakeStore) browser(id uuid.UUID) *fakeBrowser {
	for _, b := range f.browsers {
		if b.b.ID == id {
			return b
		}
	}
	return nil
}

func (f *fakeStore) RotateBrowser(_ context.Context, _, browserID uuid.UUID, secretHash string, now time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	b := f.browser(browserID)
	b.hash, b.b.LastSeenAt = secretHash, now
	return nil
}

func (f *fakeStore) TouchBrowser(_ context.Context, _, browserID uuid.UUID, now time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.browser(browserID).b.LastSeenAt = now
	return f.fail("TouchBrowser")
}

func (f *fakeStore) SaveEndUser(_ context.Context, _, projectID uuid.UUID, email string, now time.Time) (uuid.UUID, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, u := range f.users {
		if u.project == projectID && u.email == email {
			if u.verifiedAt == nil {
				u.verifiedAt = &now
			}
			return u.id, nil
		}
	}
	f.n++
	u := &fakeUser{id: uuid.New(), project: projectID, subject: "sub" + string(rune('a'+f.n)), email: email, verifiedAt: &now}
	f.users = append(f.users, u)
	return u.id, nil
}

func (f *fakeStore) Accounts(_ context.Context, projectID, browserID uuid.UUID) ([]Account, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.fail("Accounts"); err != nil {
		return nil, err
	}
	var out []Account
	for _, s := range f.sessions {
		if s.project != projectID || s.browser != browserID || s.ended {
			continue
		}
		for _, u := range f.users {
			if u.id == s.endUser {
				out = append(out, Account{
					UserSessionID: s.id, PublicID: s.publicID, EndUserID: u.id, Subject: u.subject, Email: u.email,
					AuthTime: s.authTime, SelectedAt: s.selectedAt,
				})
			}
		}
	}
	slices.SortStableFunc(out, func(a, b Account) int { return b.SelectedAt.Compare(a.SelectedAt) })
	return out, nil
}

func (f *fakeStore) SignIn(_ context.Context, _, projectID, browserID, endUserID uuid.UUID, now time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, s := range f.sessions {
		if s.browser == browserID && s.endUser == endUserID && !s.ended {
			s.authTime, s.selectedAt = now, now
			return nil
		}
	}
	f.n++
	f.sessions = append(f.sessions, &fakeSession{
		id: uuid.New(), project: projectID, browser: browserID, endUser: endUserID, publicID: "us" + uuid.NewString()[:8],
		authTime: now, selectedAt: now,
	})
	return nil
}

func (f *fakeStore) live(projectID, browserID uuid.UUID, publicID string) *fakeSession {
	for _, s := range f.sessions {
		if s.project == projectID && s.browser == browserID && s.publicID == publicID && !s.ended {
			return s
		}
	}
	return nil
}

func (f *fakeStore) Select(_ context.Context, projectID, browserID uuid.UUID, publicID string, now time.Time) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	s := f.live(projectID, browserID, publicID)
	if s == nil {
		return false, nil
	}
	s.selectedAt = now
	return true, nil
}

func (f *fakeStore) EndUserSession(_ context.Context, projectID, browserID uuid.UUID, publicID string, _ time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if s := f.live(projectID, browserID, publicID); s != nil {
		s.ended = true
	}
	return f.fail("EndUserSession")
}

func (f *fakeStore) KeepNewest(_ context.Context, _, browserID uuid.UUID, keep int, _ time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	var live []*fakeSession
	for _, s := range f.sessions {
		if s.browser == browserID && !s.ended {
			live = append(live, s)
		}
	}
	slices.SortStableFunc(live, func(a, b *fakeSession) int { return b.selectedAt.Compare(a.selectedAt) })
	for i, s := range live {
		if i >= keep {
			s.ended = true
		}
	}
	return nil
}

func (f *fakeStore) LatestTerms(_ context.Context, _, endUserID uuid.UUID) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	latest := 0
	for v := range f.terms[endUserID] {
		latest = max(latest, v)
	}
	return latest, f.fail("LatestTerms")
}

func (f *fakeStore) EndUserExists(_ context.Context, projectID uuid.UUID, email string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, u := range f.users {
		if u.project == projectID && u.email == email {
			return true, f.fail("EndUserExists")
		}
	}
	return false, f.fail("EndUserExists")
}

func (f *fakeStore) InsertTerms(_ context.Context, _, _ uuid.UUID, t TermsAcceptance) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.fail("InsertTerms"); err != nil {
		return err
	}
	if f.terms[t.EndUserID] == nil {
		f.terms[t.EndUserID] = map[int]TermsAcceptance{}
	}
	if _, ok := f.terms[t.EndUserID][t.Version]; !ok {
		f.terms[t.EndUserID][t.Version] = t
	}
	return nil
}

func (f *fakeStore) Prune(_ context.Context, _ uuid.UUID, now time.Time, idle time.Duration) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.fail("Prune"); err != nil {
		return 0, err
	}
	before := len(f.links) + len(f.sessions) + len(f.browsers)
	f.links = slices.DeleteFunc(f.links, func(l *fakeLink) bool { return !now.Before(l.l.ExpiresAt) })
	f.sessions = slices.DeleteFunc(f.sessions, func(s *fakeSession) bool { return s.ended })
	f.browsers = slices.DeleteFunc(f.browsers, func(b *fakeBrowser) bool { return !b.b.Live(now, idle) || b.ended })
	return before - len(f.links) - len(f.sessions) - len(f.browsers), nil
}

type fakeMail struct {
	sent []mailer.Message
	err  error
}

func (m *fakeMail) Enqueue(_ context.Context, msg mailer.Message) (uuid.UUID, error) {
	if m.err != nil {
		return uuid.Nil, m.err
	}
	m.sent = append(m.sent, msg)
	return uuid.New(), nil
}
