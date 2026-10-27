package resourceserver

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
	"altalune.id/othra/internal/kernel/authz"
)

func TestCreate_LocksTheIssuerHandle(t *testing.T) {
	h := newHarness(t, false)
	r := h.create(t, "https://api.example.com/mcp")
	assert.Len(t, r.PublicID, publicIDLen)
	assert.Equal(t, StatePending, r.State)
	assert.False(t, r.Trusted)
	assert.Equal(t, []uuid.UUID{h.project}, h.issuers.locked)
	assert.Equal(t, "API https://api.example.com/mcp", h.store.server(r.ID).Name)

	_, err := h.svc.Create(context.Background(), CreateRequest{
		OrgID: h.org, ProjectID: h.project, Name: "Again", Identifier: "https://api.example.com/mcp", Scopes: []string{"read"},
	})
	assert.True(t, IsIdentifierTakenError(err), "%v", err)
}

func TestCreate_RefusesBadInput(t *testing.T) {
	h := newHarness(t, false)
	ctx := context.Background()
	_, err := h.svc.Create(ctx, CreateRequest{OrgID: h.org, ProjectID: h.project, Name: "x", Identifier: "https://API.example.com", Scopes: []string{"read"}})
	assert.True(t, IsInvalidIdentifierError(err), "%v", err)
	_, err = h.svc.Create(ctx, CreateRequest{OrgID: h.org, ProjectID: h.project, Name: "x", Identifier: "https://api.example.com", Scopes: []string{"openid"}})
	assert.True(t, IsInvalidScopeError(err), "%v", err)
	assert.Empty(t, h.issuers.locked, "nothing locked the handle")
}

func TestCreate_TrimsTheName(t *testing.T) {
	h := newHarness(t, false)
	r, err := h.svc.Create(context.Background(), CreateRequest{
		OrgID: h.org, ProjectID: h.project, Name: "  Billing  ", Identifier: "https://api.example.com", Scopes: []string{"read"},
	})
	require.NoError(t, err)
	assert.Equal(t, "Billing", r.Name)
}

// trust on create follows SetTrust (self-hosted, a person), and an http loopback identifier exists only trusted.
func TestCreate_Trusted(t *testing.T) {
	loop := func(h *harness, trusted bool, person uuid.UUID) error {
		_, err := h.svc.Create(context.Background(), CreateRequest{
			OrgID: h.org, ProjectID: h.project, Name: "x", Identifier: "http://127.0.0.1:8080/mcp", Scopes: []string{"read"},
			Trusted: trusted, Person: person,
		})
		return err
	}
	cloud := newHarness(t, false)
	assert.True(t, IsTrustSelfHostedOnlyError(loop(cloud, true, cloud.person)), "cloud never trusts")
	err := loop(cloud, false, cloud.person)
	require.True(t, IsInvalidIdentifierError(err), "%v", err)
	assert.Equal(t, "needs_trust", appMeta(err)["reason"])

	self := newHarness(t, true)
	err = loop(self, false, self.person)
	require.True(t, IsInvalidIdentifierError(err), "%v", err)
	assert.Equal(t, "needs_trust", appMeta(err)["reason"])
	assert.True(t, authz.IsForbiddenError(loop(self, true, uuid.Nil)), "a key can't trust")
	assert.Empty(t, self.store.servers)

	var buf bytes.Buffer
	self.svc.WithLogger(slog.New(slog.NewJSONHandler(&buf, nil)))
	r := self.create(t, "http://127.0.0.1:8080/mcp")
	assert.True(t, r.Trusted)
	assert.Equal(t, self.person, r.TrustedBy)
	assert.Contains(t, buf.String(), `"msg":"resourceserver: trust changed"`)
	assert.Contains(t, buf.String(), `"trusted":true`)
}

func appMeta(err error) map[string]string {
	if app, ok := apperror.AsAppError(err); ok {
		return app.Meta()
	}
	return nil
}

