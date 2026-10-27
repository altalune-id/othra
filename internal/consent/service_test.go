package consent

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"altalune.id/othra/apperror"
	"altalune.id/othra/internal/kernel/tenant"
)

type fakeStore struct {
	rows   map[string]*Consent
	failOn string
}

var errFake = errors.New("fake: store down")

func newFakeStore() *fakeStore { return &fakeStore{rows: map[string]*Consent{}} }

func rowKey(k Key) string {
	rs := "-"
	if k.ResourceServerID != nil {
		rs = k.ResourceServerID.String()
	}
	return k.EndUserID.String() + "|" + k.ClientID.String() + "|" + rs
}

func (f *fakeStore) Find(ctx context.Context, _ uuid.UUID, key Key) (*Consent, error) {
	switch {
	case f.failOn == "find":
		return nil, errFake
	case tenant.Peek(ctx).ProjectID == uuid.Nil:
		return nil, errors.New("fake: no project scope")
	}
	return f.rows[rowKey(key)], nil
}

func (f *fakeStore) Upsert(_ context.Context, c *Consent, keep []string) (*Consent, error) {
	if f.failOn == "upsert" {
		return nil, errFake
	}
	if have := f.rows[rowKey(c.Key)]; have != nil {
		if keep != nil {
			have.Scopes = slices.DeleteFunc(have.Scopes, func(s string) bool { return !slices.Contains(keep, s) })
		}
		have.Scopes, have.UpdatedAt = Union(have.Scopes, c.Scopes), c.UpdatedAt
		return have, nil
	}
	stored := *c
	f.rows[rowKey(c.Key)] = &stored
	return &stored, nil
}

func (f *fakeStore) DeleteClient(_ context.Context, _, clientID uuid.UUID) error {
	if f.failOn == "deleteclient" {
		return errFake
	}
	for k, c := range f.rows {
		if c.Key.ClientID == clientID {
			delete(f.rows, k)
		}
	}
	return nil
}

func (f *fakeStore) ListByEndUser(_ context.Context, _, endUserID uuid.UUID) ([]*Consent, error) {
	if f.failOn == "list" {
		return nil, errFake
	}
	var out []*Consent
	for _, c := range f.rows {
		if c.Key.EndUserID == endUserID {
			out = append(out, c)
		}
	}
	slices.SortFunc(out, func(a, b *Consent) int { return a.GrantedAt.Compare(b.GrantedAt) })
	return out, nil
}

func (f *fakeStore) DeleteEndUserClient(_ context.Context, _, endUserID, clientID uuid.UUID) (int, error) {
	if f.failOn == "delete" {
		return 0, errFake
	}
	n := 0
	for k, c := range f.rows {
		if c.Key.EndUserID == endUserID && c.Key.ClientID == clientID {
			delete(f.rows, k)
			n++
		}
	}
	return n, nil
}

type fakeGrants struct {
	families []Family
	revoked  []uuid.UUID
	err      error
}

func (g *fakeGrants) LiveFamilies(context.Context, Project, uuid.UUID) ([]Family, error) {
	return g.families, g.err
}

func (g *fakeGrants) ResourceScopes(scopes []string) []string {
	return slices.DeleteFunc(slices.Clone(scopes), func(s string) bool {
		return slices.Contains([]string{"openid", "profile", "email", "offline_access"}, s)
	})
}

func (g *fakeGrants) RevokeClient(_ context.Context, _ Project, _, clientID uuid.UUID) (bool, error) {
	if g.err != nil {
		return false, g.err
	}
	live := false
	for i := 0; i < len(g.families); i++ {
		if g.families[i].ClientID == clientID {
			live = true
			g.families = slices.Delete(g.families, i, i+1)
			i--
		}
	}
	g.revoked = append(g.revoked, clientID)
	return live, nil
}

type fakeTx struct{}

func (fakeTx) RunInTx(ctx context.Context, fn func(ctx context.Context) error) error { return fn(ctx) }

func newTestService(store Store) (*Service, *time.Time) {
	now := time.Date(2026, 10, 23, 9, 0, 0, 0, time.UTC)
	s := NewService(store, func(_ context.Context, msg string, err error, _ ...any) *apperror.AppError {
		return apperror.New(apperror.CodeUnexpected, msg).WithCause(err)
	}).WithClock(func() time.Time { return now })
	return s, &now
}

