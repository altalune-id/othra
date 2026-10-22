package grant

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"slices"
	"time"

	"github.com/google/uuid"
)

// The scopes and the challenge method a grant knows by name.
const (
	ScopeOpenID        = "openid"
	ScopeOfflineAccess = "offline_access"
	MethodS256         = "S256"
)

// Revocation reasons, as stored in grant_families.revoked_reason.
const (
	ReasonReuse          = "reuse"
	ReasonCodeReplay     = "code_replay"
	ReasonClientMismatch = "client_mismatch"
	ReasonEndSession     = "end_session"
	ReasonSignOut        = "sign_out"
)

const (
	publicIDLen  = 21
	usedCodeKeep = 24 * time.Hour
	revokedKeep  = 7 * 24 * time.Hour
)

// Config holds the lifetimes of what the service issues.
type Config struct {
	CodeTTL         time.Duration
	AccessTokenTTL  time.Duration
	RefreshIdle     time.Duration
	RefreshAbsolute time.Duration
}

// Project is the project a grant belongs to.
type Project struct {
	OrgID uuid.UUID
	ID    uuid.UUID
}

// Client is the client that authenticated at the token endpoint.
type Client struct {
	ID       uuid.UUID
	PublicID string
	// Refresh reports whether the client holds the refresh_token grant type.
	Refresh bool
}

// Code is a stored authorization code; only its hash is kept.
type Code struct {
	ID            uuid.UUID
	ClientID      uuid.UUID
	EndUserID     uuid.UUID
	UserSessionID *uuid.UUID
	UserSession   string
	RedirectURI   string
	Scopes        []string
	CodeChallenge string
	Nonce         string
	AuthTime      time.Time
	ExpiresAt     time.Time
	UsedAt        *time.Time
	FamilyID      *uuid.UUID
	FamilySID     string
}

// Family is a grant: one code exchange and the tokens it led to; its PublicID is the sid.
type Family struct {
	ID                uuid.UUID
	PublicID          string
	ClientID          uuid.UUID
	EndUserID         uuid.UUID
	UserSessionID     *uuid.UUID
	UserSession       string
	Scopes            []string
	AuthTime          time.Time
	CreatedAt         time.Time
	LastUsedAt        time.Time
	AbsoluteExpiresAt time.Time
	RevokedAt         *time.Time
}

// Live reports whether the family is neither revoked nor past its absolute lifetime at now.
func (f *Family) Live(now time.Time) bool {
	return f.RevokedAt == nil && now.Before(f.AbsoluteExpiresAt)
}

// RefreshToken is a stored refresh token; only its hash is kept.
type RefreshToken struct {
	ID        uuid.UUID
	FamilyID  uuid.UUID
	ExpiresAt time.Time
	UsedAt    *time.Time
}

// Access is a live access token with the grant it belongs to.
type Access struct {
	Family    *Family
	Scopes    []string
	ExpiresAt time.Time
}

// Tokens is what a code exchange or a refresh issued; the plaintext tokens are never stored.
type Tokens struct {
	Family       *Family
	AccessToken  string
	ExpiresAt    time.Time
	Scopes       []string
	RefreshToken string
	// Nonce is the nonce of the authorization request; a refresh has none.
	Nonce string
}

// SECURITY: RFC 7636 §4.1, a verifier is 43 to 128 unreserved characters; compared in constant time (§4.6).
func verifyPKCE(challenge, verifier string) bool {
	if challenge == "" {
		return verifier == ""
	}
	if !validVerifier(verifier) {
		return false
	}
	sum := sha256.Sum256([]byte(verifier))
	return subtle.ConstantTimeCompare([]byte(base64.RawURLEncoding.EncodeToString(sum[:])), []byte(challenge)) == 1
}

func validVerifier(v string) bool {
	if len(v) < 43 || len(v) > 128 {
		return false
	}
	for _, c := range []byte(v) {
		switch {
		case c >= 'A' && c <= 'Z', c >= 'a' && c <= 'z', c >= '0' && c <= '9', c == '-', c == '.', c == '_', c == '~':
		default:
			return false
		}
	}
	return true
}

func subset(want, have []string) bool {
	for _, s := range want {
		if !slices.Contains(have, s) {
			return false
		}
	}
	return true
}
