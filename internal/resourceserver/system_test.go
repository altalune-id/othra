package resourceserver

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const systemMCP = "http://localhost:5150/mcp"

func systemResource(hosts ...string) SystemResource {
	return SystemResource{Name: "Othra MCP", Identifier: systemMCP, Scopes: []string{"org:read", "member:read"}, ClientHosts: hosts}
}

func TestReconcileSystem_CreatesAnAdmissibleServer(t *testing.T) {
	h := newHarness(t, false)
	h.issuers.system = true
	changed, err := h.svc.ReconcileSystem(context.Background(), h.org, h.project, systemResource("claude.ai"))
	require.NoError(t, err)
	assert.True(t, changed)

	list, err := h.svc.List(context.Background(), h.org, h.project)
	require.NoError(t, err)
	require.Len(t, list, 1)
	r := list[0]
	assert.Equal(t, systemMCP, r.Identifier)
	assert.Equal(t, []string{"org:read", "member:read"}, r.Scopes)
	assert.True(t, r.Trusted, "an http loopback identifier exists only trusted")
	assert.Equal(t, StateVerified, r.State)
	assert.True(t, r.Admissible(false), "Othra serves its own document, so no check, even where trust is off")
	assert.True(t, r.AdmitsMetadataClient("claude.ai", false))
	assert.False(t, r.AdmitsMetadataClient("evil.example", false))
	assert.Empty(t, h.checker.calls, "reconciling never fetches anything")
}

func TestReconcileSystem_NoHostsMeansMetadataClientsStayOut(t *testing.T) {
	h := newHarness(t, false)
	h.issuers.system = true
	_, err := h.svc.ReconcileSystem(context.Background(), h.org, h.project, systemResource())
	require.NoError(t, err)
	list, err := h.svc.List(context.Background(), h.org, h.project)
	require.NoError(t, err)
	assert.False(t, list[0].Admission.Accept)
	accepts, err := h.svc.AcceptsCIMD(context.Background(), h.org, h.project)
	require.NoError(t, err)
	assert.False(t, accepts)
}

func TestReconcileSystem_FollowsConfigAndIsIdempotent(t *testing.T) {
	h := newHarness(t, false)
	h.issuers.system = true
	var admissionChanges int
	h.svc.WithOnAdmissionChange(func(uuid.UUID) { admissionChanges++ })
	ctx := context.Background()
	_, err := h.svc.ReconcileSystem(ctx, h.org, h.project, systemResource("claude.ai"))
	require.NoError(t, err)
	assert.Equal(t, 1, admissionChanges)

	changed, err := h.svc.ReconcileSystem(ctx, h.org, h.project, systemResource("claude.ai"))
	require.NoError(t, err)
	assert.False(t, changed)
	assert.Equal(t, 1, admissionChanges)

	want := systemResource()
	want.Identifier, want.Scopes = "https://othra.example.com/mcp", []string{"org:read"}
	changed, err = h.svc.ReconcileSystem(ctx, h.org, h.project, want)
	require.NoError(t, err)
	assert.True(t, changed)
	assert.Equal(t, 2, admissionChanges)

	list, err := h.svc.List(ctx, h.org, h.project)
	require.NoError(t, err)
	require.Len(t, list, 1, "still one server")
	assert.Equal(t, "https://othra.example.com/mcp", list[0].Identifier)
	assert.Equal(t, []string{"org:read"}, list[0].Scopes)
	assert.False(t, list[0].Admission.Accept)
}

func TestReconcileSystem_RefusesBadHosts(t *testing.T) {
	h := newHarness(t, false)
	h.issuers.system = true
	_, err := h.svc.ReconcileSystem(context.Background(), h.org, h.project, systemResource("127.0.0.1"))
	assert.True(t, IsInvalidClientHostError(err), "%v", err)
}

