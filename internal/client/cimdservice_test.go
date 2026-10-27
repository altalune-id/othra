package client_test

import (
	"context"
	"errors"
	"log/slog"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"altalune.id/othra/apperror"
	"altalune.id/othra/internal/client"
	"altalune.id/othra/internal/kernel/ratelimit/memory"
	"altalune.id/othra/internal/web/layout"
)

const claudeURL = "https://claude.ai/oauth/claude-code-client-metadata"

func claudeDoc(name string, redirects ...string) string {
	if len(redirects) == 0 {
		redirects = []string{"http://localhost/callback", "http://127.0.0.1/callback"}
	}
	return `{"client_id":"` + claudeURL + `","client_name":"` + name + `","client_uri":"https://claude.ai",` +
		`"redirect_uris":["` + strings.Join(redirects, `","`) + `"],"grant_types":["authorization_code","refresh_token"],` +
		`"token_endpoint_auth_method":"none"}`
}

type fakeFetcher struct {
	body  string
	cache string
	err   error
	calls int
}

func (f *fakeFetcher) FetchMetadata(_ context.Context, _ string) (*client.FetchedMetadata, error) {
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	return &client.FetchedMetadata{Body: []byte(f.body), ContentType: "application/json", CacheControl: f.cache}, nil
}

type fakeLinks struct {
	links   map[uuid.UUID][]string
	err     error
	changed []uuid.UUID
}

func (l *fakeLinks) LinkMetadataClient(_ context.Context, _, _, clientID, rs uuid.UUID, scopes []string) error {
	if l.err != nil {
		return l.err
	}
	l.links[clientID] = append(scopes, rs.String())
	return nil
}

func (h *harness) withMetadata(f *fakeFetcher, limit bool) *fakeLinks {
	links := &fakeLinks{links: map[uuid.UUID][]string{}}
	limits := layout.Limits{}
	if limit {
		limits.Limiter = memory.New(1000)
	}
	h.build(f, client.MetadataConfig{
		Cache: client.MetadataCache{Min: 5 * time.Minute, Max: 24 * time.Hour, Default: time.Hour}, IssuerHost: "othra.example.com",
	})
	h.svc.WithLinks(links).WithFetchLimits(limits).WithOnMetadataChange(func(_ context.Context, _, _, id uuid.UUID) error {
		links.changed = append(links.changed, id)
		return nil
	})
	return links
}

// a document is fetched once per max-age into a native, public, auth none, require-consent shadow client.
func TestAdmit_ShadowClient(t *testing.T) {
	h := newHarness(t)
	f := &fakeFetcher{body: claudeDoc("Claude Code"), cache: "max-age=600"}
	links := h.withMetadata(f, false)
	ctx, rs := context.Background(), uuid.New()
	req := client.AdmitRequest{URL: claudeURL, ResourceServerID: rs, Scopes: []string{"sample:read"}}

	c, err := h.svc.Admit(ctx, h.org, h.project, req)
	require.NoError(t, err)
	assert.Equal(t, claudeURL, c.OAuthID())
	assert.Len(t, c.PublicID, 21, "a nanoid public id; the URL is the wire client_id")
	assert.True(t, c.Managed())
	assert.Equal(t, "Claude Code", c.Name)
	assert.Equal(t, client.TypePublic, c.ClientType)
	assert.Equal(t, client.AppNative, c.ApplicationType)
	assert.Equal(t, client.AuthNone, c.TokenEndpointAuthMethod)
	assert.Equal(t, client.ConsentRequire, c.ConsentPolicy)
	assert.True(t, c.RefreshTokens())
	assert.Equal(t, []string{"openid", "profile", "email", "offline_access"}, c.AllowedScopes)
	assert.Equal(t, h.now.Add(10*time.Minute), *c.MetadataExpiresAt)
	assert.Equal(t, []string{"sample:read", rs.String()}, links.links[c.ID])

	h.now = h.now.Add(9 * time.Minute)
	again, err := h.svc.Admit(ctx, h.org, h.project, req)
	require.NoError(t, err)
	assert.Equal(t, 1, f.calls, "fresh: no fetch")
	assert.Equal(t, c.PublicID, again.PublicID)

	got, err := h.svc.Get(ctx, h.org, h.project, claudeURL)
	require.NoError(t, err, "Get takes the URL client_id too")
	assert.Equal(t, c.ID, got.ID)
	list, err := h.svc.List(ctx, h.org, h.project)
	require.NoError(t, err)
	assert.Empty(t, list, "shadows aren't listed")

	h.now = h.now.Add(2 * time.Minute)
	f.body = claudeDoc("Claude Code 2")
	refreshed, err := h.svc.Admit(ctx, h.org, h.project, req)
	require.NoError(t, err)
	assert.Equal(t, 2, f.calls)
	assert.Equal(t, c.PublicID, refreshed.PublicID, "the same shadow row")
	assert.Equal(t, "Claude Code 2", refreshed.Name)
	assert.Equal(t, []uuid.UUID{c.ID}, links.changed, "a changed name drops remembered consent")

	h.now = h.now.Add(time.Hour)
	f.err = errors.New("network down")
	_, err = h.svc.Admit(ctx, h.org, h.project, req)
	assert.Equal(t, apperror.CodeAuthorizeClientMetadataUnusable, codeOf(err), "an expired document is never used stale")
	assert.Contains(t, h.logs.String(), `"reason":"fetch"`)
	assert.NotContains(t, h.logs.String(), "network down")
}

