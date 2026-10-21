package emailaddr

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalize(t *testing.T) {
	got, err := Normalize("  Ada@Example.COM ")
	require.NoError(t, err)
	assert.Equal(t, "ada@example.com", got)

	long := strings.Repeat("a", MaxLen-len("@x.io")) + "@x.io"
	got, err = Normalize(long)
	require.NoError(t, err)
	assert.Equal(t, long, got)

	for _, bad := range []string{"", "   ", "nope", "Ada <ada@example.com>", "a@b.c, d@e.f", "a" + long} {
		_, err := Normalize(bad)
		assert.ErrorIs(t, err, ErrInvalid, bad)
	}
}
