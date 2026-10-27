package client_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"altalune.id/othra/apperror"
	"altalune.id/othra/internal/client"
)

const (
	consoleRedirect   = "http://localhost:5150/oauth/callback"
	consolePostLogout = "http://localhost:5150/console/"
)

func (h *harness) system(t *testing.T) *client.Client {
	t.Helper()
	c, err := h.svc.CreateSystem(context.Background(), h.org, h.project, consoleRedirect, consolePostLogout)
	require.NoError(t, err)
	return c
}

// The console client is first-party and public: PKCE, implicit consent, no secret, one redirect and one sign-out URI.
func TestCreateSystem(t *testing.T) {
	h := newHarness(t)
	c := h.system(t)
	assert.True(t, c.System)
	assert.False(t, c.Managed())
	assert.Equal(t, client.TypePublic, c.ClientType)
	assert.Equal(t, client.AppWeb, c.ApplicationType)
	assert.Equal(t, client.AuthNone, c.TokenEndpointAuthMethod)
	assert.False(t, c.UsesSecret())
	assert.True(t, c.PKCERequired)
	assert.Equal(t, client.ConsentImplicit, c.ConsentPolicy)
	assert.Equal(t, []string{client.GrantAuthorizationCode}, c.GrantTypes)
	assert.Equal(t, []string{"openid", "profile", "email"}, c.AllowedScopes)
	assert.Equal(t, []string{consoleRedirect}, c.RedirectURIs)
	assert.Equal(t, []string{consolePostLogout}, c.PostLogoutRedirectURIs)
	assert.Equal(t, []any{h.project}, toAny(h.issuer.locked), "the first client locks the issuer handle")

	got, err := h.svc.System(context.Background(), h.org, h.project)
	require.NoError(t, err)
	assert.Equal(t, c.PublicID, got.PublicID)

	_, err = h.svc.System(context.Background(), h.org, h.org)
	assert.True(t, client.IsNotFoundError(err), "a project with no console client")

	_, err = h.svc.CreateSystem(context.Background(), h.org, h.project, "http://othra.example/oauth/callback", consolePostLogout)
	assert.Equal(t, apperror.CodeClientInvalidRedirectURI, codeOf(err), "the web rules apply: http only on loopback")
}

// The console client is read-only on every path that changes a client.
func TestSystemClientIsReadOnly(t *testing.T) {
	h := newHarness(t)
	c := h.system(t)
	ctx := context.Background()

	_, err := h.svc.Update(ctx, h.org, h.project, c.PublicID, web())
	assert.Equal(t, apperror.CodeClientSystemManaged, codeOf(err))
	assert.Equal(t, apperror.CodeClientSystemManaged, codeOf(h.svc.Delete(ctx, h.org, h.project, c.PublicID)))
	_, _, err = h.svc.GenerateSecret(ctx, h.org, h.project, c.PublicID)
	assert.Equal(t, apperror.CodeClientSystemManaged, codeOf(err))
	_, _, err = h.svc.RotateSecret(ctx, h.org, h.project, c.PublicID, 0)
	assert.Equal(t, apperror.CodeClientSystemManaged, codeOf(err))
	_, err = h.svc.RevokeSecret(ctx, h.org, h.project, c.PublicID, "s-any")
	assert.Equal(t, apperror.CodeClientSystemManaged, codeOf(err))

	got, err := h.svc.Get(ctx, h.org, h.project, c.PublicID)
	require.NoError(t, err)
	assert.Equal(t, []string{consoleRedirect}, got.RedirectURIs, "nothing changed")
}

// Boot puts the console client's URIs back to the derived ones under its row lock, and only those.
func TestReconcileSystem(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	changed, err := h.svc.ReconcileSystem(ctx, h.org, h.project, consoleRedirect, consolePostLogout)
	require.NoError(t, err)
	assert.False(t, changed, "no console client yet: nothing to do")

	c := h.system(t)
	changed, err = h.svc.ReconcileSystem(ctx, h.org, h.project, consoleRedirect, consolePostLogout)
	require.NoError(t, err)
	assert.False(t, changed, "already matching: no write")

	h.now = h.now.Add(1)
	moved, movedOut := "https://admin.example/oauth/callback", "https://admin.example/console/"
	changed, err = h.svc.ReconcileSystem(ctx, h.org, h.project, moved, movedOut)
	require.NoError(t, err)
	assert.True(t, changed)
	got, err := h.svc.System(ctx, h.org, h.project)
	require.NoError(t, err)
	assert.Equal(t, []string{moved}, got.RedirectURIs)
	assert.Equal(t, []string{movedOut}, got.PostLogoutRedirectURIs)
	assert.Equal(t, c.Name, got.Name)
	assert.Equal(t, c.ConsentPolicy, got.ConsentPolicy)
	assert.Equal(t, h.now.UTC(), got.UpdatedAt)

	_, err = h.svc.ReconcileSystem(ctx, h.org, h.project, "http://othra.example/oauth/callback", movedOut)
	assert.Equal(t, apperror.CodeClientInvalidRedirectURI, codeOf(err), "a public URL the redirect rules refuse fails boot")
	_, err = h.svc.ReconcileSystem(ctx, h.org, h.project, moved, "ftp://admin.example/")
	assert.Error(t, err)

	h.store.FailOn, h.store.Err = "LockSystem", errors.New("db down")
	_, err = h.svc.ReconcileSystem(ctx, h.org, h.project, moved, movedOut)
	assert.Equal(t, apperror.CodeUnexpected, codeOf(err))
}

func toAny[T any](s []T) []any {
	out := make([]any, len(s))
	for i, v := range s {
		out[i] = v
	}
	return out
}

// PRJ011: the console project holds only the client Othra seeds; Create refuses another, of any kind.
func TestCreate_RefusesTheConsoleProject(t *testing.T) {
	h := newHarness(t)
	h.issuer.system = true
	_, err := h.svc.Create(context.Background(), h.org, h.project, client.KindWeb, web())
	assert.Equal(t, apperror.CodeProjectSystemManaged, codeOf(err))
	_, err = h.svc.Create(context.Background(), h.org, h.project, client.KindCLI, cli())
	assert.True(t, client.IsSystemProjectError(err))
	assert.Empty(t, h.issuer.locked, "nothing written")
	_, err = h.svc.CreateSystem(context.Background(), h.org, h.project, consoleRedirect, consolePostLogout)
	require.NoError(t, err, "the seed itself is not a Create")
}
