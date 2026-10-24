package resourceserver

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"altalune.id/othra/apperror"
)

func (f *fakeClients) addManaged(name, url string) ClientRef {
	at := time.Date(2026, 10, 23, 9, 0, 0, 0, time.UTC)
	c := ClientRef{ID: uuid.New(), PublicID: "c" + uuid.NewString()[:20], Name: name, MetadataURL: url, MetadataFetchedAt: &at}
	f.byPublic[c.PublicID] = c
	return c
}

func (h *harness) verified(t *testing.T, identifier string, scopes ...string) *ResourceServer {
	t.Helper()
	r := h.create(t, identifier, scopes...)
	got, f, err := h.svc.Verify(context.Background(), h.org, h.project, r.PublicID)
	require.NoError(t, err)
	require.Nil(t, f)
	return got
}

// E10, RSV010, RSV011, RSV009: admission is turned on only for an admissible server, with valid hosts and catalog scopes.
func TestService_SetAdmission(t *testing.T) {
	h := newHarness(t, false)
	ctx := context.Background()
	var changes []uuid.UUID
	h.svc.WithOnAdmissionChange(func(p uuid.UUID) { changes = append(changes, p) })
	pending := h.create(t, "https://pending.example.com/mcp")
	_, err := h.svc.SetAdmission(ctx, h.org, h.project, pending.PublicID, Admission{Accept: true})
	assert.Equal(t, apperror.CodeResourceServerAdmissionUnverified, codeOf(err))
	off, err := h.svc.SetAdmission(ctx, h.org, h.project, pending.PublicID, Admission{})
	require.NoError(t, err, "turning off is always allowed")
	assert.False(t, off.Admission.Accept)

	r := h.verified(t, "https://mcp.example.com/mcp", "read", "write")
	changes = nil
	_, err = h.svc.SetAdmission(ctx, h.org, h.project, r.PublicID, Admission{Accept: true, Hosts: []string{"Claude.ai"}})
	assert.Equal(t, apperror.CodeResourceServerInvalidClientHost, codeOf(err))
	_, err = h.svc.SetAdmission(ctx, h.org, h.project, r.PublicID, Admission{Accept: true, Scopes: []string{"admin"}})
	assert.True(t, IsInvalidLinkScopesError(err))
	_, err = h.svc.SetAdmission(ctx, h.org, h.project, r.PublicID, Admission{Accept: true, Scopes: []string{}})
	assert.True(t, IsInvalidLinkScopesError(err), "picking none is refused")

	cimd := h.clients.addManaged("Claude Code", "https://claude.ai/oauth/claude-code-client-metadata")
	require.NoError(t, h.svc.LinkMetadataClient(ctx, h.org, h.project, cimd.ID, r.ID, []string{"read", "write"}))
	got, err := h.svc.SetAdmission(ctx, h.org, h.project, r.PublicID,
		Admission{Accept: true, Hosts: []string{"claude.ai"}, Scopes: []string{"read", "read"}})
	require.NoError(t, err)
	assert.Equal(t, []string{"read"}, got.Admission.Scopes)
	assert.Equal(t, []uuid.UUID{h.project}, changes, "discovery is published again")
	links, err := h.svc.ListLinks(ctx, h.org, h.project, "", r.PublicID)
	require.NoError(t, err)
	require.Len(t, links, 1)
	assert.Equal(t, []string{"read"}, links[0].Scopes, "CIMD links follow the new pin")
	assert.Equal(t, cimd.MetadataURL, links[0].Client.MetadataURL)

	_, err = h.svc.Update(ctx, h.org, h.project, r.PublicID, "MCP", []string{"write"}, nil)
	pinned, ok := err.(*ScopePinnedError) //nolint:errorlint // returned unwrapped.
	require.True(t, ok, "%v", err)
	assert.True(t, pinned.CIMD, "RSV005 cimd_admission")

	ok, err = h.svc.AcceptsCIMD(ctx, h.org, h.project)
	require.NoError(t, err)
	assert.True(t, ok)
}

