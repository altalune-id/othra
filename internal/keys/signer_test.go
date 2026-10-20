package keys

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type scopedKeys struct{ svc *Service }

func (k scopedKeys) Published(ctx context.Context, projectID uuid.UUID) ([]*Key, error) {
	return k.svc.Published(ctx, projectID)
}

func mint(t *testing.T, h *harness, iss string, iat, exp time.Time) *Token {
	t.Helper()
	tok, err := NewSigner(h.svc).Mint(context.Background(), h.project, &MintInput{
		Issuer: iss, Subject: "u1", Audience: []string{"https://api.example.com"}, ClientID: "c1", Scope: "openid",
		IssuedAt: iat, ExpiresAt: exp,
	})
	require.NoError(t, err)
	return tok
}

func TestSigner_RS256RoundTrip(t *testing.T) {
	h := newHarness(t)
	k := h.generate(t)
	now := time.Now()
	tok := mint(t, h, "https://othra.example.com/p/acme", now, now.Add(10*time.Minute))
	assert.Equal(t, k.KID, tok.KID)

	parsed, err := jwt.ParseSigned(tok.Compact, []jose.SignatureAlgorithm{jose.RS256})
	require.NoError(t, err)
	assert.Equal(t, k.KID, parsed.Headers[0].KeyID)
	assert.Equal(t, "at+jwt", parsed.Headers[0].ExtraHeaders[jose.HeaderType])

	v := NewVerifier(scopedKeys{h.svc}, WithExpectedIssuer("https://othra.example.com/p/acme"), WithExpectedType(TypeAccessToken))
	claims, err := v.Verify(context.Background(), h.project, tok.Compact)
	require.NoError(t, err)
	assert.Equal(t, "u1", claims.Subject)
	assert.Equal(t, "c1", claims.ClientID)
	assert.NotEmpty(t, claims.JWTID)
}

func TestSigner_IDTokenCarriesAtHashAndAzp(t *testing.T) {
	h := newHarness(t)
	h.generate(t)
	now := time.Now()
	tok, err := NewSigner(h.svc).MintIDToken(context.Background(), h.project, &MintIDTokenInput{
		Issuer: "i", Subject: "u1", Audience: []string{"c1", "c2"}, ClientID: "c1", Nonce: "n",
		AccessToken: "at", AuthTime: now, IssuedAt: now, ExpiresAt: now.Add(5 * time.Minute),
	})
	require.NoError(t, err)
	parsed, err := jwt.ParseSigned(tok.Compact, []jose.SignatureAlgorithm{jose.RS256})
	require.NoError(t, err)
	assert.Equal(t, "JWT", parsed.Headers[0].ExtraHeaders[jose.HeaderType])
	var claims IDTokenClaims
	require.NoError(t, parsed.UnsafeClaimsWithoutVerification(&claims))
	assert.Equal(t, AtHash("at"), claims.AtHash)
	assert.Equal(t, "c1", claims.AuthorizedParty)
	assert.Equal(t, now.Unix(), claims.AuthTime)

	single, err := NewSigner(h.svc).MintIDToken(context.Background(), h.project, &MintIDTokenInput{
		Issuer: "i", Subject: "u1", Audience: []string{"c1"}, ClientID: "c1", IssuedAt: now, ExpiresAt: now.Add(time.Minute),
	})
	require.NoError(t, err)
	parsed, err = jwt.ParseSigned(single.Compact, []jose.SignatureAlgorithm{jose.RS256})
	require.NoError(t, err)
	var plain IDTokenClaims
	require.NoError(t, parsed.UnsafeClaimsWithoutVerification(&plain))
	assert.Empty(t, plain.AuthorizedParty)
	assert.Empty(t, plain.AtHash)
	assert.Zero(t, plain.AuthTime)
}

func TestAtHash_IsTheLeftHalfOfSHA256(t *testing.T) {
	assert.Equal(t, "77QmUPtjPfzWtF2AnpK9RQ", AtHash("jHkWEdUXMU1BwAsC4vtUsZwnNvTIxEl0z9K3vx5KF0Y"), "OIDC Core A.3")
}

func TestVerifier_Refusals(t *testing.T) {
	h := newHarness(t)
	h.generate(t)
	now := time.Now()
	good := mint(t, h, "iss", now, now.Add(time.Minute))
	v := NewVerifier(scopedKeys{h.svc})

	for name, tc := range map[string]struct {
		v       *Verifier
		project uuid.UUID
		compact string
		want    string
	}{
		"garbage":           {v, h.project, "not.a.jwt", "parse"},
		"expired":           {v, h.project, mint(t, h, "iss", now.Add(-time.Hour), now.Add(-time.Minute)).Compact, "expired"},
		"future iat":        {v, h.project, mint(t, h, "iss", now.Add(time.Hour), now.Add(2*time.Hour)).Compact, "future"},
		"other issuer":      {NewVerifier(scopedKeys{h.svc}, WithExpectedIssuer("other")), h.project, good.Compact, "iss"},
		"id token expected": {NewVerifier(scopedKeys{h.svc}, WithExpectedType(TypeIDToken)), h.project, good.Compact, "typ"},
		"other project":     {v, uuid.New(), good.Compact, "unknown kid"},
		"bad signature":     {v, h.project, good.Compact[:len(good.Compact)-4] + "AAAA", "claims"},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := tc.v.Verify(context.Background(), tc.project, tc.compact)
			require.Error(t, err)
			assert.True(t, errors.Is(err, ErrInvalidToken), "%v", err)
			assert.Contains(t, err.Error(), tc.want)
		})
	}
}

func TestVerifier_AcceptsARetiringKeyAndRefusesARetiredOne(t *testing.T) {
	h := newHarness(t)
	h.generate(t)
	h.now = time.Now()
	tok := mint(t, h, "iss", h.now, h.now.Add(time.Hour))
	_, err := h.svc.Rotate(context.Background(), h.org, h.project, true)
	require.NoError(t, err)
	v := NewVerifier(scopedKeys{h.svc}, WithVerifierClock(func() time.Time { return h.now }))
	_, err = v.Verify(context.Background(), h.project, tok.Compact)
	require.NoError(t, err, "a retiring key still verifies within the overlap")

	h.now = h.now.Add(7*24*time.Hour + time.Second)
	_, err = h.svc.Prune(context.Background(), h.org, h.project)
	require.NoError(t, err)
	_, err = v.Verify(context.Background(), h.project, tok.Compact)
	assert.ErrorContains(t, err, "unknown kid")
}

func TestSigner_NeedsAnActiveKey(t *testing.T) {
	h := newHarness(t)
	_, err := NewSigner(h.svc).Mint(context.Background(), h.project, &MintInput{IssuedAt: time.Now(), ExpiresAt: time.Now().Add(time.Minute)})
	assert.True(t, IsNoActiveKeyError(err), "%v", err)
	assert.False(t, strings.Contains(err.Error(), "BEGIN"))
}
