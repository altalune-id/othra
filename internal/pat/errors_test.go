package pat

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"altalune.id/othra/apperror"
)

func TestErrors(t *testing.T) {
	for _, tc := range []struct {
		err  interface{ ToAppError() *apperror.AppError }
		code apperror.Code
		meta map[string]string
	}{
		{&NotFoundError{}, apperror.CodeAccountTokenNotFound, nil},
		{&InvalidError{Reason: InvalidScopes}, apperror.CodeAccountTokenInvalid, map[string]string{"reason": "scopes"}},
		{&ServerClosedError{Project: "Acme"}, apperror.CodeAccountTokenServerClosed, map[string]string{"project": "Acme"}},
		{&LimitError{Max: MaxLive}, apperror.CodeAccountTokenLimit, map[string]string{"max": "20"}},
		{&ReauthRequiredError{}, apperror.CodeAccountReauthRequired, nil},
	} {
		err := fmt.Errorf("wrapped: %w", tc.err.(error))
		assert.True(t, IsExpectedError(err), string(tc.code))
		assert.NotEmpty(t, err.Error())
		app := tc.err.ToAppError()
		assert.Equal(t, tc.code, app.Code(), string(tc.code))
		if tc.meta != nil {
			assert.Equal(t, tc.meta, app.Meta(), string(tc.code))
		}
	}
	assert.False(t, IsExpectedError(fmt.Errorf("boom")))
}
