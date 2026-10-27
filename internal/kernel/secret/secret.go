// Package secret mints and hashes Othra's credentials: prefixed secrets shown once and opaque tokens, stored only as hashes.
package secret

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"hash/crc32"
	"math/big"
	"strings"

	"altalune.id/othra/nanoid"
)

const (
	secretBytes   = 32
	tokenLength   = 32
	defaultHint   = 4
	versionPrefix = "sha256$"
	base62        = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	base62Body    = 43
	base62Sum     = 6
)

// Scheme is one kind of prefixed credential; a Checksum scheme encodes in base62 and appends a CRC32 for secret scanners.
type Scheme struct {
	Prefix   string
	HintLen  int
	Checksum bool
}

// Minted is a credential just made: the plaintext, shown once, and the hint kept for display.
type Minted struct {
	Plaintext string
	Hint      string
}

// Mint makes a credential of 256 random bits under the scheme.
func (s Scheme) Mint() (Minted, error) {
	raw := make([]byte, secretBytes)
	if _, err := rand.Read(raw); err != nil {
		return Minted{}, err
	}
	body := base64.RawURLEncoding.EncodeToString(raw)
	if s.Checksum {
		body = encode62(new(big.Int).SetBytes(raw), base62Body)
		body += checksum(body)
	}
	plaintext := s.Prefix + body
	return Minted{Plaintext: plaintext, Hint: plaintext[len(plaintext)-s.hintLen():]}, nil
}

// Valid reports whether plaintext is shaped like the scheme's credentials, checksum included; it says nothing about whether one exists.
func (s Scheme) Valid(plaintext string) bool {
	body, ok := strings.CutPrefix(plaintext, s.Prefix)
	if !ok || !s.Checksum {
		return ok && body != ""
	}
	if len(body) != base62Body+base62Sum || strings.Trim(body, base62) != "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(body[base62Body:]), []byte(checksum(body[:base62Body]))) == 1
}

func (s Scheme) hintLen() int {
	if s.HintLen > 0 {
		return s.HintLen
	}
	return defaultHint
}

func checksum(body string) string {
	return encode62(new(big.Int).SetUint64(uint64(crc32.ChecksumIEEE([]byte(body)))), base62Sum)
}

func encode62(n *big.Int, width int) string {
	out := make([]byte, width)
	base, mod := big.NewInt(int64(len(base62))), new(big.Int)
	for i := width - 1; i >= 0; i-- {
		n.DivMod(n, base, mod)
		out[i] = base62[mod.Int64()]
	}
	return string(out)
}

// Sum is the SHA-256 of a credential, the form API keys store.
func Sum(plaintext string) [sha256.Size]byte { return sha256.Sum256([]byte(plaintext)) }

// SumMatches compares a presented credential with a stored Sum in constant time.
func SumMatches(stored [sha256.Size]byte, presented string) bool {
	sum := Sum(presented)
	return subtle.ConstantTimeCompare(stored[:], sum[:]) == 1
}

// Hex is the SHA-256 of a credential in hex, the form opaque tokens and personal tokens store.
func Hex(plaintext string) string {
	sum := Sum(plaintext)
	return hex.EncodeToString(sum[:])
}

// Versioned is the stored form of a client or resource-server secret: "sha256$" and its hex SHA-256.
func Versioned(plaintext string) string { return versionPrefix + Hex(plaintext) }

// Matches compares a presented secret with a stored Versioned hash in constant time.
func Matches(versioned, presented string) bool {
	return subtle.ConstantTimeCompare([]byte(versioned), []byte(Versioned(presented))) == 1
}

// NewToken returns an opaque 32-character token and its Hex hash, for codes and links the server looks up by hash.
func NewToken() (raw, hash string, err error) {
	raw, err = nanoid.New(tokenLength)
	if err != nil {
		return "", "", err
	}
	return raw, Hex(raw), nil
}
