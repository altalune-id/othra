package pat

import (
	"context"
	"errors"
	"maps"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"altalune.id/othra/apperror"
	"altalune.id/othra/internal/kernel/secret"
	"altalune.id/othra/internal/kernel/usage"
	"altalune.id/othra/internal/testutil/fakes"
)

var errBoom = errors.New("boom")

type fakeStore struct {
	mu      sync.Mutex
	tokens  []*Token
	hashes  map[uuid.UUID]string
	gone    bool
	err     error
	touched map[uuid.UUID]time.Time
}

func newFakeStore() *fakeStore {
	return &fakeStore{hashes: map[uuid.UUID]string{}, touched: map[uuid.UUID]time.Time{}}
}

func (f *fakeStore) Insert(_ context.Context, t *Token, hash string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return f.err
	}
	cp := *t
	f.tokens = append(f.tokens, &cp)
	f.hashes[t.ID] = hash
	return nil
}

func (f *fakeStore) ByHash(_ context.Context, _ uuid.UUID, hash string) (*Token, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, t := range f.tokens {
		if f.hashes[t.ID] == hash {
			cp := *t
			return &cp, f.err
		}
	}
	return nil, f.err
}

func (f *fakeStore) ListByEndUser(_ context.Context, _, endUserID uuid.UUID) ([]*Token, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []*Token
	for _, tok := range slices.Backward(f.tokens) {
		if tok.EndUserID == endUserID {
			out = append(out, tok)
		}
	}
	return out, f.err
}

func (f *fakeStore) LockEndUser(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return !f.gone, f.err
}

func (f *fakeStore) CountLive(_ context.Context, _, endUserID uuid.UUID, now time.Time) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, t := range f.tokens {
		if t.EndUserID == endUserID && t.State(now) == StateActive {
			n++
		}
	}
	return n, f.err
}

func (f *fakeStore) Revoke(_ context.Context, _, endUserID uuid.UUID, publicID string, now time.Time) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, t := range f.tokens {
		if t.EndUserID == endUserID && t.PublicID == publicID && t.RevokedAt == nil {
			t.RevokedAt = &now
			return true, f.err
		}
	}
	return false, f.err
}

func (f *fakeStore) TouchLastUsed(_ context.Context, used map[uuid.UUID]time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	maps.Copy(f.touched, used)
	return nil
}

func (f *fakeStore) Prune(_ context.Context, _ uuid.UUID, cutoff time.Time) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := len(f.tokens)
	f.tokens = slices.DeleteFunc(f.tokens, func(t *Token) bool {
		return t.ExpiresAt.Before(cutoff) || t.RevokedAt != nil && t.RevokedAt.Before(cutoff)
	})
	return n - len(f.tokens), f.err
}

type fakeServers struct {
	servers []Server
	err     error
}

func (f *fakeServers) Accepting(context.Context, Project) ([]Server, error) {
	var out []Server
	for _, s := range f.servers {
		if s.Accepting {
			out = append(out, s)
		}
	}
	return out, f.err
}

func (f *fakeServers) ByPublicID(_ context.Context, _ Project, publicID string) (Server, bool, error) {
	for _, s := range f.servers {
		if s.PublicID == publicID {
			return s, true, f.err
		}
	}
	return Server{}, false, f.err
}

func (f *fakeServers) ByID(_ context.Context, _ Project, id uuid.UUID) (Server, bool, error) {
	for _, s := range f.servers {
		if s.ID == id {
			return s, true, f.err
		}
	}
	return Server{}, false, f.err
}

type fakeSubjects struct {
	err  error
	gone bool
}

func (f *fakeSubjects) Subject(_ context.Context, _ Project, endUserID uuid.UUID) (string, error) {
	if f.gone {
		return "", f.err
	}
	return "sub-" + endUserID.String()[:8], f.err
}

