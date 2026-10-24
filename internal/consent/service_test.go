package consent

import (
	"context"
	"errors"
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

func (f *fakeStore) Delete(_ context.Context, _ uuid.UUID, key Key) (bool, error) {
	if f.failOn == "delete" {
		return false, errFake
	}
	_, ok := f.rows[rowKey(key)]
	delete(f.rows, rowKey(key))
	return ok, nil
}

type fakeGrants struct{ revoked []Key }

func (g *fakeGrants) RevokeConsent(_ context.Context, _ Project, key Key) error {
	g.revoked = append(g.revoked, key)
	return nil
}

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

// C9: a revoke deletes the consent and revokes what the client holds for it.
func TestService_Revoke(t *testing.T) {
	store, grants := newFakeStore(), &fakeGrants{}
	s, _ := newTestService(store)
	s = s.WithGrants(grants)
	ctx, p := context.Background(), Project{OrgID: uuid.New(), ID: uuid.New()}
	key := Key{EndUserID: uuid.New(), ClientID: uuid.New()}
	_, err := s.Grant(ctx, p, GrantRequest{Key: key, Scopes: []string{"openid"}})
	require.NoError(t, err)
	require.NoError(t, s.Revoke(ctx, p, key))
	got, err := s.Find(ctx, p, key)
	require.NoError(t, err)
	assert.Nil(t, got)
	assert.Equal(t, []Key{key}, grants.revoked)
	require.NoError(t, s.Revoke(ctx, p, key), "revoking twice is no error")
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
	for _, on := range []string{"find", "upsert", "delete"} {
		store.failOn = on
		_, findErr := s.Find(ctx, p, Key{})
		_, grantErr := s.Grant(ctx, p, GrantRequest{Scopes: []string{"openid"}})
		revokeErr := s.Revoke(ctx, p, Key{})
		assert.Error(t, errors.Join(findErr, grantErr, revokeErr), on)
	}
}
