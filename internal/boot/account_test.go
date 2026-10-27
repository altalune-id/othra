package boot

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// connected apps split a grant's scopes as introspection and at+jwt do.
func TestConsentGrants_ResourceScopes(t *testing.T) {
	assert.Equal(t, []string{"read"}, consentGrants{}.ResourceScopes([]string{"openid", "read", "email", "offline_access", "profile"}))
}
