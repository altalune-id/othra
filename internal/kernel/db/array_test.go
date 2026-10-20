package db

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTextArrayLiteral(t *testing.T) {
	assert.Equal(t, "{}", TextArrayLiteral(nil))
	assert.Equal(t, `{"a","b"}`, TextArrayLiteral([]string{"a", "b"}))
	assert.Equal(t, `{"a\"b","c\\d","e,f"}`, TextArrayLiteral([]string{`a"b`, `c\d`, "e,f"}))
	assert.NotNil(t, TextArray([]string{"a"}))
}
