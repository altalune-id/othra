package resourceserver

import (
	"context"
	"errors"
	"slices"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"altalune.id/othra/apperror"
)

var errFake = errors.New("fake failure")

type fakeStore struct {
	mu      sync.Mutex
	servers map[uuid.UUID]ResourceServer
	links   map[[2]uuid.UUID]Link
	failOn  string
}

func newFakeStore() *fakeStore {
	return &fakeStore{servers: map[uuid.UUID]ResourceServer{}, links: map[[2]uuid.UUID]Link{}}
}

func (s *fakeStore) fail(op string) error {
	if s.failOn == op {
		return errFake
	}
	return nil
}

func (s *fakeStore) Insert(_ context.Context, r *ResourceServer) error {
	if err := s.fail("insert"); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, got := range s.servers {
		if got.ProjectID == r.ProjectID && got.Identifier == r.Identifier {
			return &IdentifierTakenError{Identifier: r.Identifier}
		}
	}
	s.servers[r.ID] = clone(r)
	return nil
}

func clone(r *ResourceServer) ResourceServer {
	c := *r
	c.Scopes = slices.Clone(r.Scopes)
	return c
}

func (s *fakeStore) Update(_ context.Context, r *ResourceServer) error {
	if err := s.fail("update"); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.servers[r.ID] = clone(r)
	return nil
}

func (s *fakeStore) Delete(_ context.Context, _, id uuid.UUID) error {
	if err := s.fail("delete"); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.servers, id)
	return nil
}