// A Service whose resource server links were never set refuses to admit instead of panicking.
func TestAdmit_WithoutLinks(t *testing.T) {
	h := newHarness(t)
	f := &fakeFetcher{body: claudeDoc("Claude Code")}
	h.build(f, client.MetadataConfig{IssuerHost: "othra.example.com"})
	_, err := h.svc.Admit(context.Background(), h.org, h.project, client.AdmitRequest{URL: claudeURL, ResourceServerID: uuid.New()})
	assert.Equal(t, apperror.CodeUnexpected, codeOf(err))
	assert.Zero(t, f.calls, "nothing is fetched")
}

// a bad client_id URL is not admitted and never fetched; a bad document is unusable.
func TestAdmit_Refusals(t *testing.T) {
	h := newHarness(t)
	f := &fakeFetcher{body: `{"client_id":"https://claude.ai/other","client_name":"C","redirect_uris":["http://localhost/cb"]}`}
	h.withMetadata(f, false)
	ctx := context.Background()
	for _, u := range []string{"http://claude.ai/x", "https://othra.example.com/client.json", "https://claude.ai/"} {
		_, err := h.svc.Admit(ctx, h.org, h.project, client.AdmitRequest{URL: u})
		assert.Equal(t, apperror.CodeAuthorizeClientNotAdmitted, codeOf(err), u)
	}
	assert.Zero(t, f.calls)
	_, err := h.svc.Admit(ctx, h.org, h.project, client.AdmitRequest{URL: claudeURL})
	assert.Equal(t, apperror.CodeAuthorizeClientMetadataUnusable, codeOf(err))
	host, err := h.svc.MetadataHost(claudeURL)
	require.NoError(t, err)
	assert.Equal(t, "claude.ai", host)
}

// past the per-URL limit the fetch is refused as rate limited.
func TestAdmit_Throttled(t *testing.T) {
	h := newHarness(t)
	f := &fakeFetcher{err: errors.New("down")}
	h.withMetadata(f, true)
	var last error
	for range client.MetadataFetchURLRule().Burst + 1 {
		_, last = h.svc.Admit(context.Background(), h.org, h.project, client.AdmitRequest{URL: claudeURL})
	}
	assert.True(t, layout.IsRateLimitedError(last), "%v", last)
	assert.Equal(t, client.MetadataFetchURLRule().Burst, f.calls)

	_, err := h.svc.Admit(context.Background(), h.org, uuid.New(), client.AdmitRequest{URL: claudeURL})
	assert.False(t, layout.IsRateLimitedError(err), "another project keeps its own budget: %v", err)
	assert.Equal(t, client.MetadataFetchURLRule().Burst+1, f.calls)
}

