package client_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"altalune.id/othra/apperror"
	"altalune.id/othra/internal/client"
	"altalune.id/othra/internal/testutil/fakes"
)

type issuerLock struct {
	locked []uuid.UUID
	err    error
}

func (l *issuerLock) LockIssuerHandle(_ context.Context, projectID uuid.UUID) error {
	if l.err != nil {
		return l.err
	}
	l.locked = append(l.locked, projectID)
	return nil
}

type harness struct {
	store   *fakes.ClientStore
	issuer  *issuerLock
	svc     *client.Service
	org     uuid.UUID
	project uuid.UUID
	now     time.Time
	logs    *bytes.Buffer
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	h := &harness{
		store: fakes.NewClientStore(), issuer: &issuerLock{}, org: uuid.New(), project: uuid.New(),
		now: time.Date(2026, 10, 20, 9, 0, 0, 0, time.UTC), logs: &bytes.Buffer{},
	}
	h.svc = client.NewService(h.store, fakes.Transactor{}, h.issuer, apperror.NewReporter(nil, false).Unexpected).
		WithLogger(slog.New(slog.NewJSONHandler(h.logs, nil)))
	h.svc.SetNow(func() time.Time { return h.now })
	return h
}

func web(uris ...string) client.Settings {
	if len(uris) == 0 {
		uris = []string{"https://app.example/callback"}
	}
	return client.Settings{Name: " Web ", RedirectURIs: uris, PostLogoutRedirectURIs: []string{"https://app.example/"}}
}

func cli() client.Settings {
	return client.Settings{Name: "CLI", RedirectURIs: []string{"http://127.0.0.1/callback"}, RefreshTokens: true}
}

func (h *harness) create(t *testing.T, kind client.Kind, s client.Settings) *client.Client {
	t.Helper()
	c, err := h.svc.Create(context.Background(), h.org, h.project, kind, s)
	require.NoError(t, err)
	return c
}

func codeOf(err error) apperror.Code {
	if ae, ok := apperror.AsAppError(err); ok {
		return ae.Code()
	}
	return ""
}

func TestCreate_WebClient(t *testing.T) {
	h := newHarness(t)
	c := h.create(t, client.KindWeb, web())
	assert.Len(t, c.PublicID, 21)
	assert.Equal(t, "Web", c.Name)
	assert.Equal(t, "confidential", c.ClientType)
	assert.Equal(t, "web", c.ApplicationType)
	assert.Equal(t, client.AuthClientSecretBasic, c.TokenEndpointAuthMethod)
	assert.Equal(t, []string{"authorization_code"}, c.GrantTypes)
	assert.Equal(t, []string{"openid", "profile", "email"}, c.AllowedScopes)
	assert.Equal(t, client.ConsentRequire, c.ConsentPolicy)
	assert.Empty(t, c.Secrets, "create never makes a secret")
	assert.Zero(t, h.store.SecretCount())
	assert.Equal(t, []uuid.UUID{h.project}, h.issuer.locked, "the first client locks the issuer handle")

	got, err := h.svc.Get(context.Background(), h.org, h.project, c.PublicID)
	require.NoError(t, err)
	assert.Equal(t, c.RedirectURIs, got.RedirectURIs)
}

func TestCreate_CLIClient(t *testing.T) {
	h := newHarness(t)
	c := h.create(t, client.KindCLI, cli())
	assert.Equal(t, "public", c.ClientType)
	assert.Equal(t, "native", c.ApplicationType)
	assert.Equal(t, client.AuthNone, c.TokenEndpointAuthMethod)
	assert.Equal(t, []string{"authorization_code", "refresh_token"}, c.GrantTypes)
	assert.Contains(t, c.AllowedScopes, "offline_access")
	assert.Equal(t, []string{}, c.PostLogoutRedirectURIs)
}

