package auth

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"altalune.id/othra/apperror"
)

func TestInvalidCredentialsError(t *testing.T) {
	err := &InvalidCredentialsError{}
	assert.True(t, IsInvalidCredentialsError(err))
	assert.False(t, IsInvalidCredentialsError(assert.AnError))
	ae := err.ToAppError()
	assert.Equal(t, apperror.CodeAuthInvalidCredentials, ae.Code())
	assert.Equal(t, "Invalid email or password", ae.Message())
}

func TestResetNotUsableError(t *testing.T) {
	for reason, msg := range map[ResetReason]string{
		ResetExpired:  "This reset link has expired. Ask for a new one.",
		ResetUsed:     "This reset link was already used. Ask for a new one if you still need it.",
		ResetNotFound: "This reset link is not valid. Check that you copied the whole link from your email.",
	} {
		err := &ResetNotUsableError{Reason: reason}
		assert.True(t, IsResetNotUsableError(err))
		assert.Equal(t, reason, ResetReasonOf(err))
		ae := err.ToAppError()
		assert.Equal(t, apperror.CodeAuthResetNotUsable, ae.Code())
		assert.Equal(t, msg, ae.Message())
		assert.Equal(t, map[string]string{"reason": string(reason)}, ae.Meta())
	}
	assert.False(t, IsResetNotUsableError(assert.AnError))
	assert.Empty(t, ResetReasonOf(assert.AnError))
}