type harness struct {
	svc      *Service
	store    *fakeStore
	servers  *fakeServers
	subjects *fakeSubjects
	usage    *usage.Worker
	p        Project
	user     uuid.UUID
	open     Server
	closed   Server
	now      time.Time
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	h := &harness{
		store: newFakeStore(), subjects: &fakeSubjects{}, user: uuid.New(), now: time.Date(2026, 10, 26, 9, 0, 0, 0, time.UTC),
		p: Project{OrgID: uuid.New(), ID: uuid.New(), PublicID: "prj1", Name: "Acme Store"},
		open: Server{
			ID: uuid.New(), PublicID: "rs-open", Name: "MCP", Identifier: "https://mcp.example.com/mcp", Scopes: []string{"read", "write"},
			Accepting: true,
		},
		closed: Server{ID: uuid.New(), PublicID: "rs-closed", Identifier: "https://closed.example.com", Scopes: []string{"read"}},
	}
	h.servers = &fakeServers{servers: []Server{h.open, h.closed}}
	h.usage = usage.New("pat-usage", h.store, time.Hour, nil)
	h.svc = NewService(h.store, fakes.Transactor{}, h.servers, h.subjects, h.usage, apperror.NewReporter(nil, false).Unexpected).
		WithClock(func() time.Time { return h.now })
	return h
}

func (h *harness) create(t *testing.T, scopes ...string) *Created {
	t.Helper()
	if len(scopes) == 0 {
		scopes = []string{"read"}
	}
	c, err := h.svc.Create(context.Background(), h.p, CreateRequest{
		EndUserID: h.user, Name: "cli", ServerPublicID: h.open.PublicID, Scopes: scopes, Expiry: Expiry30Days,
	})
	require.NoError(t, err)
	return c
}

func codeOf(err error) apperror.Code {
	if app, ok := apperror.AsAppError(err); ok {
		return app.Code()
	}
	return ""
}

func TestCreate(t *testing.T) {
	h := newHarness(t)
	c := h.create(t, "write", "read", "read")
	assert.True(t, Scheme.Valid(c.Secret))
	assert.True(t, strings.HasPrefix(c.Secret, Prefix))
	assert.Equal(t, c.Secret[len(c.Secret)-4:], c.Token.Hint)
	assert.Equal(t, []string{"read", "write"}, c.Token.Scopes)
	assert.Equal(t, h.now.Add(30*24*time.Hour), c.Token.ExpiresAt)
	assert.Equal(t, h.open.ID, c.Token.ResourceServerID)
	assert.Equal(t, secret.Versioned(c.Secret), h.store.hashes[c.Token.ID], "SHA-256 at rest, never the token")
	assert.Equal(t, h.open, c.Server)
}

// M8 P6, P7, A9: every refusal has its code.
func TestCreate_Refused(t *testing.T) {
	ctx := context.Background()
	for name, tc := range map[string]struct {
		edit func(h *harness, r *CreateRequest)
		code apperror.Code
	}{
		"blank name":          {func(_ *harness, r *CreateRequest) { r.Name = "  " }, apperror.CodeAccountTokenInvalid},
		"long name":           {func(_ *harness, r *CreateRequest) { r.Name = strings.Repeat("é", 101) }, apperror.CodeAccountTokenInvalid},
		"91 days":             {func(_ *harness, r *CreateRequest) { r.Expiry = 91 }, apperror.CodeAccountTokenInvalid},
		"unknown server":      {func(_ *harness, r *CreateRequest) { r.ServerPublicID = "nope" }, apperror.CodeAccountTokenInvalid},
		"closed server":       {func(h *harness, r *CreateRequest) { r.ServerPublicID = h.closed.PublicID }, apperror.CodeAccountTokenServerClosed},
		"a scope off-catalog": {func(_ *harness, r *CreateRequest) { r.Scopes = []string{"admin"} }, apperror.CodeAccountTokenInvalid},
		"no scope":            {func(_ *harness, r *CreateRequest) { r.Scopes = nil }, apperror.CodeAccountTokenInvalid},
	} {
		t.Run(name, func(t *testing.T) {
			h := newHarness(t)
			req := CreateRequest{EndUserID: h.user, Name: "cli", ServerPublicID: h.open.PublicID, Scopes: []string{"read"},
				Expiry: Expiry7Days}
			tc.edit(h, &req)
			_, err := h.svc.Create(ctx, h.p, req)
			assert.Equal(t, tc.code, codeOf(err))
			assert.Empty(t, h.store.tokens)
		})
	}

	h := newHarness(t)
	_, err := h.svc.Create(ctx, h.p, CreateRequest{EndUserID: h.user, Name: "x", ServerPublicID: h.closed.PublicID,
		Scopes: []string{"read"}, Expiry: Expiry7Days})
	app, _ := apperror.AsAppError(err)
	assert.Equal(t, "Acme Store", app.Meta()["project"], "ACC006 names the project")

	for range MaxLive {
		h.create(t)
	}
	_, err = h.svc.Create(ctx, h.p, CreateRequest{EndUserID: h.user, Name: "x", ServerPublicID: h.open.PublicID,
		Scopes: []string{"read"}, Expiry: Expiry7Days})
	assert.Equal(t, apperror.CodeAccountTokenLimit, codeOf(err), "the 21st live token")
	h.now = h.now.Add(31 * 24 * time.Hour)
	h.create(t)
	assert.Len(t, h.store.tokens, MaxLive+1, "expired tokens don't count")

	h.store.gone = true
	_, err = h.svc.Create(ctx, h.p, CreateRequest{EndUserID: h.user, Name: "x", ServerPublicID: h.open.PublicID,
		Scopes: []string{"read"}, Expiry: Expiry7Days})
	assert.Equal(t, apperror.CodeAccountTokenNotFound, codeOf(err))
	h.store.gone, h.store.err = false, errBoom
	_, err = h.svc.Create(ctx, h.p, CreateRequest{EndUserID: h.user, Name: "x", ServerPublicID: h.open.PublicID,
		Scopes: []string{"read"}, Expiry: Expiry7Days})
	assert.Equal(t, apperror.CodeUnexpected, codeOf(err))
	h.store.err, h.servers.err = nil, errBoom
	_, err = h.svc.Create(ctx, h.p, CreateRequest{EndUserID: h.user, Name: "x", ServerPublicID: h.open.PublicID,
		Scopes: []string{"read"}, Expiry: Expiry7Days})
	assert.Equal(t, apperror.CodeUnexpected, codeOf(err))
}

