package hosted

import (
	"context"
	"slices"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/altalune-id/othra/apperror"
	"github.com/altalune-id/othra/internal/consent"
)

const (
	mcpResource = "https://mcp1.test/mcp"
	otherRS     = "https://mcp2.test/mcp"
	cimdClient  = "https://claude.ai/oauth/claude-code-client-metadata"
)

func rowID(clientID string) uuid.UUID { return uuid.NewSHA1(uuid.Nil, []byte(clientID)) }

type fakeResources struct {
	servers map[string]Resource
	links   map[string]map[string][]string
}

func testResources() *fakeResources {
	return &fakeResources{
		servers: map[string]Resource{
			mcpResource: {
				ID: rowID(mcpResource), Identifier: mcpResource, Name: "Sample MCP", Scopes: []string{"sample:read", "sample:write"},
				Descriptions: map[string]string{"sample:read": "Read your samples"}, Catalog: []string{"sample:read", "sample:write"},
			},
			otherRS: {ID: rowID(otherRS), Identifier: otherRS, Name: "Other MCP", Scopes: []string{"other:read"}},
		},
		links: map[string]map[string][]string{
			mcpResource: {"cli": {"sample:read"}, "nat": {"sample:read", "sample:write"}, "req": {"sample:read"}, "web": {"sample:read"}},
		},
	}
}

func (f *fakeResources) Resource(_ context.Context, _ Project, c Client, identifier string) (Resource, bool, error) {
	r, ok := f.servers[identifier]
	pin, linked := f.links[identifier][c.ID]
	if !ok || !linked {
		return Resource{}, false, nil
	}
	r.Scopes = pin
	return r, true, nil
}

type fakeAdmissions struct {
	mu    sync.Mutex
	calls int
	fail  error
}

func (f *fakeAdmissions) Admit(_ context.Context, _ Project, clientURL string, resources []string) (Client, Resource, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	switch {
	case f.fail != nil:
		return Client{}, Resource{}, f.fail
	case clientURL != cimdClient || len(slices.Compact(slices.Clone(resources))) != 1 || resources[0] != mcpResource:
		return Client{}, Resource{}, apperror.New(apperror.CodeAuthorizeClientNotAdmitted, "not admitted")
	}
	return Client{
			ID: cimdClient, RowID: rowID(cimdClient), MetadataHost: "claude.ai", Name: "Claude Code", Native: true,
			RedirectURIs:  []string{"http://localhost/callback", "http://127.0.0.1/callback"},
			AllowedScopes: []string{"openid", "profile", "email", "offline_access"}, Refresh: true, PKCERequired: true, ConsentRequired: true,
		}, Resource{
			ID: rowID(mcpResource), Identifier: mcpResource, Name: "Sample MCP", Scopes: []string{"sample:read"},
		}, nil
}

type fakeConsents struct {
	mu     sync.Mutex
	rows   map[string]*consent.Consent
	grants []consent.GrantRequest
	now    func() time.Time
	failOn string
}

func newFakeConsents(now func() time.Time) *fakeConsents {
	return &fakeConsents{rows: map[string]*consent.Consent{}, now: now}
}

func consentRow(k consent.Key) string {
	rs := "-"
	if k.ResourceServerID != nil {
		rs = k.ResourceServerID.String()
	}
	return k.EndUserID.String() + "|" + k.ClientID.String() + "|" + rs
}

func (f *fakeConsents) Consent(_ context.Context, _ Project, key consent.Key) (*consent.Consent, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failOn == "consent" {
		return nil, errFake
	}
	if c, ok := f.rows[consentRow(key)]; ok {
		cp := *c
		return &cp, nil
	}
	return nil, nil
}

func (f *fakeConsents) Grant(_ context.Context, _ Project, req consent.GrantRequest) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failOn == "grant" {
		return errFake
	}
	f.grants = append(f.grants, req)
	c, ok := f.rows[consentRow(req.Key)]
	if !ok {
		c = &consent.Consent{Key: req.Key}
		f.rows[consentRow(req.Key)] = c
	}
	c.Scopes, c.UpdatedAt = consent.Union(c.Scopes, req.Scopes), f.now()
	return nil
}

func (f *fakeConsents) stored(key consent.Key) *consent.Consent {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.rows[consentRow(key)]
}
