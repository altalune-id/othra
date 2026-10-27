package secret

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"math/big"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMint_Base64URL(t *testing.T) {
	m, err := Scheme{Prefix: "othra_cs_"}.Mint()
	require.NoError(t, err)
	body, ok := strings.CutPrefix(m.Plaintext, "othra_cs_")
	require.True(t, ok)
	raw, err := base64.RawURLEncoding.DecodeString(body)
	require.NoError(t, err)
	assert.Len(t, raw, 32, "256 random bits, the client-secret and API-key format")
	assert.Equal(t, m.Plaintext[len(m.Plaintext)-4:], m.Hint)

	other, err := Scheme{Prefix: "othra_cs_", HintLen: 6}.Mint()
	require.NoError(t, err)
	assert.Len(t, other.Hint, 6)
	assert.NotEqual(t, m.Plaintext, other.Plaintext)
}

func TestMint_Checksum(t *testing.T) {
	s := Scheme{Prefix: "othra_pt_", Checksum: true}
	m, err := s.Mint()
	require.NoError(t, err)
	body := strings.TrimPrefix(m.Plaintext, "othra_pt_")
	assert.Len(t, body, 49, "43 base62 characters of secret and 6 of CRC32")
	assert.Empty(t, strings.Trim(body, base62))
	assert.True(t, s.Valid(m.Plaintext))

	flipped := []byte(m.Plaintext)
	flipped[12] ^= 1
	assert.False(t, s.Valid(string(flipped)), "a garbled token fails the checksum offline")
	assert.False(t, s.Valid("othra_pt_"+body[:48]))
	assert.False(t, s.Valid("othra_cs_"+body))
	assert.False(t, s.Valid("othra_pt_"+strings.Repeat("-", 49)))
}

func TestEncode62_FitsAll256Bits(t *testing.T) {
	top := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 256), big.NewInt(1))
	out := encode62(top, base62Body)
	assert.Len(t, out, base62Body)
	assert.NotEqual(t, byte('0'), out[0], "2^256-1 needs every one of the 43 places")
	assert.Equal(t, "000000", encode62(big.NewInt(0), base62Sum))
}

func TestValid_NoChecksum(t *testing.T) {
	s := Scheme{Prefix: "othra_rs_"}
	assert.True(t, s.Valid("othra_rs_x"))
	assert.False(t, s.Valid("othra_rs_"))
	assert.False(t, s.Valid("othra_cs_x"))
}

func TestHashes(t *testing.T) {
	sum := sha256.Sum256([]byte("othra_cs_abc"))
	assert.Equal(t, sum, Sum("othra_cs_abc"))
	assert.Equal(t, hex.EncodeToString(sum[:]), Hex("othra_cs_abc"))
	assert.Equal(t, "sha256$"+hex.EncodeToString(sum[:]), Versioned("othra_cs_abc"))
	assert.True(t, Matches(Versioned("a"), "a"))
	assert.False(t, Matches(Versioned("a"), "b"))
	assert.True(t, SumMatches(Sum("a"), "a"))
	assert.False(t, SumMatches(Sum("a"), "b"))
}

func TestNewToken(t *testing.T) {
	raw, hash, err := NewToken()
	require.NoError(t, err)
	assert.Len(t, raw, 32)
	assert.Equal(t, Hex(raw), hash)
}
