package upstream

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSpecOf(t *testing.T) {
	g, ok := SpecOf(KindGoogle)
	assert.True(t, ok)
	assert.Equal(t, "https://accounts.google.com", g.Issuer, "the canonical issuer stored on identities")
	assert.Equal(t, []string{"accounts.google.com"}, g.AcceptIssuers, "Google's second iss form")
	assert.Equal(t, "select_account", g.Prompt, "Google has no prompt=login")
	_, ok = SpecOf("github")
	assert.False(t, ok)
}

func TestRedirectURI(t *testing.T) {
	assert.Equal(t, "https://id.example.com/p/acme/federation/google/callback", RedirectURI("https://id.example.com/p/acme", KindGoogle))
}