func TestCreate_RefusesBadSettings(t *testing.T) {
	h := newHarness(t)
	_, err := h.svc.Create(context.Background(), h.org, h.project, client.KindWeb, web("https://App.example/cb"))
	e, ok := errors.AsType[*client.InvalidRedirectURIError](err)
	require.True(t, ok, "%v", err)
	assert.Equal(t, "redirect_uris[0]", e.Field)
	assert.Equal(t, client.RedirectURINotCanonical, e.Reason)
	assert.Equal(t, "https://app.example/cb", e.Canonical)

	s := web()
	s.PostLogoutRedirectURIs = []string{"http://app.example/"}
	_, err = h.svc.Create(context.Background(), h.org, h.project, client.KindWeb, s)
	e, ok = errors.AsType[*client.InvalidRedirectURIError](err)
	require.True(t, ok, "%v", err)
	assert.Equal(t, "post_logout_redirect_uris[0]", e.Field)
	assert.Equal(t, client.RedirectURIHTTPSRequired, e.Reason)

	s = cli()
	s.PostLogoutRedirectURIs = []string{"http://127.0.0.1/bye"}
	_, err = h.svc.Create(context.Background(), h.org, h.project, client.KindCLI, s)
	assert.Equal(t, apperror.CodeClientInvalidSettings, codeOf(err))

	_, err = h.svc.Create(context.Background(), h.org, h.project, client.KindCLI, client.Settings{Name: "CLI", RedirectURIs: []string{"http://127.0.0.1:9000/cb"}})
	assert.Equal(t, apperror.CodeClientInvalidRedirectURI, codeOf(err))

	_, err = h.svc.Create(context.Background(), h.org, h.project, client.KindWeb, client.Settings{Name: "Web"})
	assert.Equal(t, apperror.CodeClientInvalidRedirectURI, codeOf(err))
	assert.Empty(t, h.issuer.locked, "a refused client locks nothing")
}

func TestCreate_AuthMethodAndPKCE(t *testing.T) {
	h := newHarness(t)
	off := false
	c := h.create(t, client.KindWeb, web())
	assert.True(t, c.PKCERequired, "PKCE is on by default")
	assert.True(t, c.UsesSecret())

	s := web()
	s.AuthMethod, s.PKCERequired = client.AuthClientSecretPost, &off
	c = h.create(t, client.KindWeb, s)
	assert.Equal(t, client.AuthClientSecretPost, c.TokenEndpointAuthMethod)
	assert.False(t, c.PKCERequired, "a confidential client may turn PKCE off")
	assert.True(t, c.UsesSecret())

	c = h.create(t, client.KindCLI, cli())
	assert.True(t, c.PKCERequired)
	assert.False(t, c.UsesSecret())

	for name, tc := range map[string]struct {
		mutate func(s *client.Settings)
		reason client.SettingsReason
	}{
		"a native client keeps PKCE":    {mutate: func(s *client.Settings) { s.PKCERequired = &off }, reason: client.SettingsPKCENative},
		"a native client has no secret": {mutate: func(s *client.Settings) { s.AuthMethod = client.AuthClientSecretBasic }, reason: client.SettingsAuthMethodNative},
	} {
		t.Run(name, func(t *testing.T) {
			s := cli()
			tc.mutate(&s)
			_, err := h.svc.Create(context.Background(), h.org, h.project, client.KindCLI, s)
			e, ok := errors.AsType[*client.InvalidSettingsError](err)
			require.True(t, ok, "%v", err)
			assert.Equal(t, tc.reason, e.Reason)
		})
	}

	s = web()
	s.AuthMethod = client.AuthNone
	_, err := h.svc.Create(context.Background(), h.org, h.project, client.KindWeb, s)
	assert.Equal(t, apperror.CodeValidation, codeOf(err), "a web client authenticates with a secret")
}

// The console puts a redirect URI error on its row: meta.field names the list and the index, meta.uri the URI.
func TestCreate_RedirectURIErrorNamesTheRow(t *testing.T) {
	h := newHarness(t)
	bad := "https://app.example/cb#frag"
	_, err := h.svc.Create(context.Background(), h.org, h.project, client.KindWeb,
		web("https://app.example/a", "https://app.example/b", bad))
	ae, ok := apperror.AsAppError(err)
	require.True(t, ok, "%v", err)
	assert.Equal(t, apperror.CodeClientInvalidRedirectURI, ae.Code())
	assert.Equal(t, map[string]string{"field": "redirect_uris[2]", "uri": bad, "reason": "fragment"}, ae.Meta())

	s := web()
	s.PostLogoutRedirectURIs = []string{"https://app.example/", "https://app.example/"}
	_, err = h.svc.Create(context.Background(), h.org, h.project, client.KindWeb, s)
	ae, _ = apperror.AsAppError(err)
	assert.Equal(t, "post_logout_redirect_uris[1]", ae.Meta()["field"])
	assert.Equal(t, "duplicate", ae.Meta()["reason"])

	_, err = h.svc.Create(context.Background(), h.org, h.project, client.KindWeb, client.Settings{Name: "Web"})
	ae, _ = apperror.AsAppError(err)
	assert.Equal(t, map[string]string{"field": "redirect_uris", "reason": "required"}, ae.Meta(), "a list-wide error names the list")
}

