package federation

import (
	"context"
	"errors"
	"slices"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/altalune-id/othra/internal/upstream"
)

type fakeStore struct {
	mu        sync.Mutex
	rows      []storedIdentity
	endUsers  map[uuid.UUID]bool
	insertErr []error
	onInsert  func()
	err       error
}

func newFakeStore() *fakeStore { return &fakeStore{endUsers: map[uuid.UUID]bool{}} }

func (f *fakeStore) BySubject(_ context.Context, _ uuid.UUID, issuer, subject string) (*storedIdentity, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return nil, f.err
	}
	for _, r := range f.rows {
		if r.Issuer == issuer && r.Subject == subject {
			return &r, nil
		}
	}
	return nil, nil
}

func (f *fakeStore) Insert(_ context.Context, _, _, endUserID uuid.UUID, id Identity, publicID string, now time.Time) error {
	if hook := f.onInsert; hook != nil {
		f.onInsert = nil
		hook()
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.insertErr) > 0 {
		err := f.insertErr[0]
		f.insertErr = f.insertErr[1:]
		return err
	}
	for _, r := range f.rows {
		switch {
		case r.Issuer == id.Issuer && r.Subject == id.Subject:
			return errSubjectTaken
		case r.EndUserID == endUserID && r.UpstreamID == id.UpstreamID:
			return errUpstreamTaken
		}
	}
	id.PublicID, id.LinkedAt = publicID, now
	f.rows = append(f.rows, storedIdentity{ID: uuid.New(), PublicID: publicID, EndUserID: endUserID, Identity: id})
	f.endUsers[endUserID] = true
	return nil
}

func (f *fakeStore) Touch(_ context.Context, _, id uuid.UUID, email string, verified bool, _ time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := range f.rows {
		if f.rows[i].ID == id {
			f.rows[i].Email, f.rows[i].EmailVerified = email, verified
		}
	}
	return f.err
}

func (f *fakeStore) ListByEndUser(_ context.Context, _, endUserID uuid.UUID) ([]Identity, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []Identity
	for _, r := range f.rows {
		if r.EndUserID == endUserID {
			out = append(out, r.Identity)
		}
	}
	return out, f.err
}

func (f *fakeStore) LockEndUser(_ context.Context, _, endUserID uuid.UUID) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.endUsers[endUserID], f.err
}

func (f *fakeStore) Delete(_ context.Context, _, endUserID uuid.UUID, publicID string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := len(f.rows)
	f.rows = slices.DeleteFunc(f.rows, func(r storedIdentity) bool { return r.EndUserID == endUserID && r.PublicID == publicID })
	return len(f.rows) < n, f.err
}

func (f *fakeStore) EveryEndUserLinked(context.Context, uuid.UUID) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for id := range f.endUsers {
		if !slices.ContainsFunc(f.rows, func(r storedIdentity) bool { return r.EndUserID == id }) {
			return false, f.err
		}
	}
	return true, f.err
}

type fakeUpstreams struct {
	creds   *upstream.Credentials
	err     error
	offered int
}

func (f *fakeUpstreams) Offered(context.Context, uuid.UUID, uuid.UUID) ([]upstream.Kind, error) {
	f.offered++
	if f.creds == nil || !f.creds.Upstream.Enabled {
		return nil, f.err
	}
	return []upstream.Kind{f.creds.Upstream.Kind}, f.err
}

func (f *fakeUpstreams) Credentials(context.Context, uuid.UUID, uuid.UUID, upstream.Kind) (*upstream.Credentials, error) {
	if f.err != nil {
		return nil, f.err
	}
	if f.creds == nil || !f.creds.Upstream.Enabled {
		return nil, &upstream.NotFoundError{}
	}
	return f.creds, nil
}

type fakeUsers struct {
	byEmail   map[string]uuid.UUID
	createErr error
	onCreate  func() error
	onLookup  func()
	created   int
	signInErr error
	signedIn  []SessionRequest
}

func (f *fakeUsers) EndUserIDByEmail(_ context.Context, _ Project, email string) (uuid.UUID, bool, error) {
	if hook := f.onLookup; hook != nil {
		f.onLookup = nil
		hook()
	}
	id, ok := f.byEmail[email]
	return id, ok, nil
}

func (f *fakeUsers) Create(_ context.Context, _ Project, email, _ string) (uuid.UUID, error) {
	if hook := f.onCreate; hook != nil {
		f.onCreate = nil
		return uuid.Nil, hook()
	}
	if f.createErr != nil {
		return uuid.Nil, f.createErr
	}
	f.created++
	id := uuid.New()
	f.byEmail[email] = id
	return id, nil
}

func (f *fakeUsers) SignIn(_ context.Context, _ Project, req SessionRequest) (*Session, error) {
	if f.signInErr != nil {
		return nil, f.signInErr
	}
	f.signedIn = append(f.signedIn, req)
	return &Session{BrowserSecret: "secret", EndUserID: req.EndUserID, UserSession: "us1"}, nil
}

var errBoom = errors.New("boom")

type fakeSwitch struct {
	on     bool
	err    error
	locked int
}

func (f *fakeSwitch) LockEmailLink(context.Context, uuid.UUID) (bool, error) {
	f.locked++
	return f.on, f.err
}

func (f *fakeSwitch) EmailLink(context.Context, uuid.UUID) (bool, error) { return f.on, f.err }