func TestListRevokeAndServers(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	first, second := h.create(t), h.create(t, "write")
	list, err := h.svc.List(ctx, h.p, h.user)
	require.NoError(t, err)
	require.Len(t, list, 2)
	assert.Equal(t, second.Token.PublicID, list[0].Token.PublicID, "newest first")
	assert.Equal(t, h.open, list[0].Server)

	require.NoError(t, h.svc.Revoke(ctx, h.p, h.user, first.Token.PublicID))
	assert.Equal(t, apperror.CodeAccountTokenNotFound, codeOf(h.svc.Revoke(ctx, h.p, h.user, first.Token.PublicID)), "already revoked")
	assert.Equal(t, apperror.CodeAccountTokenNotFound, codeOf(h.svc.Revoke(ctx, h.p, uuid.New(), second.Token.PublicID)),
		"another end user's token is not found")

	servers, err := h.svc.TokenServers(ctx, h.p)
	require.NoError(t, err)
	assert.Equal(t, []Server{h.open}, servers)

	h.store.err = errBoom
	_, err = h.svc.List(ctx, h.p, h.user)
	assert.Equal(t, apperror.CodeUnexpected, codeOf(err))
	assert.Equal(t, apperror.CodeUnexpected, codeOf(h.svc.Revoke(ctx, h.p, h.user, second.Token.PublicID)))
	h.store.err, h.servers.err = nil, errBoom
	_, err = h.svc.TokenServers(ctx, h.p)
	assert.Equal(t, apperror.CodeUnexpected, codeOf(err))
	_, err = h.svc.List(ctx, h.p, h.user)
	assert.Equal(t, apperror.CodeUnexpected, codeOf(err))
}