func TestCreate_ChecksTheName(t *testing.T) {
	h := newHarness(t)
	for name, want := range map[string]map[string]string{
		"   ":                    {"field": "name", "reason": "required"},
		strings.Repeat("é", 101): {"field": "name", "reason": "too_long", "param": "100"},
	} {
		s := web()
		s.Name = name
		_, err := h.svc.Create(context.Background(), h.org, h.project, client.KindWeb, s)
		ae, ok := apperror.AsAppError(err)
		require.True(t, ok, "%v", err)
		assert.Equal(t, apperror.CodeValidation, ae.Code())
		assert.Equal(t, want, ae.Meta())
	}
	s := web()
	s.Name = " " + strings.Repeat("é", 100) + " "
	c, err := h.svc.Create(context.Background(), h.org, h.project, client.KindWeb, s)
	require.NoError(t, err)
	assert.Equal(t, strings.Repeat("é", 100), c.Name)
}

func TestCreate_AFailedInsertLeavesTheHandleUnlocked(t *testing.T) {
	h := newHarness(t)
	h.store.FailOn, h.store.Err = "Insert", errors.New("boom")
	_, err := h.svc.Create(context.Background(), h.org, h.project, client.KindWeb, web())
	assert.Equal(t, apperror.CodeUnexpected, codeOf(err))
	assert.Empty(t, h.issuer.locked)

	h = newHarness(t)
	h.issuer.err = errors.New("lock failed")
	_, err = h.svc.Create(context.Background(), h.org, h.project, client.KindWeb, web())
	assert.Equal(t, apperror.CodeUnexpected, codeOf(err))
}

func TestUpdateDeleteList(t *testing.T) {
	h := newHarness(t)
	c := h.create(t, client.KindWeb, web())
	h.now = h.now.Add(time.Minute)
	s := web("https://app.example/a", "http://localhost:3000/cb")
	s.RefreshTokens, s.ConsentPolicy = true, client.ConsentImplicit
	got, err := h.svc.Update(context.Background(), h.org, h.project, c.PublicID, s)
	require.NoError(t, err)
	assert.Equal(t, s.RedirectURIs, got.RedirectURIs)
	assert.True(t, got.RefreshTokens())
	assert.Equal(t, client.ConsentImplicit, got.ConsentPolicy)
	assert.Equal(t, h.now, got.UpdatedAt)
	assert.Equal(t, client.KindWeb, got.Kind(), "the kind never changes")

	_, err = h.svc.Update(context.Background(), h.org, h.project, c.PublicID, web("ftp://x/"))
	assert.Equal(t, apperror.CodeClientInvalidRedirectURI, codeOf(err))
	_, err = h.svc.Update(context.Background(), h.org, h.project, "nope", web())
	assert.Equal(t, apperror.CodeClientNotFound, codeOf(err))

	h.create(t, client.KindCLI, cli())
	list, err := h.svc.List(context.Background(), h.org, h.project)
	require.NoError(t, err)
	assert.Len(t, list, 2)
	other, err := h.svc.List(context.Background(), h.org, uuid.New())
	require.NoError(t, err)
	assert.Empty(t, other, "another project has none")

	require.NoError(t, h.svc.Delete(context.Background(), h.org, h.project, c.PublicID))
	_, err = h.svc.Get(context.Background(), h.org, h.project, c.PublicID)
	assert.Equal(t, apperror.CodeClientNotFound, codeOf(err))
	assert.Equal(t, apperror.CodeClientNotFound, codeOf(h.svc.Delete(context.Background(), h.org, h.project, c.PublicID)))

	h.store.FailOn, h.store.Err = "List", errors.New("boom")
	_, err = h.svc.List(context.Background(), h.org, h.project)
	assert.Equal(t, apperror.CodeUnexpected, codeOf(err))
}

