package keys

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"slices"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"altalune.id/othra/apperror"
	"altalune.id/othra/sealer"
)

var (
	poolOnce sync.Once
	pool     []*rsa.PrivateKey
	poolNext int
	poolMu   sync.Mutex
)

// NOTE: keys come from a pool generated once per test binary, so tests stay fast and kids stay distinct.
func testKey(int) (*rsa.PrivateKey, error) {
	poolOnce.Do(func() {
		for range 8 {
			k, err := rsa.GenerateKey(rand.Reader, 2048)
			if err != nil {
				panic(err)
			}
			pool = append(pool, k)
		}
	})
	poolMu.Lock()
	defer poolMu.Unlock()
	k := pool[poolNext%len(pool)]
	poolNext++
	return k, nil
}

type row struct {
	key    Key
	sealed []byte
}

type fakeStore struct {
	mu     sync.Mutex
	rows   map[uuid.UUID]*row
	reads  int
	failOn string
}

var errFake = errors.New("fake store failure")

func newFakeStore() *fakeStore { return &fakeStore{rows: map[uuid.UUID]*row{}} }

func (s *fakeStore) fail(op string) error {
	if s.failOn == op {
		return errFake
	}
	return nil
}

func (s *fakeStore) Insert(_ context.Context, k *Key, sealed []byte) error {
	if err := s.fail("insert"); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rows[k.ID] = &row{key: *k, sealed: sealed}
	return nil
}

func (s *fakeStore) Update(_ context.Context, k *Key) error {
	if err := s.fail("update"); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rows[k.ID].key = *k
	if k.Status == StatusRetired {
		s.rows[k.ID].sealed = nil
	}
	return nil
}

func (s *fakeStore) Live(_ context.Context, projectID uuid.UUID) (next, active *Key, err error) {
	if err := s.fail("live"); err != nil {
		return nil, nil, err
	}
	for _, k := range s.byProject(projectID) {
		switch k.Status {
		case StatusNext:
			next = k
		case StatusActive:
			active = k
		}
	}
	return next, active, nil
}

func (s *fakeStore) byProject(projectID uuid.UUID) []*Key {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []*Key
	for _, r := range s.rows {
		if r.key.ProjectID == projectID {
			k := r.key
			out = append(out, &k)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if !out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].CreatedAt.Before(out[j].CreatedAt)
		}
		return out[i].KID < out[j].KID
	})
	return out
}

func (s *fakeStore) List(_ context.Context, projectID uuid.UUID) ([]*Key, error) {
	if err := s.fail("list"); err != nil {
		return nil, err
	}
	out := s.byProject(projectID)
	slices.Reverse(out)
	return out, nil
}

func (s *fakeStore) Published(_ context.Context, projectID uuid.UUID) ([]*Key, error) {
	if err := s.fail("published"); err != nil {
		return nil, err
	}
	s.mu.Lock()
	s.reads++
	s.mu.Unlock()
	return slices.DeleteFunc(s.byProject(projectID), func(k *Key) bool { return !k.Published() }), nil
}

func (s *fakeStore) Active(_ context.Context, projectID uuid.UUID) (*Key, []byte, error) {
	if err := s.fail("active"); err != nil {
		return nil, nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.reads++
	for _, r := range s.rows {
		if r.key.ProjectID == projectID && r.key.Status == StatusActive {
			k := r.key
			return &k, r.sealed, nil
		}
	}
	return nil, nil, &NoActiveKeyError{ProjectID: projectID.String()}
}

func (s *fakeStore) Retiring(_ context.Context, projectID uuid.UUID, now time.Time) ([]*Key, error) {
	if err := s.fail("retiring"); err != nil {
		return nil, err
	}
	return slices.DeleteFunc(s.byProject(projectID), func(k *Key) bool {
		return k.Status != StatusRetiring || k.RetireAfter.After(now)
	}), nil
}

func (s *fakeStore) DeleteRetired(_ context.Context, projectID uuid.UUID, cutoff time.Time) (int64, error) {
	if err := s.fail("delete"); err != nil {
		return 0, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var n int64
	for id, r := range s.rows {
		if r.key.ProjectID == projectID && r.key.Status == StatusRetired && r.key.RetiredAt.Before(cutoff) {
			delete(s.rows, id)
			n++
		}
	}
	return n, nil
}

func (s *fakeStore) status(projectID uuid.UUID) map[Status]int {
	out := map[Status]int{}
	for _, k := range s.byProject(projectID) {
		out[k.Status]++
	}
	return out
}

type passTx struct{}

func (passTx) RunInTx(ctx context.Context, fn func(ctx context.Context) error) error { return fn(ctx) }

type harness struct {
	svc     *Service
	store   *fakeStore
	org     uuid.UUID
	project uuid.UUID
	now     time.Time
	changed []uuid.UUID
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	key, err := sealer.GenerateKey()
	require.NoError(t, err)
	sl, err := sealer.New(key)
	require.NoError(t, err)
	h := &harness{store: newFakeStore(), org: uuid.New(), project: uuid.New(), now: time.Date(2026, 10, 18, 9, 0, 0, 0, time.UTC)}
	h.svc = NewService(h.store, passTx{}, sl, Config{RSABits: 2048, PrePublish: 10 * time.Minute, Overlap: 7 * 24 * time.Hour},
		apperror.NewReporter(nil, false).Unexpected).
		WithOnChange(func(id uuid.UUID) { h.changed = append(h.changed, id) })
	h.svc.generate = testKey
	h.svc.now = func() time.Time { return h.now }
	return h
}

func (h *harness) generate(t *testing.T) *Key {
	t.Helper()
	k, err := h.svc.Generate(context.Background(), h.org, h.project)
	require.NoError(t, err)
	return k
}

func codeOf(err error) apperror.Code {
	if app, ok := apperror.AsAppError(err); ok {
		return app.Code()
	}
	return ""
}
