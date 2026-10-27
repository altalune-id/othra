package upstream

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"altalune.id/othra/apperror"
	"altalune.id/othra/sealer"
)

type harness struct {
	svc     *Service
	store   *fakeStore
	guard   *fakeGuard
	seal    sealer.Sealer
	org     uuid.UUID
	project uuid.UUID
	logs    *bytes.Buffer
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	return newHarnessWith(t, fakeIssuers{url: "https://id.example.com/p/acme"})
}

func newHarnessWith(t *testing.T, issuers Issuers) *harness {
	t.Helper()
	key, err := sealer.GenerateKey()
	require.NoError(t, err)
	seal, err := sealer.New(key)
	require.NoError(t, err)
	h := &harness{store: newFakeStore(), guard: &fakeGuard{on: true}, seal: seal, org: uuid.New(), project: uuid.New(), logs: &bytes.Buffer{}}
	h.svc = NewService(h.store, passTx{}, seal, h.guard, issuers, apperror.NewReporter(nil, false).Unexpected).
		WithLogger(slog.New(slog.NewJSONHandler(h.logs, nil))).
		WithClock(func() time.Time { return time.Date(2026, 10, 24, 9, 0, 0, 0, time.UTC) })
	return h
}

func codeOf(err error) apperror.Code {
	if app, ok := apperror.AsAppError(err); ok {
		return app.Code()
	}
	return ""
}

func (h *harness) create(t *testing.T) *Upstream {
	t.Helper()
	u, err := h.svc.Create(context.Background(), h.org, h.project, CreateRequest{
		Kind: KindGoogle, ClientID: " 123.apps.googleusercontent.com ", ClientSecret: "GOCSPX-secret", Enabled: true,
	})
	require.NoError(t, err)
	return u
}

// the secret is sealed with AAD bound to the project and the row, and only Credentials opens it.
func TestService_CreateSealsTheSecret(t *testing.T) {
	h := newHarness(t)
	u := h.create(t)
	assert.Equal(t, "123.apps.googleusercontent.com", u.ClientID)
	assert.Len(t, u.PublicID, 21)
	assert.True(t, u.Enabled)
	sealed := h.store.secrets[u.ID]
	assert.NotContains(t, string(sealed), "GOCSPX")
	_, err := h.seal.Open(sealed, []byte("upstream_provider:"+uuid.NewString()+":"+u.ID.String()+":client_secret"))
	assert.Error(t, err, "the AAD binds the project")
	assert.NotContains(t, h.logs.String(), "GOCSPX")

	c, err := h.svc.Credentials(context.Background(), h.org, h.project, KindGoogle)
	require.NoError(t, err)
	assert.Equal(t, "GOCSPX-secret", c.ClientSecret)
	assert.Equal(t, u.ID, c.Upstream.ID)

	_, err = h.svc.Create(context.Background(), h.org, h.project, CreateRequest{Kind: KindGoogle, ClientID: "x", ClientSecret: "y"})
	assert.Equal(t, apperror.CodeUpstreamExists, codeOf(err))
}

// UPS002: the client id is 1 to 255 characters and a secret is required to create.
func TestService_InvalidConfig(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	for name, req := range map[string]CreateRequest{
		"no client id":   {Kind: KindGoogle, ClientID: "  ", ClientSecret: "s"},
		"long client id": {Kind: KindGoogle, ClientID: strings.Repeat("a", 256), ClientSecret: "s"},
		"no secret":      {Kind: KindGoogle, ClientID: "c"},
		"long secret":    {Kind: KindGoogle, ClientID: "c", ClientSecret: strings.Repeat("s", maxSecretLen+1)},
	} {
		_, err := h.svc.Create(ctx, h.org, h.project, req)
		assert.Equal(t, apperror.CodeUpstreamInvalidConfig, codeOf(err), name)
	}
	_, err := h.svc.Create(ctx, h.org, h.project, CreateRequest{Kind: "github", ClientID: "c", ClientSecret: "s"})
	assert.Equal(t, apperror.CodeUpstreamInvalidConfig, codeOf(err), "an unknown kind")
}

// updating keeps the secret unless a new one is given.
func TestService_Update(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	u := h.create(t)
	got, err := h.svc.Update(ctx, h.org, h.project, u.PublicID, "new-client", "")
	require.NoError(t, err)
	assert.Equal(t, "new-client", got.ClientID)
	c, err := h.svc.Credentials(ctx, h.org, h.project, KindGoogle)
	require.NoError(t, err)
	assert.Equal(t, "GOCSPX-secret", c.ClientSecret, "an empty secret keeps the stored one")
	_, err = h.svc.Update(ctx, h.org, h.project, u.PublicID, "new-client", "rotated")
	require.NoError(t, err)
	c, err = h.svc.Credentials(ctx, h.org, h.project, KindGoogle)
	require.NoError(t, err)
	assert.Equal(t, "rotated", c.ClientSecret)
	_, err = h.svc.Update(ctx, h.org, h.project, "nope", "c", "")
	assert.Equal(t, apperror.CodeUpstreamNotFound, codeOf(err))
}