func TestGenerateSecret(t *testing.T) {
	h := newHarness(t)
	c := h.create(t, client.KindWeb, web())
	got, secret, err := h.svc.GenerateSecret(context.Background(), h.org, h.project, c.PublicID)
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(secret, client.SecretPrefix))
	assert.Len(t, secret, len(client.SecretPrefix)+43, "32 bytes of base64url")
	require.Len(t, got.Secrets, 1)
	assert.True(t, got.Secrets[0].Current())
	assert.Equal(t, secret[len(secret)-4:], got.Secrets[0].Hint)
	assert.Equal(t, client.HashSecret(secret), h.store.Hashes[got.Secrets[0].ID])
	assert.Regexp(t, `^sha256\$[0-9a-f]{64}$`, h.store.Hashes[got.Secrets[0].ID])
	assert.True(t, client.SecretMatches(h.store.Hashes[got.Secrets[0].ID], secret))
	assert.False(t, client.SecretMatches(h.store.Hashes[got.Secrets[0].ID], secret+"x"))

	_, _, err = h.svc.GenerateSecret(context.Background(), h.org, h.project, c.PublicID)
	assert.Equal(t, apperror.CodeClientSecretExists, codeOf(err))

	cl := h.create(t, client.KindCLI, cli())
	_, _, err = h.svc.GenerateSecret(context.Background(), h.org, h.project, cl.PublicID)
	assert.Equal(t, apperror.CodeClientHasNoSecret, codeOf(err), "a CLI client refuses a secret")
	_, _, err = h.svc.RotateSecret(context.Background(), h.org, h.project, cl.PublicID, time.Hour)
	assert.Equal(t, apperror.CodeClientHasNoSecret, codeOf(err))
	_, err = h.svc.RevokeSecret(context.Background(), h.org, h.project, cl.PublicID, "x")
	assert.Equal(t, apperror.CodeClientHasNoSecret, codeOf(err))
	_, _, err = h.svc.GenerateSecret(context.Background(), h.org, h.project, "nope")
	assert.Equal(t, apperror.CodeClientNotFound, codeOf(err))
}

func TestRotateSecret_KeepsAtMostTwoLive(t *testing.T) {
	h := newHarness(t)
	c := h.create(t, client.KindWeb, web())
	_, first, err := h.svc.GenerateSecret(context.Background(), h.org, h.project, c.PublicID)
	require.NoError(t, err)

	h.now = h.now.Add(time.Minute)
	got, second, err := h.svc.RotateSecret(context.Background(), h.org, h.project, c.PublicID, 24*time.Hour)
	require.NoError(t, err)
	assert.NotEqual(t, first, second)
	require.Len(t, got.Secrets, 2)
	assert.True(t, got.Secrets[0].Current(), "newest first")
	require.NotNil(t, got.Secrets[1].ExpiresAt)
	assert.Equal(t, h.now.Add(24*time.Hour), *got.Secrets[1].ExpiresAt, "the old one keeps working for the grace")

	h.now = h.now.Add(time.Minute)
	got, _, err = h.svc.RotateSecret(context.Background(), h.org, h.project, c.PublicID, time.Hour)
	require.NoError(t, err)
	require.Len(t, got.Secrets, 2, "rotating during a grace ends the older secret at once")
	assert.Equal(t, h.now.Add(time.Hour), *got.Secrets[1].ExpiresAt)
	assert.Equal(t, 2, h.store.SecretCount())

	h.now = h.now.Add(time.Minute)
	got, _, err = h.svc.RotateSecret(context.Background(), h.org, h.project, c.PublicID, 0)
	require.NoError(t, err)
	assert.Len(t, got.Secrets, 1, "a rotation with no grace ends every old secret")
	assert.Equal(t, 1, h.store.SecretCount())

	h.now = h.now.Add(time.Minute)
	got, _, err = h.svc.RotateSecret(context.Background(), h.org, h.project, c.PublicID, 30*24*time.Hour)
	require.NoError(t, err)
	assert.Equal(t, h.now.Add(client.MaxGrace), *got.Secrets[1].ExpiresAt, "the grace is capped at 7 days")

	h.now = h.now.Add(client.MaxGrace)
	got, err = h.svc.Get(context.Background(), h.org, h.project, c.PublicID)
	require.NoError(t, err)
	assert.Len(t, got.Secrets, 1, "an expired secret is no longer live")
}

