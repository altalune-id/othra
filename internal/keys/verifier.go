package keys

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
	"github.com/google/uuid"
)

// DefaultClockSkew absorbs clock drift between the host that signed and the host that verifies.
const DefaultClockSkew = 30 * time.Second

// ErrInvalidToken is the cause of every token Verify refuses.
var ErrInvalidToken = errors.New("keys: invalid token")

// PublishedKeys returns a project's published keys; ctx carries the project's tenant scope.
type PublishedKeys interface {
	Published(ctx context.Context, projectID uuid.UUID) ([]*Key, error)
}

// Verifier checks RS256 JWTs against a project's published keys.
type Verifier struct {
	keys      PublishedKeys
	now       func() time.Time
	issuer    string
	typ       jose.ContentType
	clockSkew time.Duration
}

// VerifierOption tunes a Verifier.
type VerifierOption func(*Verifier)

// WithExpectedIssuer refuses tokens whose iss is not want.
func WithExpectedIssuer(want string) VerifierOption { return func(v *Verifier) { v.issuer = want } }

// WithExpectedType refuses tokens whose typ header is not typ, so an access token is never read as an ID token.
func WithExpectedType(typ jose.ContentType) VerifierOption { return func(v *Verifier) { v.typ = typ } }

// WithVerifierClock sets the clock.
func WithVerifierClock(now func() time.Time) VerifierOption { return func(v *Verifier) { v.now = now } }

// NewVerifier builds a Verifier over keys.
func NewVerifier(keys PublishedKeys, opts ...VerifierOption) *Verifier {
	v := &Verifier{keys: keys, now: time.Now, clockSkew: DefaultClockSkew}
	for _, o := range opts {
		o(v)
	}
	return v
}

// Verify checks the signature by kid, the typ, exp, iat, nbf and iss of a compact RS256 token of projectID.
func (v *Verifier) Verify(ctx context.Context, projectID uuid.UUID, compact string) (*Claims, error) {
	var claims Claims
	if err := v.signed(ctx, projectID, compact, v.typ, &claims); err != nil {
		return nil, err
	}
	now := v.now().Unix()
	skew := int64(v.clockSkew.Seconds())
	switch {
	case claims.ExpiresAt == 0:
		return nil, fmt.Errorf("%w: missing exp", ErrInvalidToken)
	case now >= claims.ExpiresAt:
		return nil, fmt.Errorf("%w: expired", ErrInvalidToken)
	case claims.IssuedAt > now+skew:
		return nil, fmt.Errorf("%w: iat is in the future", ErrInvalidToken)
	case claims.NotBefore > now+skew:
		return nil, fmt.Errorf("%w: nbf is in the future", ErrInvalidToken)
	case v.issuer != "" && claims.Issuer != v.issuer:
		return nil, fmt.Errorf("%w: iss does not match", ErrInvalidToken)
	}
	return &claims, nil
}

// VerifyIDTokenHint checks an ID token of projectID used as id_token_hint: signed by a published key, typ JWT, iss issuer; exp is ignored.
func (v *Verifier) VerifyIDTokenHint(ctx context.Context, projectID uuid.UUID, issuer, compact string) (*IDTokenClaims, error) {
	var claims IDTokenClaims
	if err := v.signed(ctx, projectID, compact, TypeIDToken, &claims); err != nil {
		return nil, err
	}
	switch {
	case claims.Issuer != issuer:
		return nil, fmt.Errorf("%w: iss does not match", ErrInvalidToken)
	case claims.Subject == "", len(claims.Audience) == 0:
		return nil, fmt.Errorf("%w: sub and aud are required", ErrInvalidToken)
	}
	return &claims, nil
}

func (v *Verifier) signed(ctx context.Context, projectID uuid.UUID, compact string, typ jose.ContentType, dst any) error {
	parsed, err := jwt.ParseSigned(compact, []jose.SignatureAlgorithm{jose.RS256})
	if err != nil {
		return fmt.Errorf("%w: parse: %s", ErrInvalidToken, err.Error())
	}
	if len(parsed.Headers) != 1 || parsed.Headers[0].KeyID == "" {
		return fmt.Errorf("%w: one signature with a kid is required", ErrInvalidToken)
	}
	h := parsed.Headers[0]
	if typ != "" {
		if got, _ := h.ExtraHeaders[jose.HeaderType].(string); got != string(typ) {
			return fmt.Errorf("%w: typ %q", ErrInvalidToken, got)
		}
	}
	keys, err := v.keys.Published(ctx, projectID)
	if err != nil {
		return err
	}
	pub, err := publicKeyFor(keys, h.KeyID)
	if err != nil {
		return err
	}
	if err := parsed.Claims(pub, dst); err != nil {
		return fmt.Errorf("%w: claims: %s", ErrInvalidToken, err.Error())
	}
	return nil
}

func publicKeyFor(keys []*Key, kid string) (*rsa.PublicKey, error) {
	for _, k := range keys {
		if k.KID != kid {
			continue
		}
		n, errN := base64.RawURLEncoding.DecodeString(k.JWK.N)
		e, errE := base64.RawURLEncoding.DecodeString(k.JWK.E)
		if errN != nil || errE != nil {
			return nil, fmt.Errorf("%w: key %s is malformed", ErrInvalidToken, kid)
		}
		return &rsa.PublicKey{N: new(big.Int).SetBytes(n), E: int(new(big.Int).SetBytes(e).Int64())}, nil
	}
	return nil, fmt.Errorf("%w: unknown kid %q", ErrInvalidToken, kid)
}