// PRJ010: Google can't be turned off or removed while the email link is off; UPS004: never while linked.
func TestService_DisableAndDeleteGuards(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	u := h.create(t)
	h.guard.on = false
	_, err := h.svc.SetEnabled(ctx, h.org, h.project, u.PublicID, false)
	assert.Equal(t, apperror.CodeProjectSignInLastMethod, codeOf(err))
	assert.Equal(t, apperror.CodeProjectSignInLastMethod, codeOf(h.svc.Delete(ctx, h.org, h.project, u.PublicID)))
	assert.Equal(t, []uuid.UUID{h.project, h.project}, h.guard.locked, "the project row is locked for the decision")

	h.guard.on = true
	off, err := h.svc.SetEnabled(ctx, h.org, h.project, u.PublicID, false)
	require.NoError(t, err)
	assert.False(t, off.Enabled)
	offered, err := h.svc.Offered(ctx, h.org, h.project)
	require.NoError(t, err)
	assert.Empty(t, offered, "a disabled upstream is not offered")
	enabled, err := h.svc.AnyEnabled(ctx, h.org, h.project)
	require.NoError(t, err)
	assert.False(t, enabled)
	_, err = h.svc.Credentials(ctx, h.org, h.project, KindGoogle)
	assert.True(t, IsNotFoundError(err), "a disabled upstream gives no credentials")
	on, err := h.svc.SetEnabled(ctx, h.org, h.project, u.PublicID, true)
	require.NoError(t, err)
	assert.True(t, on.Enabled)

	offered, err = h.svc.Offered(ctx, h.org, h.project)
	require.NoError(t, err)
	assert.Equal(t, []Kind{KindGoogle}, offered)
	enabled, err = h.svc.AnyEnabled(ctx, h.org, h.project)
	require.NoError(t, err)
	assert.True(t, enabled)
	h.store.linked[u.ID] = true
	assert.Equal(t, apperror.CodeUpstreamHasIdentities, codeOf(h.svc.Delete(ctx, h.org, h.project, u.PublicID)))
	h.store.linked[u.ID] = false
	require.NoError(t, h.svc.Delete(ctx, h.org, h.project, u.PublicID))
	list, err := h.svc.List(ctx, h.org, h.project)
	require.NoError(t, err)
	assert.Empty(t, list)
}

// install seeds Google from config once; a console edit is never overwritten.
func TestService_Seed(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	require.NoError(t, h.svc.Seed(ctx, h.org, h.project, KindGoogle, "", ""), "unconfigured: nothing")
	list, err := h.svc.List(ctx, h.org, h.project)
	require.NoError(t, err)
	assert.Empty(t, list)

	require.NoError(t, h.svc.Seed(ctx, h.org, h.project, KindGoogle, "seed-client", "seed-secret"))
	c, err := h.svc.Credentials(ctx, h.org, h.project, KindGoogle)
	require.NoError(t, err)
	assert.Equal(t, "seed-client", c.Upstream.ClientID)
	assert.Equal(t, "seed-secret", c.ClientSecret)

	_, err = h.svc.Update(ctx, h.org, h.project, c.Upstream.PublicID, "console-client", "")
	require.NoError(t, err)
	require.NoError(t, h.svc.Seed(ctx, h.org, h.project, KindGoogle, "seed-client", "seed-secret"))
	c, err = h.svc.Credentials(ctx, h.org, h.project, KindGoogle)
	require.NoError(t, err)
	assert.Equal(t, "console-client", c.Upstream.ClientID, "never overwrites")
	assert.Contains(t, h.logs.String(), "client id differs")
}

func TestService_StoreFailures(t *testing.T) {
	ctx := context.Background()
	for _, on := range []string{"list", "get", "lock", "bykind", "secret", "update", "delete", "insert"} {
		h := newHarness(t)
		u := h.create(t)
		h.store.failOn = on
		_, e1 := h.svc.List(ctx, h.org, h.project)
		_, e2 := h.svc.Get(ctx, h.org, h.project, u.PublicID)
		_, e3 := h.svc.Credentials(ctx, h.org, h.project, KindGoogle)
		_, e4 := h.svc.SetEnabled(ctx, h.org, h.project, u.PublicID, true)
		e5 := h.svc.Delete(ctx, h.org, h.project, u.PublicID)
		_, e6 := h.svc.Create(ctx, h.org, uuid.New(), CreateRequest{Kind: KindGoogle, ClientID: "c", ClientSecret: "s"})
		e7 := h.svc.Seed(ctx, h.org, uuid.New(), KindGoogle, "c", "s")
		got := []error{e1, e2, e3, e4, e5, e6, e7}
		found := false
		for _, err := range got {
			if codeOf(err) == apperror.CodeUnexpected {
				found = true
			}
		}
		assert.True(t, found, on)
	}
	h := newHarness(t)
	u := h.create(t)
	h.guard.err = errFake
	_, err := h.svc.SetEnabled(ctx, h.org, h.project, u.PublicID, false)
	assert.Equal(t, apperror.CodeUnexpected, codeOf(err))
}