// SECURITY: a failing insert leaves nothing behind and is reported, not echoed.
func TestCreate_StoreFailures(t *testing.T) {
	h := newHarness(t, false)
	h.store.failOn = "insert"
	_, err := h.svc.Create(context.Background(), CreateRequest{OrgID: h.org, ProjectID: h.project, Name: "x", Identifier: "https://a.example", Scopes: []string{"r"}})
	assert.Equal(t, apperror.CodeUnexpected, codeOf(err))
	h.store.failOn = ""
	h.issuers.failOn = "lock"
	_, err = h.svc.Create(context.Background(), CreateRequest{OrgID: h.org, ProjectID: h.project, Name: "x", Identifier: "https://a.example", Scopes: []string{"r"}})
	assert.Equal(t, apperror.CodeUnexpected, codeOf(err))
	assert.Empty(t, h.store.servers)
}

func TestListAndGet(t *testing.T) {
	h := newHarness(t, false)
	a := h.create(t, "https://a.example")
	h.create(t, "https://b.example")
	list, err := h.svc.List(context.Background(), h.org, h.project)
	require.NoError(t, err)
	assert.Len(t, list, 2)
	got, err := h.svc.Get(context.Background(), h.org, h.project, a.PublicID)
	require.NoError(t, err)
	assert.Equal(t, a.Identifier, got.Identifier)

	_, err = h.svc.Get(context.Background(), h.org, uuid.New(), a.PublicID)
	assert.True(t, IsNotFoundError(err), "another project's server is not found")
	h.store.failOn = "list"
	_, err = h.svc.List(context.Background(), h.org, h.project)
	assert.Equal(t, apperror.CodeUnexpected, codeOf(err))
}

func TestUpdate_RefusesRemovingAPinnedScope(t *testing.T) {
	h := newHarness(t, false)
	r := h.create(t, "https://api.example.com", "read", "write", "admin")
	web, cli := h.clients.add("Web"), h.clients.add("CLI")
	ctx := context.Background()
	_, err := h.svc.LinkClient(ctx, h.org, h.project, web.PublicID, r.PublicID, []string{"read", "write"})
	require.NoError(t, err)
	_, err = h.svc.LinkClient(ctx, h.org, h.project, cli.PublicID, r.PublicID, []string{"write"})
	require.NoError(t, err)

	_, err = h.svc.Update(ctx, h.org, h.project, r.PublicID, "API", []string{"read"}, nil)
	require.True(t, IsScopePinnedError(err), "%v", err)
	pinned := asError[*ScopePinnedError](t, err)
	assert.Equal(t, "write", pinned.Scope)
	assert.ElementsMatch(t, []string{"Web", "CLI"}, pinned.Clients)

	updated, err := h.svc.Update(ctx, h.org, h.project, r.PublicID, " Renamed ", []string{"read", "write", "extra"}, nil)
	require.NoError(t, err, "removing admin, which nothing pins, is fine")
	assert.Equal(t, "Renamed", updated.Name)
	assert.Equal(t, []string{"read", "write", "extra"}, h.store.server(r.ID).Scopes)

	_, err = h.svc.Update(ctx, h.org, h.project, r.PublicID, "API", []string{"openid"}, nil)
	assert.True(t, IsInvalidScopeError(err))
	_, err = h.svc.Update(ctx, h.org, h.project, "nope", "API", []string{"read"}, nil)
	assert.True(t, IsNotFoundError(err))
}