func TestService_GrantAddsScopes(t *testing.T) {
	store := newFakeStore()
	s, now := newTestService(store)
	ctx, p := context.Background(), Project{OrgID: uuid.New(), ID: uuid.New(), PublicID: "prj"}
	rs := uuid.New()
	key := Key{EndUserID: uuid.New(), ClientID: uuid.New(), ResourceServerID: &rs}

	got, err := s.Find(ctx, p, key)
	require.NoError(t, err)
	assert.Nil(t, got)

	first, err := s.Grant(ctx, p, GrantRequest{Key: key, Scopes: []string{"sample:read", "sample:read"}, IP: "203.0.113.9", UserAgent: "UA"})
	require.NoError(t, err)
	assert.Equal(t, []string{"sample:read"}, first.Scopes)
	assert.Len(t, first.PublicID, 21)
	assert.Equal(t, *now, first.GrantedAt)

	*now = now.Add(time.Minute)
	second, err := s.Grant(ctx, p, GrantRequest{Key: key, Scopes: []string{"offline_access"}})
	require.NoError(t, err)
	assert.Equal(t, []string{"sample:read", "offline_access"}, second.Scopes, "C5: the union")
	assert.Equal(t, *now, second.UpdatedAt, "re-entry compares this with the marker")

	oidcOnly := Key{EndUserID: key.EndUserID, ClientID: key.ClientID}
	got, err = s.Find(ctx, p, oidcOnly)
	require.NoError(t, err)
	assert.Nil(t, got, "a consent without a resource server is its own row")
}

// R-6: approving again keeps only stored scopes still allowed, so churn in a server's catalog can't grow a consent past MaxScopes.
func TestService_GrantDropsScopesNoLongerAllowed(t *testing.T) {
	s, _ := newTestService(newFakeStore())
	ctx, p := context.Background(), Project{OrgID: uuid.New(), ID: uuid.New()}
	key := Key{EndUserID: uuid.New(), ClientID: uuid.New()}
	_, err := s.Grant(ctx, p, GrantRequest{Key: key, Scopes: []string{"openid", "old:read"}})
	require.NoError(t, err)
	got, err := s.Grant(ctx, p, GrantRequest{Key: key, Scopes: []string{"new:read"}, Allowed: []string{"openid", "new:read"}})
	require.NoError(t, err)
	assert.Equal(t, []string{"openid", "new:read"}, got.Scopes)
}

func TestService_GrantRefusesBadScopeCounts(t *testing.T) {
	s, _ := newTestService(newFakeStore())
	_, err := s.Grant(context.Background(), Project{}, GrantRequest{})
	assert.Error(t, err)
	many := make([]string, MaxScopes+1)
	for i := range many {
		many[i] = "s" + uuid.NewString()
	}
	_, err = s.Grant(context.Background(), Project{}, GrantRequest{Scopes: many})
	assert.Error(t, err)
}

// R-5, P8, A25: Disconnect drops every consent of the client across resource servers and revokes its grants; a second one is ACC003.
func TestService_Disconnect(t *testing.T) {
	store := newFakeStore()
	s, _ := newTestService(store)
	var logs bytes.Buffer
	s.WithLogger(slog.New(slog.NewJSONHandler(&logs, nil)))
	ctx, p := context.Background(), Project{OrgID: uuid.New(), ID: uuid.New(), PublicID: "prj"}
	user, client, other, rs := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	grants := &fakeGrants{families: []Family{{ClientID: client}, {ClientID: other}}}
	s = s.WithGrants(grants, fakeTx{})
	for _, k := range []Key{{EndUserID: user, ClientID: client}, {EndUserID: user, ClientID: client, ResourceServerID: &rs}} {
		_, err := s.Grant(ctx, p, GrantRequest{Key: k, Scopes: []string{"openid"}})
		require.NoError(t, err)
	}
	web := Connection{EndUserID: user, EndUserPublicID: "eu-ada", ClientID: client, ClientOAuthID: "cl-web"}
	require.NoError(t, s.Disconnect(ctx, p, web))
	assert.Empty(t, store.rows)
	assert.Equal(t, []uuid.UUID{client}, grants.revoked)
	assert.Len(t, grants.families, 1, "the other client keeps its grant")
	assert.Contains(t, logs.String(), `"msg":"consent.disconnected","project":"prj","end_user":"eu-ada","client_id":"cl-web"`)
	assert.NotContains(t, logs.String(), user.String(), "public ids only")
	assert.True(t, IsAppNotConnectedError(s.Disconnect(ctx, p, web)))
	require.NoError(t, s.Disconnect(ctx, p, Connection{EndUserID: user, ClientID: other}), "a live grant alone is connected (an implicit first-party app)")

	grants.err = errFake
	assert.Equal(t, apperror.CodeUnexpected, codeOf(s.Disconnect(ctx, p, web)))
	grants.err = nil
	store.failOn = "delete"
	assert.Equal(t, apperror.CodeUnexpected, codeOf(s.Disconnect(ctx, p, web)))
}

