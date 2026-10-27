package upstream

import (
	"context"
	"errors"
	"sync"

	"github.com/google/uuid"
)

var errFake = errors.New("fake failure")

type fakeStore struct {
	mu      sync.Mutex
	rows    map[uuid.UUID]Upstream
	secrets map[uuid.UUID][]byte
	linked  map[uuid.UUID]bool
	failOn  string
}

func newFakeStore() *fakeStore {
	return &fakeStore{rows: map[uuid.UUID]Upstream{}, secrets: map[uuid.UUID][]byte{}, linked: map[uuid.UUID]bool{}}
}

func (f *fakeStore) fail(op string) error {
	if f.failOn == op {
		return errFake
	}
	return nil
}

func (f *fakeStore) Insert(_ context.Context, u *Upstream, sealed []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.fail("insert"); err != nil {
		return err
	}
	for _, r := range f.rows {
		if r.ProjectID == u.ProjectID && r.Kind == u.Kind {
			return &ExistsError{}
		}
	}
	f.rows[u.ID], f.secrets[u.ID] = *u, sealed
	return nil
}

func (f *fakeStore) Update(_ context.Context, u *Upstream, sealed []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.fail("update"); err != nil {
		return err
	}
	f.rows[u.ID] = *u
	if sealed != nil {
		f.secrets[u.ID] = sealed
	}
	return nil
}

func (f *fakeStore) Delete(_ context.Context, _, id uuid.UUID) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.fail("delete"); err != nil {
		return err
	}
	if f.linked[id] {
		return &HasIdentitiesError{}
	}
	delete(f.rows, id)
	return nil
}

func (f *fakeStore) List(_ context.Context, projectID uuid.UUID) ([]*Upstream, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.fail("list"); err != nil {
		return nil, err
	}
	var out []*Upstream
	for _, r := range f.rows {
		if r.ProjectID == projectID {
			r.IdentitiesLinked = f.linked[r.ID]
			out = append(out, &r)
		}
	}
	return out, nil
}

func (f *fakeStore) find(op string, match func(Upstream) bool) (*Upstream, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.fail(op); err != nil {
		return nil, err
	}
	for _, r := range f.rows {
		if match(r) {
			r.IdentitiesLinked = f.linked[r.ID]
			return &r, nil
		}
	}
	return nil, &NotFoundError{}
}

func (f *fakeStore) ByPublicID(_ context.Context, projectID uuid.UUID, publicID string) (*Upstream, error) {
	return f.find("get", func(r Upstream) bool { return r.ProjectID == projectID && r.PublicID == publicID })
}

func (f *fakeStore) Lock(_ context.Context, projectID uuid.UUID, publicID string) (*Upstream, error) {
	return f.find("lock", func(r Upstream) bool { return r.ProjectID == projectID && r.PublicID == publicID })
}

func (f *fakeStore) ByKind(_ context.Context, projectID uuid.UUID, kind Kind) (*Upstream, error) {
	return f.find("bykind", func(r Upstream) bool { return r.ProjectID == projectID && r.Kind == kind })
}

func (f *fakeStore) Secret(_ context.Context, _, id uuid.UUID) ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.fail("secret"); err != nil {
		return nil, err
	}
	return f.secrets[id], nil
}

type fakeGuard struct {
	on     bool
	err    error
	locked []uuid.UUID
}

func (g *fakeGuard) EmailLinkOn(_ context.Context, projectID uuid.UUID) (bool, error) {
	g.locked = append(g.locked, projectID)
	return g.on, g.err
}

type passTx struct{}

func (passTx) RunInTx(ctx context.Context, fn func(ctx context.Context) error) error { return fn(ctx) }