func TestDelete_RefusedWhileLinked(t *testing.T) {
	h := newHarness(t, false)
	r := h.create(t, "https://api.example.com")
	web := h.clients.add("Web")
	ctx := context.Background()
	_, err := h.svc.LinkClient(ctx, h.org, h.project, web.PublicID, r.PublicID, []string{"read"})
	require.NoError(t, err)

	err = h.svc.Delete(ctx, h.org, h.project, r.PublicID)
	require.True(t, IsLinkedError(err), "%v", err)
	assert.Equal(t, []string{"Web"}, asError[*LinkedError](t, err).Clients)

	require.NoError(t, h.svc.UnlinkClient(ctx, h.org, h.project, web.PublicID, r.PublicID))
	require.NoError(t, h.svc.UnlinkClient(ctx, h.org, h.project, web.PublicID, r.PublicID), "a missing link is no error")
	require.NoError(t, h.svc.Delete(ctx, h.org, h.project, r.PublicID))
	assert.Empty(t, h.store.servers)
	assert.True(t, IsNotFoundError(h.svc.Delete(ctx, h.org, h.project, r.PublicID)))
}

func TestVerify_PassPromotes(t *testing.T) {
	h := newHarness(t, false)
	r := h.create(t, "https://api.example.com/mcp")
	var buf bytes.Buffer
	h.svc.WithLogger(slog.New(slog.NewJSONHandler(&buf, nil)))
	got, f, err := h.svc.Verify(context.Background(), h.org, h.project, r.PublicID)
	require.NoError(t, err)
	assert.Nil(t, f)
	assert.Equal(t, StateVerified, got.State)
	assert.Equal(t, h.now, *got.VerifiedAt)
	assert.Equal(t, []string{"https://api.example.com/mcp " + testIssuer}, h.checker.calls)
	assert.Equal(t, StateVerified, h.store.server(r.ID).State)
	line := buf.String()
	assert.Equal(t, 1, strings.Count(line, "\n"), line)
	assert.Contains(t, line, `"from":"pending","to":"verified"`)
	assert.Contains(t, line, `"check":"manual"`)
	assert.Contains(t, line, `"project":"`+testProjectPublicID+`"`)
	assert.NotContains(t, line, h.project.String(), "no UUID in the log")
}

// a failed "Verify now" shows its error but changes neither the count nor the state.
func TestVerify_AFailedManualCheckChangesNothing(t *testing.T) {
	h := newHarness(t, false)
	r := h.create(t, "https://api.example.com")
	_, _, err := h.svc.Verify(context.Background(), h.org, h.project, r.PublicID)
	require.NoError(t, err)
	before := h.store.server(r.ID)

	h.now = h.now.Add(time.Minute)
	h.checker.set(&Failure{Reason: FailureNotFound, Status: 404, CheckedAt: h.now})
	got, f, err := h.svc.Verify(context.Background(), h.org, h.project, r.PublicID)
	require.NoError(t, err)
	require.NotNil(t, f)
	assert.Equal(t, FailureNotFound, f.Reason)
	assert.Equal(t, StateVerified, got.State)
	assert.Equal(t, before, h.store.server(r.ID), "nothing was saved")
}

func TestVerify_IsRateLimited(t *testing.T) {
	h := newHarness(t, false)
	r := h.create(t, "https://api.example.com")
	h.checker.set(&Failure{Reason: FailureTimeout, CheckedAt: h.now})
	_, _, err := h.svc.Verify(context.Background(), h.org, h.project, r.PublicID)
	require.NoError(t, err)
	_, _, err = h.svc.Verify(context.Background(), h.org, h.project, r.PublicID)
	require.True(t, IsVerifyTooSoonError(err), "%v", err)
	assert.Equal(t, h.now.Add(VerifyCooldown), asError[*VerifyTooSoonError](t, err).RetryAt)
	assert.Len(t, h.checker.calls, 1)

	other := h.create(t, "https://other.example")
	_, _, err = h.svc.Verify(context.Background(), h.org, h.project, other.PublicID)
	require.NoError(t, err, "the limit is per server")

	h.now = h.now.Add(VerifyCooldown)
	_, _, err = h.svc.Verify(context.Background(), h.org, h.project, r.PublicID)
	require.NoError(t, err)
	assert.Len(t, h.svc.manual, 1, "expired entries are dropped")
}