// RFC 7662 §2.2, M8 P4: a personal token is active only for its own server, while live and while that server takes tokens.
func TestIntrospect(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	c := h.create(t)

	got, ok, err := h.svc.Introspect(ctx, h.p, h.open.ID, c.Secret)
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, "sub-"+h.user.String()[:8], got.Subject)
	assert.Equal(t, h.open.Identifier, got.Audience)
	assert.Equal(t, c.Token.PublicID, got.Token.PublicID)

	flushed := func() map[uuid.UUID]time.Time {
		cctx, cancel := context.WithCancel(ctx)
		cancel()
		_ = h.usage.Run(cctx)
		return h.store.touched
	}
	assert.Equal(t, h.now, flushed()[c.Token.ID], "last use is recorded through the batcher")

	for name, tc := range map[string]struct {
		caller uuid.UUID
		token  string
	}{
		"another server":     {h.closed.ID, c.Secret},
		"a garbled checksum": {h.open.ID, garble(t, c.Secret, len(c.Secret)-1)},
		"a garbled body":     {h.open.ID, garble(t, c.Secret, len(Prefix))},
		"not a personal one": {h.open.ID, "othra_cs_abc"},
		"unknown":            {h.open.ID, Prefix + strings.Repeat("0", 49)},
	} {
		_, ok, err := h.svc.Introspect(ctx, h.p, tc.caller, tc.token)
		require.NoError(t, err, name)
		assert.False(t, ok, name)
	}

	h.servers.servers[0].Accepting = false
	_, ok, err = h.svc.Introspect(ctx, h.p, h.open.ID, c.Secret)
	require.NoError(t, err)
	assert.False(t, ok, "a server that stopped taking tokens")
	h.servers.servers[0].Accepting = true

	h.subjects.gone = true
	_, ok, err = h.svc.Introspect(ctx, h.p, h.open.ID, c.Secret)
	require.NoError(t, err)
	assert.False(t, ok, "an end user that is gone")
	h.subjects.gone = false

	require.NoError(t, h.svc.Revoke(ctx, h.p, h.user, c.Token.PublicID))
	_, ok, err = h.svc.Introspect(ctx, h.p, h.open.ID, c.Secret)
	require.NoError(t, err)
	assert.False(t, ok, "revoked: inactive at once")

	live := h.create(t)
	h.now = h.now.Add(31 * 24 * time.Hour)
	_, ok, err = h.svc.Introspect(ctx, h.p, h.open.ID, live.Secret)
	require.NoError(t, err)
	assert.False(t, ok, "expired")

	h.now = h.now.Add(-31 * 24 * time.Hour)
	h.subjects.err = errBoom
	_, _, err = h.svc.Introspect(ctx, h.p, h.open.ID, live.Secret)
	assert.Equal(t, apperror.CodeUnexpected, codeOf(err))
	h.subjects.err, h.servers.err = nil, errBoom
	_, _, err = h.svc.Introspect(ctx, h.p, h.open.ID, live.Secret)
	assert.Equal(t, apperror.CodeUnexpected, codeOf(err))
	h.servers.err, h.store.err = nil, errBoom
	_, _, err = h.svc.Introspect(ctx, h.p, h.open.ID, live.Secret)
	assert.Equal(t, apperror.CodeUnexpected, codeOf(err))
}

func TestPrune(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	old, kept := h.create(t), h.create(t)
	require.NoError(t, h.svc.Revoke(ctx, h.p, h.user, old.Token.PublicID))
	h.now = h.now.Add(31 * 24 * time.Hour)
	n, err := h.svc.Prune(ctx, h.p.OrgID, h.p.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, n, "revoked 31 days ago: pruned; expired a day ago: kept")
	require.Len(t, h.store.tokens, 1)
	assert.Equal(t, kept.Token.ID, h.store.tokens[0].ID)
	h.now = h.now.Add(30 * 24 * time.Hour)
	n, err = h.svc.Prune(ctx, h.p.OrgID, h.p.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, n, "expired over 30 days ago")
	h.store.err = errBoom
	_, err = h.svc.Prune(ctx, h.p.OrgID, h.p.ID)
	assert.Equal(t, apperror.CodeUnexpected, codeOf(err))
}

func TestToken_State(t *testing.T) {
	now := time.Date(2026, 10, 26, 9, 0, 0, 0, time.UTC)
	tok := &Token{ExpiresAt: now.Add(time.Hour)}
	assert.Equal(t, StateActive, tok.State(now))
	assert.Equal(t, StateExpired, tok.State(now.Add(time.Hour)))
	tok.RevokedAt = &now
	assert.Equal(t, StateRevoked, tok.State(now))
	assert.True(t, Expiry90Days.Valid())
	assert.False(t, Expiry(1).Valid())
	assert.Equal(t, MaxLifetime, Expiry90Days.Duration())
}

// NOTE: garble rotates the character at i within base62, so the result always differs; a random swap can repeat it (1 in 62).
func garble(t *testing.T, token string, i int) string {
	t.Helper()
	const alphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	j := strings.IndexByte(alphabet, token[i])
	require.GreaterOrEqual(t, j, 0, "token char %q", token[i])
	out := token[:i] + string(alphabet[(j+1)%len(alphabet)]) + token[i+1:]
	require.NotEqual(t, token, out)
	require.False(t, Scheme.Valid(out), "the CRC rejects a one-character change")
	return out
}