func TestService_ConnectedApps(t *testing.T) {
	store := newFakeStore()
	s, now := newTestService(store)
	ctx, p := context.Background(), Project{OrgID: uuid.New(), ID: uuid.New()}
	user, web, cli, rs := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	_, err := s.Grant(ctx, p, GrantRequest{Key: Key{EndUserID: user, ClientID: web, ResourceServerID: &rs}, Scopes: []string{"read", "email"}})
	require.NoError(t, err)
	_, err = s.Grant(ctx, p, GrantRequest{Key: Key{EndUserID: user, ClientID: web}, Scopes: []string{"openid", "email"}})
	require.NoError(t, err)
	grants := &fakeGrants{families: []Family{
		{
			ClientID: web, ResourceServerID: &rs, Scopes: []string{"openid", "read", "write", "profile"},
			CreatedAt: now.Add(time.Hour), LastUsedAt: now.Add(2 * time.Hour),
		},
		{ClientID: cli, Scopes: []string{"openid", "offline_access"}, CreatedAt: now.Add(-time.Hour), LastUsedAt: now.Add(3 * time.Hour)},
	}}
	s = s.WithGrants(grants, fakeTx{})
	apps, err := s.ConnectedApps(ctx, p, user)
	require.NoError(t, err)
	require.Len(t, apps, 2)
	assert.Equal(t, cli, apps[0].ClientID, "the latest use first")
	assert.False(t, apps[0].Consented, "an implicit first-party app has no consent")
	assert.True(t, apps[0].Offline)
	assert.Equal(t, web, apps[1].ClientID)
	assert.True(t, apps[1].Consented)
	assert.Equal(t, *now, apps[1].ConnectedAt)
	assert.Equal(t, now.Add(2*time.Hour), apps[1].LastUsedAt)
	assert.Equal(t, []Resource{
		{ResourceServerID: &rs, Scopes: []string{"read", "write"}}, {Scopes: []string{"email", "openid", "profile"}},
	}, apps[1].Resources, "A14: the OIDC scopes are the sign-in line, never under the resource server")

	grants.err = errFake
	_, err = s.ConnectedApps(ctx, p, user)
	assert.ErrorIs(t, err, errFake)
	store.failOn = "list"
	_, err = s.ConnectedApps(ctx, p, user)
	assert.Equal(t, apperror.CodeUnexpected, codeOf(err))
}

func codeOf(err error) apperror.Code {
	if ae, ok := apperror.AsAppError(err); ok {
		return ae.Code()
	}
	return ""
}

// M10, CIMD §8.4: a changed metadata document drops every consent of the client.
func TestService_ForgetClient(t *testing.T) {
	store := newFakeStore()
	s, _ := newTestService(store)
	ctx, p := context.Background(), Project{OrgID: uuid.New(), ID: uuid.New()}
	client, other := uuid.New(), uuid.New()
	for _, c := range []uuid.UUID{client, client, other} {
		_, err := s.Grant(ctx, p, GrantRequest{Key: Key{EndUserID: uuid.New(), ClientID: c}, Scopes: []string{"openid"}})
		require.NoError(t, err)
	}
	require.NoError(t, s.ForgetClient(ctx, p, client))
	assert.Len(t, store.rows, 1)
	store.failOn = "deleteclient"
	assert.Error(t, s.ForgetClient(ctx, p, client))
}

func TestService_StoreFailures(t *testing.T) {
	store := newFakeStore()
	s, _ := newTestService(store)
	ctx, p := context.Background(), Project{}
	for _, on := range []string{"find", "upsert"} {
		store.failOn = on
		_, findErr := s.Find(ctx, p, Key{})
		_, grantErr := s.Grant(ctx, p, GrantRequest{Scopes: []string{"openid"}})
		assert.Error(t, errors.Join(findErr, grantErr), on)
	}
}