func TestVerify_Refusals(t *testing.T) {
	h := newHarness(t, true)
	loop := h.create(t, "http://localhost:8080")
	_, _, err := h.svc.Verify(context.Background(), h.org, h.project, loop.PublicID)
	assert.True(t, IsInvalidIdentifierError(err), "an http identifier can't be verified")
	_, _, err = h.svc.Verify(context.Background(), h.org, h.project, "nope")
	assert.True(t, IsNotFoundError(err))

	r := h.create(t, "https://api.example.com")
	h.issuers.failOn = "issuer"
	_, _, err = h.svc.Verify(context.Background(), h.org, h.project, r.PublicID)
	assert.Equal(t, apperror.CodeUnexpected, codeOf(err))

	h.issuers.failOn = ""
	h.now = h.now.Add(VerifyCooldown)
	h.store.failOn = "update"
	_, _, err = h.svc.Verify(context.Background(), h.org, h.project, r.PublicID)
	assert.Equal(t, apperror.CodeUnexpected, codeOf(err))
}

// trust is self-hosted only, logged, and turning it off returns the server to pending.
func TestSetTrust(t *testing.T) {
	cloud := newHarness(t, false)
	r := cloud.create(t, "https://api.example.com")
	_, err := cloud.svc.SetTrust(context.Background(), cloud.org, cloud.project, r.PublicID, true, cloud.person)
	assert.True(t, IsTrustSelfHostedOnlyError(err), "%v", err)
	assert.False(t, cloud.store.server(r.ID).Trusted)

	h := newHarness(t, true)
	r = h.create(t, "https://api.example.com")
	var buf bytes.Buffer
	h.svc.WithLogger(slog.New(slog.NewJSONHandler(&buf, nil)))
	got, err := h.svc.SetTrust(context.Background(), h.org, h.project, r.PublicID, true, h.person)
	require.NoError(t, err)
	assert.True(t, got.Trusted)
	assert.Equal(t, h.now, *got.TrustedAt)
	assert.Contains(t, buf.String(), `"trusted":true`)
	assert.Contains(t, buf.String(), `"by":"usr`+h.person.String()[:18]+`"`)
	assert.Contains(t, buf.String(), `"project":"`+testProjectPublicID+`"`)
	assert.NotContains(t, buf.String(), h.person.String(), "no UUID in the log")
	assert.NotContains(t, buf.String(), h.project.String())

	buf.Reset()
	_, err = h.svc.SetTrust(context.Background(), h.org, h.project, r.PublicID, true, h.person)
	require.NoError(t, err)
	assert.Empty(t, buf.String(), "no change, no line")

	got, err = h.svc.SetTrust(context.Background(), h.org, h.project, r.PublicID, false, h.person)
	require.NoError(t, err)
	assert.False(t, got.Trusted)
	assert.Equal(t, StatePending, got.State)

	loop := h.create(t, "http://[::1]:9000")
	_, err = h.svc.SetTrust(context.Background(), h.org, h.project, loop.PublicID, false, h.person)
	assert.True(t, IsInvalidIdentifierError(err), "an http identifier stays trusted")
	_, err = h.svc.SetTrust(context.Background(), h.org, h.project, "nope", true, h.person)
	assert.True(t, IsNotFoundError(err))
}

