// Package keys holds each project's token signing keys: sealed RS256 keys, their rotation, the JWKS and the signer.
package keys

import (
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"time"

	"github.com/google/uuid"
)

// AlgRS256 is the one JWS algorithm keys sign with in M2.
const AlgRS256 = "RS256"

// RetiredKeep is how long a retired key stays listed before it is deleted with its sealed private key.
const RetiredKeep = 30 * 24 * time.Hour

// Status is where a key is in its life.
type Status string

// The key statuses: next is published but does not sign; active signs; retiring stays published until retire_after; retired is not.
const (
	StatusNext     Status = "next"
	StatusActive   Status = "active"
	StatusRetiring Status = "retiring"
	StatusRetired  Status = "retired"
)

// JWK is the public half of a key as RFC 7517 publishes it.
type JWK struct {
	Kty string `json:"kty"`
	Kid string `json:"kid"`
	Use string `json:"use"`
	Alg string `json:"alg"`
	N   string `json:"n"`
	E   string `json:"e"`
}

// JWKS is the published key set.
type JWKS struct {
	Keys []JWK `json:"keys"`
}

// Key is one signing key of a project; it never holds the private key.
type Key struct {
	ID          uuid.UUID
	PublicID    string
	OrgID       uuid.UUID
	ProjectID   uuid.UUID
	KID         string
	Alg         string
	JWK         JWK
	Status      Status
	CreatedAt   time.Time
	ActivatesAt *time.Time
	ActivatedAt *time.Time
	RetireAfter *time.Time
	RetiredAt   *time.Time
}

// Published reports whether the key belongs in the project's JWKS.
func (k *Key) Published() bool {
	return k.Status == StatusNext || k.Status == StatusActive || k.Status == StatusRetiring
}

// Activate makes a next or fresh key the signer at now.
func (k *Key) Activate(now time.Time) {
	k.Status = StatusActive
	k.ActivatedAt = &now
}

// Retire hands the active key over: it stays published until now + overlap.
func (k *Key) Retire(now time.Time, overlap time.Duration) {
	after := now.Add(overlap)
	k.Status = StatusRetiring
	k.RetireAfter = &after
}

// Unpublish moves a retiring key out of the JWKS at now.
func (k *Key) Unpublish(now time.Time) {
	k.Status = StatusRetired
	k.RetiredAt = &now
}

// PublicJWK is the RFC 7517 form of pub; its kid is the RFC 7638 thumbprint.
func PublicJWK(pub *rsa.PublicKey) JWK {
	n := base64.RawURLEncoding.EncodeToString(pub.N.Bytes())
	e := base64.RawURLEncoding.EncodeToString(big.NewInt(int64(pub.E)).Bytes())
	thumb, _ := json.Marshal(struct {
		E   string `json:"e"`
		Kty string `json:"kty"`
		N   string `json:"n"`
	}{E: e, Kty: "RSA", N: n})
	sum := sha256.Sum256(thumb)
	return JWK{Kty: "RSA", Kid: base64.RawURLEncoding.EncodeToString(sum[:]), Use: "sig", Alg: AlgRS256, N: n, E: e}
}

// SECURITY: the AAD binds a sealed private key to its row, so a blob moved to another project or kid does not open.
func sealAAD(projectID uuid.UUID, kid string) []byte {
	return []byte("signing_key:" + projectID.String() + ":" + kid)
}