// R4, M2: a registered client needs a link to an admissible server; a metadata document client needs admission from its host.
func TestService_ResolveAndAdmits(t *testing.T) {
	h := newHarness(t, false)
	ctx := context.Background()
	r := h.verified(t, "https://mcp.example.com/mcp", "read", "write")
	web := h.clients.add("Web")
	_, _, ok, err := h.svc.Resolve(ctx, h.org, h.project, web.ID, r.Identifier)
	require.NoError(t, err)
	assert.False(t, ok, "not linked")
	_, err = h.svc.LinkClient(ctx, h.org, h.project, web.PublicID, r.PublicID, []string{"read"})
	require.NoError(t, err)
	got, pin, ok, err := h.svc.Resolve(ctx, h.org, h.project, web.ID, r.Identifier)
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, r.ID, got.ID)
	assert.Equal(t, []string{"read"}, pin)
	for _, id := range []string{r.Identifier + "/", "https://other.example.com/mcp"} {
		_, _, ok, err = h.svc.Resolve(ctx, h.org, h.project, web.ID, id)
		require.NoError(t, err)
		assert.False(t, ok, id)
	}

	aud, ok, err := h.svc.Admits(ctx, h.org, h.project, r.ID, web.ID, "", []string{"read"})
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, r.Identifier, aud)
	_, ok, err = h.svc.Admits(ctx, h.org, h.project, r.ID, web.ID, "", []string{"read", "write"})
	require.NoError(t, err)
	assert.False(t, ok, "write is outside the pin")
	_, ok, err = h.svc.Admits(ctx, h.org, h.project, r.ID, uuid.New(), "claude.ai", []string{"read"})
	require.NoError(t, err)
	assert.False(t, ok, "admission is off")

	_, err = h.svc.SetAdmission(ctx, h.org, h.project, r.PublicID, Admission{Accept: true, Hosts: []string{"claude.ai"}})
	require.NoError(t, err)
	_, ok, err = h.svc.ForMetadataClient(ctx, h.org, h.project, r.Identifier, "claude.ai")
	require.NoError(t, err)
	assert.True(t, ok)
	_, ok, err = h.svc.ForMetadataClient(ctx, h.org, h.project, r.Identifier, "evil.ai")
	require.NoError(t, err)
	assert.False(t, ok)
	_, ok, err = h.svc.ForMetadataClient(ctx, h.org, h.project, "https://nope.example.com/mcp", "claude.ai")
	require.NoError(t, err)
	assert.False(t, ok)
	_, ok, err = h.svc.Admits(ctx, h.org, h.project, r.ID, uuid.New(), "claude.ai", []string{"read", "write"})
	require.NoError(t, err)
	assert.True(t, ok, "all scopes of the catalog")
	_, ok, err = h.svc.Admits(ctx, h.org, h.project, uuid.New(), uuid.New(), "claude.ai", nil)
	require.NoError(t, err)
	assert.False(t, ok, "an unknown server")
}

// E10, CLT008: a metadata document client's links change only through admission; its links never block a delete.
func TestService_ManagedClientLinks(t *testing.T) {
	h := newHarness(t, false)
	ctx := context.Background()
	r := h.verified(t, "https://mcp.example.com/mcp")
	cimd := h.clients.addManaged("Claude Code", "https://claude.ai/oauth/claude-code-client-metadata")
	_, err := h.svc.LinkClient(ctx, h.org, h.project, cimd.PublicID, r.PublicID, []string{"read"})
	assert.Equal(t, apperror.CodeClientMetadataManaged, codeOf(err))
	assert.Equal(t, apperror.CodeClientMetadataManaged, codeOf(h.svc.UnlinkClient(ctx, h.org, h.project, cimd.PublicID, r.PublicID)))
	require.NoError(t, h.svc.LinkMetadataClient(ctx, h.org, h.project, cimd.ID, r.ID, []string{"read"}))
	require.NoError(t, h.svc.Delete(ctx, h.org, h.project, r.PublicID), "CIMD links cascade with the server")
}

// RSV004 description: a description names a scope of the server and has 1 to 120 characters.
func TestService_ScopeDescriptions(t *testing.T) {
	h := newHarness(t, false)
	ctx := context.Background()
	r, err := h.svc.Create(ctx, CreateRequest{
		OrgID: h.org, ProjectID: h.project, Name: "MCP", Identifier: "https://mcp.example.com/mcp", Scopes: []string{"read"},
		Descriptions: map[string]string{"read": "Read your messages"},
	})
	require.NoError(t, err)
	assert.Equal(t, "Read your messages", r.Descriptions["read"])
	for name, d := range map[string]map[string]string{
		"unknown scope": {"write": "x"},
		"blank":         {"read": "  "},
		"too long":      {"read": string(make([]rune, MaxDescriptionLength+1))},
	} {
		_, err := h.svc.Update(ctx, h.org, h.project, r.PublicID, "MCP", []string{"read"}, d)
		e, ok := err.(*InvalidScopeError) //nolint:errorlint // returned unwrapped.
		require.True(t, ok, "%s: %v", name, err)
		assert.Equal(t, ScopeDescription, e.Reason, name)
	}
}
