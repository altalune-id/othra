package boot

import (
	"crypto/rand"
	"crypto/rsa"
	"testing"

	"github.com/go-jose/go-jose/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/altalune-id/othra/oidcrp"
	"github.com/altalune-id/othra/oidcrp/oidcrptest"
)

func signedBy(t *testing.T, kid string) string {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	signer, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.RS256, Key: jose.JSONWebKey{Key: key, KeyID: kid}}, nil)
	require.NoError(t, err)
	jws, err := signer.Sign([]byte(`{"sub":"s"}`))
	require.NoError(t, err)
	raw, err := jws.CompactSerialize()
	require.NoError(t, err)
	return raw
}

// RPL §2: a hint whose key /p/system still publishes is kept; a retired, forged or garbled one is left out.
func TestLiveHint(t *testing.T) {
	idp := oidcrptest.New(t)
	keys := oidcrp.NewKeySource(idp.Issuer(), idp.Client())
	live := idp.IDToken(nil)
	assert.Equal(t, live, liveHint(t.Context(), keys, live))

	idp.RotateKey()
	assert.Equal(t, live, liveHint(t.Context(), keys, live), "an older key still published verifies")
	assert.Empty(t, liveHint(t.Context(), keys, signedBy(t, "retired")), "a kid no longer published")
	assert.Empty(t, liveHint(t.Context(), keys, signedBy(t, "k1")), "a published kid with another key's signature")
	assert.Empty(t, liveHint(t.Context(), keys, "not.a.jws"))
	assert.Empty(t, liveHint(t.Context(), oidcrp.NewKeySource("https://127.0.0.1:1", idp.Client()), live), "no discovery")
}
