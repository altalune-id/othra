package upstream

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/altalune-id/othra/apperror"
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
		{&NotFoundError{}, apperror.CodeUpstreamNotFound, IsNotFoundError},
		{&InvalidConfigError{Reason: ConfigClientID}, apperror.CodeUpstreamInvalidConfig, IsInvalidConfigError},
		{&ExistsError{}, apperror.CodeUpstreamExists, IsExistsError},
		{&HasIdentitiesError{}, apperror.CodeUpstreamHasIdentities, IsHasIdentitiesError},
	} {
		assert.Equal(t, tt.code, tt.err.ToAppError().Code(), "%T", tt.err)
		assert.True(t, tt.is(tt.err), "%T", tt.err)
		assert.True(t, IsExpectedError(tt.err), "%T", tt.err)
		assert.NotEmpty(t, tt.err.Error())
	}
	assert.False(t, IsExpectedError(errors.New("x")))
}