// a global per-host ceiling stays as a backstop across projects.
func TestAdmit_HostCeiling(t *testing.T) {
	h := newHarness(t)
	f := &fakeFetcher{err: errors.New("down")}
	h.withMetadata(f, true)
	var last error
	for i := range client.MetadataFetchHostCeilingRule().Burst + 1 {
		_, last = h.svc.Admit(context.Background(), h.org, uuid.New(), client.AdmitRequest{URL: claudeURL + strconv.Itoa(i)})
	}
	assert.True(t, layout.IsRateLimitedError(last), "%v", last)
	assert.Equal(t, client.MetadataFetchHostCeilingRule().Burst, f.calls)
}

// CLT008: a shadow client is changed only by its document; deleting it is the revoke.
func TestShadowClient_ReadOnly(t *testing.T) {
	h := newHarness(t)
	h.withMetadata(&fakeFetcher{body: claudeDoc("Claude Code")}, false)
	ctx := context.Background()
	c, err := h.svc.Admit(ctx, h.org, h.project, client.AdmitRequest{URL: claudeURL, ResourceServerID: uuid.New()})
	require.NoError(t, err)
	_, err = h.svc.Update(ctx, h.org, h.project, c.PublicID, cli())
	assert.Equal(t, apperror.CodeClientMetadataManaged, codeOf(err))
	_, _, err = h.svc.RotateSecret(ctx, h.org, h.project, c.PublicID, 0)
	assert.Equal(t, apperror.CodeClientMetadataManaged, codeOf(err))
	_, err = h.svc.RevokeSecret(ctx, h.org, h.project, c.PublicID, "x")
	assert.Equal(t, apperror.CodeClientMetadataManaged, codeOf(err))
	require.NoError(t, h.svc.Delete(ctx, h.org, h.project, c.PublicID))
}

func TestAdmit_StoreAndLinkFailures(t *testing.T) {
	h := newHarness(t)
	links := h.withMetadata(&fakeFetcher{body: claudeDoc("Claude Code")}, false)
	h.store.FailOn, h.store.Err = "SaveShadow", errors.New("db down")
	_, err := h.svc.Admit(context.Background(), h.org, h.project, client.AdmitRequest{URL: claudeURL})
	assert.Equal(t, apperror.CodeUnexpected, codeOf(err))
	h.store.FailOn = "ByPublicID"
	_, err = h.svc.Admit(context.Background(), h.org, h.project, client.AdmitRequest{URL: claudeURL})
	assert.Equal(t, apperror.CodeUnexpected, codeOf(err))
	h.store.FailOn, links.err = "", errors.New("link down")
	_, err = h.svc.Admit(context.Background(), h.org, h.project, client.AdmitRequest{URL: claudeURL})
	assert.Equal(t, apperror.CodeUnexpected, codeOf(err))
}

type eachProject struct {
	org, project uuid.UUID
	err          error
}

func (e eachProject) Each(ctx context.Context, fn func(ctx context.Context, orgID, projectID uuid.UUID) error) error {
	if err := fn(ctx, e.org, e.project); err != nil {
		return err
	}
	return e.err
}

// a shadow unused for ShadowKeep past its document's expiry is swept by the hourly job.
func TestPruneShadows(t *testing.T) {
	h := newHarness(t)
	h.withMetadata(&fakeFetcher{body: claudeDoc("Claude Code"), cache: "max-age=3600"}, false)
	ctx := context.Background()
	_, err := h.svc.Admit(ctx, h.org, h.project, client.AdmitRequest{URL: claudeURL, ResourceServerID: uuid.New()})
	require.NoError(t, err)
	n, err := h.svc.PruneShadows(ctx, h.org, h.project)
	require.NoError(t, err)
	assert.Zero(t, n)

	h.now = h.now.Add(time.Hour + client.ShadowKeep + time.Minute)
	jobs := client.NewScheduler(h.svc, eachProject{org: h.org, project: h.project}, slog.New(slog.NewJSONHandler(h.logs, nil))).SchedulerJobs()
	require.Len(t, jobs, 1)
	require.NoError(t, jobs[0].Run(ctx))
	_, err = h.svc.Get(ctx, h.org, h.project, claudeURL)
	assert.True(t, client.IsNotFoundError(err))
	assert.Contains(t, h.logs.String(), `"deleted":1`)

	h.store.FailOn, h.store.Err = "PruneShadows", errors.New("db down")
	assert.Error(t, jobs[0].Run(ctx))
}
