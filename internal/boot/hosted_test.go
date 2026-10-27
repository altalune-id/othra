package boot

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPurposeKey_OnePerPurpose(t *testing.T) {
	secret := []byte("state-secret-state-secret-state-secret")
	csrf, cookie := purposeKey(secret, "othra-hosted-csrf-v1"), purposeKey(secret, "othra-hosted-cookie-v1")
	assert.Len(t, csrf, 32)
	assert.NotEqual(t, csrf, cookie)
	assert.Equal(t, csrf, purposeKey(secret, "othra-hosted-csrf-v1"), "stable across restarts")
	assert.NotEqual(t, csrf, purposeKey([]byte("another secret"), "othra-hosted-csrf-v1"))
}