func TestReconcileSystem_RepinsLinkedMetadataClients(t *testing.T) {
	h := newHarness(t, false)
	h.issuers.system = true
	ctx := context.Background()
	_, err := h.svc.ReconcileSystem(ctx, h.org, h.project, systemResource("claude.ai"))
	require.NoError(t, err)
	list, err := h.svc.List(ctx, h.org, h.project)
	require.NoError(t, err)
	c := h.clients.addManaged("Claude Code", "https://claude.ai/oauth/claude-code-client-metadata")
	require.NoError(t, h.svc.LinkMetadataClient(ctx, h.org, h.project, c.ID, list[0].ID, []string{"org:read", "member:read"}))

	want := systemResource("claude.ai")
	want.Scopes = []string{"org:read"}
	_, err = h.svc.ReconcileSystem(ctx, h.org, h.project, want)
	require.NoError(t, err)
	_, pin, ok, err := h.svc.Resolve(ctx, h.org, h.project, c.ID, systemMCP)
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, []string{"org:read"}, pin)
}

// SECURITY: Othra owns this row; a failed self-fetch behind a proxy must never lock MCP clients out.
func TestRecheck_SkipsTheSystemServer(t *testing.T) {
	h := newHarness(t, false)
	h.issuers.system = true
	ctx := context.Background()
	_, err := h.svc.ReconcileSystem(ctx, h.org, h.project, systemResource())
	require.NoError(t, err)
	h.now = h.now.Add(365 * 24 * time.Hour)
	n, err := h.svc.Recheck(ctx, h.org, h.project)
	require.NoError(t, err)
	assert.Zero(t, n)
	assert.Empty(t, h.checker.calls)
}

// SECURITY: only boot writes the console project's server: no RPC reaches it, reads still do.
func TestRefuseSystem(t *testing.T) {
	h := newHarness(t, false)
	assert.NoError(t, h.svc.RefuseSystem(context.Background(), h.org, h.project))
	h.issuers.system = true
	assert.True(t, IsSystemProjectError(h.svc.RefuseSystem(context.Background(), h.org, h.project)))
}

// SECURITY: this server is trusted without a check, so a bad mcp.audience must never become one.
func TestReconcileSystem_ValidatesTheIdentifier(t *testing.T) {
	for _, bad := range []string{"http://othra.example.com/mcp", "https://othra.example.com/mcp?x=1", "https://othra.example.com/mcp#f", "urn:othra:mcp"} {
		h := newHarness(t, false)
		h.issuers.system = true
		want := systemResource()
		want.Identifier = bad
		changed, err := h.svc.ReconcileSystem(context.Background(), h.org, h.project, want)
		assert.True(t, IsInvalidIdentifierError(err), "%s: %v", bad, err)
		assert.False(t, changed, bad)
		list, err := h.svc.List(context.Background(), h.org, h.project)
		require.NoError(t, err)
		assert.Empty(t, list, bad)
	}
	for _, good := range []string{"http://localhost:5150/mcp", "http://127.0.0.1:5150/mcp", "https://othra.example.com/mcp"} {
		h := newHarness(t, false)
		h.issuers.system = true
		want := systemResource()
		want.Identifier = good
		_, err := h.svc.ReconcileSystem(context.Background(), h.org, h.project, want)
		assert.NoError(t, err, good)
	}
}

func TestCloseSystem_StopsAdmissionAndIsIdempotent(t *testing.T) {
	h := newHarness(t, false)
	h.issuers.system = true
	ctx := context.Background()
	var admissionChanges int
	h.svc.WithOnAdmissionChange(func(uuid.UUID) { admissionChanges++ })

	changed, err := h.svc.CloseSystem(ctx, h.org, h.project)
	require.NoError(t, err)
	assert.False(t, changed, "no server yet")

	_, err = h.svc.ReconcileSystem(ctx, h.org, h.project, systemResource("claude.ai"))
	require.NoError(t, err)
	admissionChanges = 0
	changed, err = h.svc.CloseSystem(ctx, h.org, h.project)
	require.NoError(t, err)
	assert.True(t, changed)
	assert.Equal(t, 1, admissionChanges, "discovery drops its cache")
	accepts, err := h.svc.AcceptsCIMD(ctx, h.org, h.project)
	require.NoError(t, err)
	assert.False(t, accepts)

	changed, err = h.svc.CloseSystem(ctx, h.org, h.project)
	require.NoError(t, err)
	assert.False(t, changed)
	assert.Equal(t, 1, admissionChanges)
}
