package keys

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"altalune.id/othra/apperror"
)

func TestErrors_CarryTheirCodes(t *testing.T) {
	noActive := &NoActiveKeyError{ProjectID: "p"}
	pending := &RotationPendingError{ProjectID: "p"}
	assert.Equal(t, apperror.CodeSigningKeyNoActive, noActive.ToAppError().Code())
	assert.Equal(t, apperror.CodeSigningKeyRotationPending, pending.ToAppError().Code())
	assert.True(t, IsNoActiveKeyError(noActive))
	assert.True(t, IsRotationPendingError(pending))
	assert.True(t, IsExpectedError(noActive))
	assert.True(t, IsExpectedError(pending))
	assert.False(t, IsExpectedError(errors.New("boom")))
	assert.Contains(t, noActive.Error(), "no active signing key")
	assert.Contains(t, pending.Error(), "next key waiting")
}
