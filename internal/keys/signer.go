package keys

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
	"github.com/google/uuid"
)

// The JWS typ headers: RFC 9068 access tokens and OIDC ID tokens.
const (
	TypeAccessToken jose.ContentType = "at+jwt"
	TypeIDToken     jose.ContentType = "JWT"
)

// Claims is the RFC 9068 access token claim set.
type Claims struct {
	Issuer    string   `json:"iss"`
	Subject   string   `json:"sub"`
	Audience  []string `json:"aud"`
	ExpiresAt int64    `json:"exp"`
	IssuedAt  int64    `json:"iat"`
	NotBefore int64    `json:"nbf,omitempty"`
	JWTID     string   `json:"jti"`
	ClientID  string   `json:"client_id,omitempty"`
	SessionID string   `json:"sid,omitempty"`
	Scope     string   `json:"scope,omitempty"`
}

// MintInput is one access token to sign.
type MintInput struct {
	Issuer    string
	Subject   string
	Audience  []string
	ClientID  string
	SessionID string
	Scope     string
	IssuedAt  time.Time
	ExpiresAt time.Time
}

// IDTokenClaims is the OIDC Core ID token claim set; azp only when there are several audiences, auth_time only when set.
type IDTokenClaims struct {
	Issuer          string   `json:"iss"`
	Subject         string   `json:"sub"`
	Audience        []string `json:"aud"`
	ExpiresAt       int64    `json:"exp"`
	IssuedAt        int64    `json:"iat"`
	JWTID           string   `json:"jti"`
	Nonce           string   `json:"nonce,omitempty"`
	AtHash          string   `json:"at_hash,omitempty"`
	AuthorizedParty string   `json:"azp,omitempty"`
	AuthTime        int64    `json:"auth_time,omitempty"`
	SessionID       string   `json:"sid,omitempty"`
}

// MintIDTokenInput is one ID token to sign.
type MintIDTokenInput struct {
	Issuer      string
	Subject     string
	Audience    []string
	ClientID    string
	Nonce       string
	AccessToken string
	SessionID   string
	AuthTime    time.Time
	IssuedAt    time.Time
	ExpiresAt   time.Time
}

// Token is a signed compact JWT.
type Token struct {
	Compact   string
	KID       string
	ExpiresAt time.Time
}

// ActiveKeys returns a project's signing key; ctx carries the project's tenant scope.
type ActiveKeys interface {
	ActiveKey(ctx context.Context, projectID uuid.UUID) (*SigningKey, error)
}

// Signer signs RS256 JWTs with a project's active key.
type Signer struct{ keys ActiveKeys }

// NewSigner builds a Signer over keys.
func NewSigner(keys ActiveKeys) *Signer { return &Signer{keys: keys} }

// Mint signs an access token for projectID.
func (s *Signer) Mint(ctx context.Context, projectID uuid.UUID, in *MintInput) (*Token, error) {
	jti, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("keys: jti: %w", err)
	}
	claims := Claims{
		Issuer: in.Issuer, Subject: in.Subject, Audience: in.Audience,
		ExpiresAt: in.ExpiresAt.Unix(), IssuedAt: in.IssuedAt.Unix(), NotBefore: in.IssuedAt.Unix(),
		JWTID: jti.String(), ClientID: in.ClientID, SessionID: in.SessionID, Scope: in.Scope,
	}
	return s.sign(ctx, projectID, TypeAccessToken, claims, in.ExpiresAt)
}

// MintIDToken signs an ID token for projectID.
func (s *Signer) MintIDToken(ctx context.Context, projectID uuid.UUID, in *MintIDTokenInput) (*Token, error) {
	jti, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("keys: jti: %w", err)
	}
	claims := IDTokenClaims{
		Issuer: in.Issuer, Subject: in.Subject, Audience: in.Audience,
		ExpiresAt: in.ExpiresAt.Unix(), IssuedAt: in.IssuedAt.Unix(), JWTID: jti.String(),
		Nonce: in.Nonce, SessionID: in.SessionID,
	}
	if len(in.Audience) > 1 {
		claims.AuthorizedParty = in.ClientID
	}
	if in.AccessToken != "" {
		claims.AtHash = AtHash(in.AccessToken)
	}
	if !in.AuthTime.IsZero() {
		claims.AuthTime = in.AuthTime.Unix()
	}
	return s.sign(ctx, projectID, TypeIDToken, claims, in.ExpiresAt)
}

// AtHash is the OIDC Core at_hash for RS256: the left half of the SHA-256 of the access token, base64url.
func AtHash(accessToken string) string {
	sum := sha256.Sum256([]byte(accessToken))
	return base64.RawURLEncoding.EncodeToString(sum[:sha256.Size/2])
}

func (s *Signer) sign(ctx context.Context, projectID uuid.UUID, typ jose.ContentType, claims any, expiresAt time.Time) (*Token, error) {
	key, err := s.keys.ActiveKey(ctx, projectID)
	if err != nil {
		return nil, err
	}
	if key.Alg != AlgRS256 {
		return nil, fmt.Errorf("keys: unsupported algorithm %q", key.Alg)
	}
	opts := (&jose.SignerOptions{}).WithType(typ).WithHeader(jose.HeaderKey("kid"), key.KID)
	signer, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.RS256, Key: key.private}, opts)
	if err != nil {
		return nil, fmt.Errorf("keys: signer: %w", err)
	}
	compact, err := jwt.Signed(signer).Claims(claims).Serialize()
	if err != nil {
		return nil, fmt.Errorf("keys: serialize: %w", err)
	}
	return &Token{Compact: compact, KID: key.KID, ExpiresAt: expiresAt}, nil
}