// three scheduled failures turn verified into failed, and a pass resets the count.
func TestRecheck_ThreeScheduledFailuresFail(t *testing.T) {
	h := newHarness(t, false)
	r := h.create(t, "https://api.example.com")
	ctx := context.Background()
	n, err := h.svc.Recheck(ctx, h.org, h.project)
	require.NoError(t, err)
	assert.Equal(t, 1, n, "a never-checked server is checked at once")
	assert.Equal(t, StateVerified, h.store.server(r.ID).State)

	n, err = h.svc.Recheck(ctx, h.org, h.project)
	require.NoError(t, err)
	assert.Zero(t, n)
	assert.Len(t, h.checker.calls, 1, "a fresh check is not repeated")

	var buf bytes.Buffer
	h.svc.WithLogger(slog.New(slog.NewJSONHandler(&buf, nil)))
	h.checker.set(&Failure{Reason: FailureStatus, Status: 502})
	for i := int32(1); i <= FailureLimit; i++ {
		h.now = h.now.Add(25 * time.Hour)
		h.checker.failure.CheckedAt = h.now
		_, err = h.svc.Recheck(ctx, h.org, h.project)
		require.NoError(t, err)
		got := h.store.server(r.ID)
		assert.Equal(t, i, got.ConsecutiveFailures)
		if i < FailureLimit {
			assert.Equal(t, StateVerified, got.State, "after %d failures", i)
		}
	}
	got := h.store.server(r.ID)
	assert.Equal(t, StateFailed, got.State)
	assert.Equal(t, FailureStatus, got.LastError.Reason)
	assert.Contains(t, buf.String(), `"from":"verified","to":"failed"`)
	assert.Contains(t, buf.String(), `"reason":"status"`)

	h.checker.set(nil)
	h.now = h.now.Add(25 * time.Hour)
	n, err = h.svc.Recheck(ctx, h.org, h.project)
	require.NoError(t, err)
	assert.Equal(t, 1, n)
	got = h.store.server(r.ID)
	assert.Equal(t, StateVerified, got.State)
	assert.Zero(t, got.ConsecutiveFailures)
	assert.Nil(t, got.LastError)
}

func TestRecheck_SkipsTrustedAndReportsFailures(t *testing.T) {
	h := newHarness(t, true)
	r := h.create(t, "https://api.example.com")
	_, err := h.svc.SetTrust(context.Background(), h.org, h.project, r.PublicID, true, h.person)
	require.NoError(t, err)
	h.create(t, "http://localhost:1")
	n, err := h.svc.Recheck(context.Background(), h.org, h.project)
	require.NoError(t, err)
	assert.Zero(t, n)
	assert.Empty(t, h.checker.calls, "trusted servers are never re-checked")

	h.create(t, "https://b.example")
	h.issuers.failOn = "issuer"
	_, err = h.svc.Recheck(context.Background(), h.org, h.project)
	assert.Equal(t, apperror.CodeUnexpected, codeOf(err))
	h.issuers.failOn = ""
	h.store.failOn = "update"
	_, err = h.svc.Recheck(context.Background(), h.org, h.project)
	assert.Equal(t, apperror.CodeUnexpected, codeOf(err))
	h.store.failOn = "due"
	_, err = h.svc.Recheck(context.Background(), h.org, h.project)
	assert.Equal(t, apperror.CodeUnexpected, codeOf(err))

	h.store.failOn = ""
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = h.svc.Recheck(ctx, h.org, h.project)
	assert.ErrorIs(t, err, context.Canceled)
}

func TestLinks(t *testing.T) {
	h := newHarness(t, false)
	ctx := context.Background()
	a := h.create(t, "https://a.example", "read", "write")
	b := h.create(t, "https://b.example", "sync")
	web := h.clients.add("Web")

	l, err := h.svc.LinkClient(ctx, h.org, h.project, web.PublicID, a.PublicID, []string{"read", "read"})
	require.NoError(t, err)
	assert.Equal(t, []string{"read"}, l.Scopes)
	assert.Equal(t, "Web", l.Client.Name)
	assert.Equal(t, a.PublicID, l.ResourceServer.PublicID)

	l, err = h.svc.LinkClient(ctx, h.org, h.project, web.PublicID, a.PublicID, []string{"write"})
	require.NoError(t, err, "linking again replaces the scopes")
	assert.Equal(t, []string{"write"}, l.Scopes)
	_, err = h.svc.LinkClient(ctx, h.org, h.project, web.PublicID, b.PublicID, []string{"sync"})
	require.NoError(t, err)

	l, err = h.svc.UpdateLink(ctx, h.org, h.project, web.PublicID, a.PublicID, []string{"read", "write"})
	require.NoError(t, err)
	assert.Equal(t, []string{"read", "write"}, l.Scopes)

	byClient, err := h.svc.ListLinks(ctx, h.org, h.project, web.PublicID, "")
	require.NoError(t, err)
	require.Len(t, byClient, 2)
	assert.Equal(t, "Web", byClient[0].Client.Name)
	byServer, err := h.svc.ListLinks(ctx, h.org, h.project, "", b.PublicID)
	require.NoError(t, err)
	require.Len(t, byServer, 1)
	assert.Equal(t, web.PublicID, byServer[0].Client.PublicID)
}