// SECURITY: R-5, a secret authenticates only while live: the current one and the old one through its grace, never a revoked or expired one.
func TestVerifySecret(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	c := h.create(t, client.KindWeb, web())
	_, first, err := h.svc.GenerateSecret(ctx, h.org, h.project, c.PublicID)
	require.NoError(t, err)
	_, second, err := h.svc.RotateSecret(ctx, h.org, h.project, c.PublicID, time.Hour)
	require.NoError(t, err)

	got, err := h.svc.Authenticate(ctx, h.org, h.project, c.PublicID)
	require.NoError(t, err)
	assert.True(t, got.VerifySecret(second, h.now), "the current secret")
	assert.True(t, got.VerifySecret(first, h.now), "the old one in its grace")
	assert.False(t, got.VerifySecret(first+"x", h.now))
	assert.False(t, got.VerifySecret("", h.now))
	assert.False(t, got.VerifySecret(first, h.now.Add(time.Hour)), "the grace ends at its expiry")

	plain, err := h.svc.Get(ctx, h.org, h.project, c.PublicID)
	require.NoError(t, err)
	assert.False(t, plain.VerifySecret(second, h.now), "only an authentication read holds the hashes")

	h.now = h.now.Add(time.Hour)
	got, err = h.svc.Authenticate(ctx, h.org, h.project, c.PublicID)
	require.NoError(t, err)
	assert.False(t, got.VerifySecret(first, h.now), "an expired secret is not read")
	assert.True(t, got.VerifySecret(second, h.now))

	native := h.create(t, client.KindCLI, cli())
	got, err = h.svc.Authenticate(ctx, h.org, h.project, native.PublicID)
	require.NoError(t, err)
	assert.False(t, got.VerifySecret(second, h.now), "a public client has no secret")

	_, err = h.svc.Authenticate(ctx, h.org, h.project, "nope")
	assert.True(t, client.IsNotFoundError(err))
}

func TestRevokeSecret(t *testing.T) {
	h := newHarness(t)
	c := h.create(t, client.KindWeb, web())
	_, _, err := h.svc.GenerateSecret(context.Background(), h.org, h.project, c.PublicID)
	require.NoError(t, err)
	got, _, err := h.svc.RotateSecret(context.Background(), h.org, h.project, c.PublicID, time.Hour)
	require.NoError(t, err)
	old := got.Secrets[1]

	got, err = h.svc.RevokeSecret(context.Background(), h.org, h.project, c.PublicID, old.PublicID)
	require.NoError(t, err)
	require.Len(t, got.Secrets, 1)
	assert.True(t, got.Secrets[0].Current())

	_, err = h.svc.RevokeSecret(context.Background(), h.org, h.project, c.PublicID, old.PublicID)
	assert.Equal(t, apperror.CodeClientSecretNotFound, codeOf(err))

	got, err = h.svc.RevokeSecret(context.Background(), h.org, h.project, c.PublicID, got.Secrets[0].PublicID)
	require.NoError(t, err)
	assert.Empty(t, got.Secrets, "revoking the current secret leaves a web client to generate a new one")
	_, _, err = h.svc.GenerateSecret(context.Background(), h.org, h.project, c.PublicID)
	assert.NoError(t, err)
}

// SECURITY: the plaintext never reaches a log line; each secret change logs one structured line.
func TestSecrets_NeverLogThePlaintext(t *testing.T) {
	h := newHarness(t)
	c := h.create(t, client.KindWeb, web())
	_, first, err := h.svc.GenerateSecret(context.Background(), h.org, h.project, c.PublicID)
	require.NoError(t, err)
	got, second, err := h.svc.RotateSecret(context.Background(), h.org, h.project, c.PublicID, time.Hour)
	require.NoError(t, err)
	_, err = h.svc.RevokeSecret(context.Background(), h.org, h.project, c.PublicID, got.Secrets[1].PublicID)
	require.NoError(t, err)
	logs := h.logs.String()
	assert.Equal(t, 3, strings.Count(logs, "\n"), logs)
	for _, secret := range []string{first, second, client.HashSecret(first), client.HashSecret(second)} {
		assert.NotContains(t, logs, secret)
		assert.NotContains(t, logs, secret[len(client.SecretPrefix):])
	}
	assert.Contains(t, logs, `"client":"`+c.PublicID+`"`)
	assert.Contains(t, logs, "client: rotate secret")
}

func TestSecrets_ReportUnexpectedFailures(t *testing.T) {
	h := newHarness(t)
	c := h.create(t, client.KindWeb, web())
	h.svc.SetNewSecret(func() (string, string, string, error) { return "", "", "", errors.New("no entropy") })
	_, secret, err := h.svc.GenerateSecret(context.Background(), h.org, h.project, c.PublicID)
	assert.Equal(t, apperror.CodeUnexpected, codeOf(err))
	assert.Empty(t, secret)

	h = newHarness(t)
	c = h.create(t, client.KindWeb, web())
	h.store.FailOn, h.store.Err = "InsertSecret", errors.New("boom")
	_, secret, err = h.svc.RotateSecret(context.Background(), h.org, h.project, c.PublicID, time.Hour)
	assert.Equal(t, apperror.CodeUnexpected, codeOf(err))
	assert.Empty(t, secret, "a failed rotation returns no secret")
}
