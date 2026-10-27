package session

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestEndUserContext(t *testing.T) {
	_, ok := EndUserFrom(context.Background())
	assert.False(t, ok, "outside the account plane there is no end user")
	eu := EndUser{EndUserID: uuid.New(), Subject: "eu-1"}
	got, ok := EndUserFrom(EndUserInto(context.Background(), eu))
	assert.True(t, ok)
	assert.Equal(t, eu, got)
}