func TestLinks_Refusals(t *testing.T) {
	h := newHarness(t, false)
	ctx := context.Background()
	a := h.create(t, "https://a.example", "read")
	web := h.clients.add("Web")

	_, err := h.svc.LinkClient(ctx, h.org, h.project, "nope", a.PublicID, []string{"read"})
	assert.True(t, IsClientNotFoundError(err), "%v", err)
	_, err = h.svc.LinkClient(ctx, h.org, h.project, web.PublicID, "nope", []string{"read"})
	assert.True(t, IsNotFoundError(err), "%v", err)
	_, err = h.svc.LinkClient(ctx, h.org, h.project, web.PublicID, a.PublicID, []string{"admin"})
	assert.True(t, IsInvalidLinkScopesError(err), "%v", err)
	_, err = h.svc.LinkClient(ctx, h.org, h.project, web.PublicID, a.PublicID, nil)
	assert.True(t, IsInvalidLinkScopesError(err), "%v", err)
	_, err = h.svc.UpdateLink(ctx, h.org, h.project, web.PublicID, a.PublicID, []string{"read"})
	assert.True(t, IsNotFoundError(err), "updating a missing link: %v", err)
	_, err = h.svc.ListLinks(ctx, h.org, h.project, "nope", "")
	assert.True(t, IsClientNotFoundError(err))
	_, err = h.svc.ListLinks(ctx, h.org, h.project, "", "nope")
	assert.True(t, IsNotFoundError(err))
	assert.True(t, IsClientNotFoundError(h.svc.UnlinkClient(ctx, h.org, h.project, "nope", a.PublicID)))
	assert.True(t, IsNotFoundError(h.svc.UnlinkClient(ctx, h.org, h.project, web.PublicID, "nope")))

	h.clients.failOn = "lookup"
	_, err = h.svc.LinkClient(ctx, h.org, h.project, web.PublicID, a.PublicID, []string{"read"})
	assert.Equal(t, apperror.CodeUnexpected, codeOf(err))
	h.clients.failOn = ""
	_, err = h.svc.LinkClient(ctx, h.org, h.project, web.PublicID, a.PublicID, []string{"read"})
	require.NoError(t, err)
	h.clients.failOn = "refs"
	_, err = h.svc.ListLinks(ctx, h.org, h.project, "", a.PublicID)
	assert.Equal(t, apperror.CodeUnexpected, codeOf(err))
	h.clients.failOn = ""
	h.store.failOn = "links"
	_, err = h.svc.ListLinks(ctx, h.org, h.project, "", a.PublicID)
	assert.Equal(t, apperror.CodeUnexpected, codeOf(err))
	h.store.failOn = "deletelink"
	assert.Equal(t, apperror.CodeUnexpected, codeOf(h.svc.UnlinkClient(ctx, h.org, h.project, web.PublicID, a.PublicID)))
	h.store.failOn = "upsert"
	_, err = h.svc.LinkClient(ctx, h.org, h.project, web.PublicID, a.PublicID, []string{"read"})
	assert.Equal(t, apperror.CodeUnexpected, codeOf(err))
}
