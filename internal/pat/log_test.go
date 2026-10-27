package pat

import (
	"bytes"
	"context"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// R-7: create and revoke log the token's public id as token_id, never the token itself.
func TestService_LogsTheTokenID(t *testing.T) {
	h := newHarness(t)
	var logs bytes.Buffer
	h.svc.WithLogger(slog.New(slog.NewJSONHandler(&logs, nil)))
	c := h.create(t)
	require.NoError(t, h.svc.Revoke(context.Background(), h.p, h.user, c.Token.PublicID))
	assert.Contains(t, logs.String(), `"msg":"pat: created"`)
	assert.Contains(t, logs.String(), `"token_id":"`+c.Token.PublicID+`"`)
	assert.Contains(t, logs.String(), `"msg":"pat: revoked"`)
	assert.NotContains(t, logs.String(), `"token":`)
	assert.NotContains(t, logs.String(), c.Secret)
}
