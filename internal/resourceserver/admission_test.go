package resourceserver

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// admission is off by default, needs an admissible server, and checks the client_id host.
func TestResourceServer_AdmitsMetadataClient(t *testing.T) {
	now := time.Now()
	verified := &ResourceServer{State: StateVerified, VerifiedAt: &now, Scopes: []string{"sample:read", "sample:write"}}
	assert.False(t, verified.AdmitsMetadataClient("claude.ai", false), "off by default")

	verified.Admission = Admission{Accept: true}
	assert.True(t, verified.AdmitsMetadataClient("claude.ai", false), "no host list means any host")

	verified.Admission.Hosts = []string{"claude.ai", "cimd.test"}
	assert.True(t, verified.AdmitsMetadataClient("claude.ai", false))
	assert.True(t, verified.AdmitsMetadataClient("Claude.AI", false), "hosts compare case-insensitively")
	assert.False(t, verified.AdmitsMetadataClient("evil.ai", false))
	assert.False(t, verified.AdmitsMetadataClient("sub.claude.ai", false), "never a suffix match")

	pending := &ResourceServer{State: StatePending, Admission: Admission{Accept: true}}
	assert.False(t, pending.AdmitsMetadataClient("claude.ai", true), "only an admissible server")
	trusted := &ResourceServer{State: StatePending, Trusted: true, Admission: Admission{Accept: true}}
	assert.True(t, trusted.AdmitsMetadataClient("claude.ai", true))
	assert.False(t, trusted.AdmitsMetadataClient("claude.ai", false))
}

// CIMD clients get the pinned scopes, or the whole catalog.
func TestResourceServer_MetadataClientScopes(t *testing.T) {
	r := &ResourceServer{Scopes: []string{"sample:read", "sample:write"}}
	assert.Equal(t, []string{"sample:read", "sample:write"}, r.MetadataClientScopes())
	r.Admission.Scopes = []string{"sample:read"}
	got := r.MetadataClientScopes()
	assert.Equal(t, []string{"sample:read"}, got)
	got[0] = "changed"
	assert.Equal(t, "sample:read", r.Admission.Scopes[0], "a copy")
}

// RSV010: allowed hosts are lower-case DNS names, no IP literals, no repeats, at most 64.
func TestValidateClientHosts(t *testing.T) {
	require.NoError(t, ValidateClientHosts(nil))
	require.NoError(t, ValidateClientHosts([]string{"claude.ai", "cimd.test", "a-b.example.co.id"}))
	for name, tc := range map[string]struct {
		hosts  []string
		reason HostReason
	}{
		"upper case":   {[]string{"Claude.ai"}, HostFormat},
		"scheme":       {[]string{"https://claude.ai"}, HostFormat},
		"path":         {[]string{"claude.ai/x"}, HostFormat},
		"port":         {[]string{"claude.ai:443"}, HostFormat},
		"single label": {[]string{"localhost"}, HostFormat},
		"empty":        {[]string{""}, HostFormat},
		"wildcard":     {[]string{"*.claude.ai"}, HostFormat},
		"bad label":    {[]string{"-claude.ai"}, HostFormat},
		"trailing dot": {[]string{"claude.ai."}, HostFormat},
		"too long":     {[]string{strings.Repeat("a", 64) + ".ai"}, HostFormat},
		"ipv4":         {[]string{"203.0.113.9"}, HostIPLiteral},
		"ipv6":         {[]string{"2001:db8::1"}, HostIPLiteral},
		"duplicate":    {[]string{"claude.ai", "claude.ai"}, HostDuplicate},
		"too many": {func() []string {
			out := make([]string, MaxClientHosts+1)
			for i := range out {
				out[i] = "h" + strings.Repeat("x", i%10) + string(rune('a'+i%26)) + ".test"
			}
			return out
		}(), HostTooMany},
	} {
		err := ValidateClientHosts(tc.hosts)
		he, ok := err.(*InvalidClientHostError) //nolint:errorlint // returned unwrapped.
		require.True(t, ok, "%s: %v", name, err)
		assert.Equal(t, tc.reason, he.Reason, name)
	}
}
