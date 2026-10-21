package enduser

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"altalune.id/othra/apperror"
)

func TestErrors_CarryTheirCodes(t *testing.T) {
	for _, tt := range []struct {
		err interface {
			error
			ToAppError() *apperror.AppError
		}
		code apperror.Code
		is   func(error) bool
	}{
		{&LinkNotUsableError{}, apperror.CodeSignInLinkNotUsable, IsLinkNotUsableError},
		{&EmailDisabledError{}, apperror.CodeSignInEmailDisabled, IsEmailDisabledError},
		{&SessionGoneError{}, apperror.CodeSignInSessionGone, IsSessionGoneError},
	} {
		assert.Equal(t, tt.code, tt.err.ToAppError().Code(), "%T", tt.err)
		assert.True(t, tt.is(tt.err), "%T", tt.err)
		assert.True(t, IsExpectedError(tt.err), "%T", tt.err)
		assert.NotEmpty(t, tt.err.Error())
		assert.Empty(t, tt.err.ToAppError().Meta(), "%T sends no params", tt.err)
	}
	assert.False(t, IsExpectedError(errors.New("boom")))
}