func (s *fakeStore) List(_ context.Context, projectID uuid.UUID) ([]*ResourceServer, error) {
	if err := s.fail("list"); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []*ResourceServer
	for _, r := range s.servers {
		if r.ProjectID == projectID {
			c := clone(&r)
			out = append(out, &c)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func (s *fakeStore) ByPublicID(_ context.Context, projectID uuid.UUID, publicID string) (*ResourceServer, error) {
	if err := s.fail("get"); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, r := range s.servers {
		if r.ProjectID == projectID && r.PublicID == publicID {
			c := clone(&r)
			return &c, nil
		}
	}
	return nil, &NotFoundError{}
}

func (s *fakeStore) ByIdentifier(_ context.Context, projectID uuid.UUID, identifier string) (*ResourceServer, error) {
	return s.find("get", func(r ResourceServer) bool { return r.ProjectID == projectID && r.Identifier == identifier })
}

func (s *fakeStore) ByID(_ context.Context, projectID, id uuid.UUID) (*ResourceServer, error) {
	return s.find("get", func(r ResourceServer) bool { return r.ProjectID == projectID && r.ID == id })
}

func (s *fakeStore) find(op string, match func(ResourceServer) bool) (*ResourceServer, error) {
	if err := s.fail(op); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, r := range s.servers {
		if match(r) {
			c := clone(&r)
			return &c, nil
		}
	}
	return nil, &NotFoundError{}
}

func (s *fakeStore) Lock(ctx context.Context, projectID uuid.UUID, publicID string) (*ResourceServer, error) {
	if err := s.fail("lock"); err != nil {
		return nil, err
	}
	return s.ByPublicID(ctx, projectID, publicID)
}

func (s *fakeStore) Due(_ context.Context, projectID uuid.UUID, cutoff time.Time, limit int) ([]*ResourceServer, error) {
	if err := s.fail("due"); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []*ResourceServer
	for _, r := range s.servers {
		if r.ProjectID == projectID && !r.Trusted && (r.LastCheckedAt == nil || r.LastCheckedAt.Before(cutoff)) {
			c := clone(&r)
			out = append(out, &c)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (s *fakeStore) Links(_ context.Context, projectID, clientID, serverID uuid.UUID) ([]*Link, error) {
	if err := s.fail("links"); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []*Link
	for _, l := range s.links {
		if l.ProjectID != projectID || clientID != uuid.Nil && l.ClientID != clientID ||
			serverID != uuid.Nil && l.ResourceServer.ID != serverID {
			continue
		}
		r := clone(l.ResourceServer)
		got := l
		got.ResourceServer, got.Scopes, got.Client = &r, slices.Clone(l.Scopes), ClientRef{}
		out = append(out, &got)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ResourceServer.Name < out[j].ResourceServer.Name })
	return out, nil
}

func (s *fakeStore) UpsertLink(_ context.Context, l *Link) error {
	if err := s.fail("upsert"); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	key := [2]uuid.UUID{l.ClientID, l.ResourceServer.ID}
	if old, ok := s.links[key]; ok {
		old.Scopes, old.UpdatedAt = slices.Clone(l.Scopes), l.UpdatedAt
		s.links[key] = old
		return nil
	}
	s.links[key] = *l
	return nil
}

func (s *fakeStore) UpdateLink(_ context.Context, l *Link) (bool, error) {
	if err := s.fail("updatelink"); err != nil {
		return false, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	key := [2]uuid.UUID{l.ClientID, l.ResourceServer.ID}
	old, ok := s.links[key]
	if !ok {
		return false, nil
	}
	old.Scopes, old.UpdatedAt = slices.Clone(l.Scopes), l.UpdatedAt
	s.links[key] = old
	return true, nil
}

func (s *fakeStore) DeleteLink(_ context.Context, _, clientID, serverID uuid.UUID) error {
	if err := s.fail("deletelink"); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.links, [2]uuid.UUID{clientID, serverID})
	return nil
}

func (s *fakeStore) server(id uuid.UUID) ResourceServer {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.servers[id]
}

type passTx struct{}

func (passTx) RunInTx(ctx context.Context, fn func(ctx context.Context) error) error { return fn(ctx) }

type fakeIssuers struct {
	locked []uuid.UUID
	failOn string
	issuer string
}

func (f *fakeIssuers) Project(context.Context, uuid.UUID) (ProjectRef, error) {
	if f.failOn == "issuer" {
		return ProjectRef{}, errFake
	}
	return ProjectRef{PublicID: testProjectPublicID, Issuer: f.issuer}, nil
}

const testProjectPublicID = "prjprjprjprjprjprjprj"

type fakePeople struct{}

func (fakePeople) UserPublicID(_ context.Context, id uuid.UUID) (string, error) {
	if id == uuid.Nil {
		return "", errFake
	}
	return "usr" + id.String()[:18], nil
}

func (f *fakeIssuers) LockIssuerHandle(_ context.Context, projectID uuid.UUID) error {
	if f.failOn == "lock" {
		return errFake
	}
	f.locked = append(f.locked, projectID)
	return nil
}

type fakeClients struct {
	byPublic map[string]ClientRef
	failOn   string
}

func (f *fakeClients) add(name string) ClientRef {
	c := ClientRef{ID: uuid.New(), PublicID: "c" + uuid.NewString()[:20], Name: name}
	f.byPublic[c.PublicID] = c
	return c
}

func (f *fakeClients) Lookup(_ context.Context, _, _ uuid.UUID, publicID string) (ClientRef, bool, error) {
	if f.failOn == "lookup" {
		return ClientRef{}, false, errFake
	}
	c, ok := f.byPublic[publicID]
	return c, ok, nil
}

func (f *fakeClients) Refs(_ context.Context, _, _ uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]ClientRef, error) {
	if f.failOn == "refs" {
		return nil, errFake
	}
	out := map[uuid.UUID]ClientRef{}
	for _, c := range f.byPublic {
		if slices.Contains(ids, c.ID) {
			out[c.ID] = c
		}
	}
	return out, nil
}

type fakeChecker struct {
	mu      sync.Mutex
	failure *Failure
	calls   []string
}

func (f *fakeChecker) Check(_ context.Context, identifier, issuer string) *Failure {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, identifier+" "+issuer)
	if f.failure == nil {
		return nil
	}
	c := *f.failure
	return &c
}

func (f *fakeChecker) set(fail *Failure) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.failure = fail
}

type harness struct {
	svc     *Service
	store   *fakeStore
	issuers *fakeIssuers
	clients *fakeClients
	checker *fakeChecker
	org     uuid.UUID
	project uuid.UUID
	person  uuid.UUID
	now     time.Time
}

func newHarness(t *testing.T, trust bool) *harness {
	t.Helper()
	h := &harness{
		store: newFakeStore(), issuers: &fakeIssuers{issuer: testIssuer}, clients: &fakeClients{byPublic: map[string]ClientRef{}},
		checker: &fakeChecker{}, org: uuid.New(), project: uuid.New(), person: uuid.New(),
		now: time.Date(2026, 10, 20, 9, 0, 0, 0, time.UTC),
	}
	h.svc = NewService(Deps{
		Store: h.store, Transactor: passTx{}, Issuers: h.issuers, People: fakePeople{}, Clients: h.clients, Checker: h.checker,
		Unexpected: apperror.NewReporter(nil, false).Unexpected,
	}, Config{Interval: time.Hour, StaleAfter: 24 * time.Hour, TrustAvailable: trust})
	h.svc.now = func() time.Time { return h.now }
	return h
}

func (h *harness) create(t *testing.T, identifier string, scopes ...string) *ResourceServer {
	t.Helper()
	if len(scopes) == 0 {
		scopes = []string{"read", "write"}
	}
	r, err := h.svc.Create(context.Background(), CreateRequest{
		OrgID: h.org, ProjectID: h.project, Name: "API " + identifier, Identifier: identifier, Scopes: scopes, Person: h.person,
		Trusted: IsLoopbackHTTP(identifier),
	})
	require.NoError(t, err)
	return r
}

func codeOf(err error) apperror.Code {
	if app, ok := apperror.AsAppError(err); ok {
		return app.Code()
	}
	return ""
}

func asError[E error](t *testing.T, err error) E {
	t.Helper()
	e, ok := errors.AsType[E](err)
	require.True(t, ok, "%v", err)
	return e
}
