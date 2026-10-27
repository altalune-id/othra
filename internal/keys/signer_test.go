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
	assert.Len(t, claims.JWTID, 21, "a nanoid jti")
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
	assert.Len(t, claims.JWTID, 21, "a nanoid, never a UUID on a surface")
	_, uuidErr := uuid.Parse(claims.JWTID)
	assert.Error(t, uuidErr)
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

// SECURITY: an ID token always names its client in aud.
func TestSigner_IDTokenRefusesAWrongAudience(t *testing.T) {
	h := newHarness(t)
	h.generate(t)
	now := time.Now()
	for name, aud := range map[string][]string{"none": nil, "another client": {"c2"}} {
		_, err := NewSigner(h.svc).MintIDToken(context.Background(), h.project, &MintIDTokenInput{
			Issuer: "i", Subject: "u1", Audience: aud, ClientID: "c1", IssuedAt: now, ExpiresAt: now.Add(time.Minute),
		})
		assert.Error(t, err, name)
	}
}

// OIDC Core §5.4: email and email_verified with the email scope, name with profile.
func TestSigner_IDTokenCarriesTheScopedProfile(t *testing.T) {
	h := newHarness(t)
	h.generate(t)
	now := time.Now()
	claims := func(p *Profile) IDTokenClaims {
		tok, err := NewSigner(h.svc).MintIDToken(context.Background(), h.project, &MintIDTokenInput{
			Issuer: "i", Subject: "u1", Audience: []string{"c1"}, ClientID: "c1", SessionID: "sid1",
			IssuedAt: now, ExpiresAt: now.Add(time.Minute), Profile: p,
		})
		require.NoError(t, err)
		parsed, err := jwt.ParseSigned(tok.Compact, []jose.SignatureAlgorithm{jose.RS256})
		require.NoError(t, err)
		var c IDTokenClaims
		require.NoError(t, parsed.UnsafeClaimsWithoutVerification(&c))
		return c
	}
	all := claims(&Profile{Email: "ada@example.com", EmailVerified: true, Name: "Ada", WithEmail: true, WithName: true})
	assert.Equal(t, "ada@example.com", all.Email)
	require.NotNil(t, all.EmailVerified)
	assert.True(t, *all.EmailVerified)
	assert.Equal(t, "Ada", all.Name)
	assert.Equal(t, "sid1", all.SessionID)

	none := claims(&Profile{Email: "ada@example.com", Name: "Ada"})
	assert.Empty(t, none.Email)
	assert.Nil(t, none.EmailVerified)
	assert.Empty(t, none.Name)
}

// RP-Initiated Logout §2: an expired hint still names its session; a hint from another issuer or of another type does not.
func TestVerifier_IDTokenHintIgnoresExpiry(t *testing.T) {
	h := newHarness(t)
	h.generate(t)
	past := time.Now().Add(-24 * time.Hour)
	tok, err := NewSigner(h.svc).MintIDToken(context.Background(), h.project, &MintIDTokenInput{
		Issuer: "https://othra.example.com/p/acme", Subject: "u1", Audience: []string{"c1"}, ClientID: "c1", SessionID: "sid1",
		IssuedAt: past, ExpiresAt: past.Add(5 * time.Minute),
	})
	require.NoError(t, err)
	v := NewVerifier(scopedKeys{h.svc})
	claims, err := v.VerifyIDTokenHint(context.Background(), h.project, "https://othra.example.com/p/acme", tok.Compact)
	require.NoError(t, err)
	assert.Equal(t, "u1", claims.Subject)
	assert.Equal(t, "sid1", claims.SessionID)
	assert.Equal(t, []string{"c1"}, claims.Audience)

	_, err = v.VerifyIDTokenHint(context.Background(), h.project, "https://othra.example.com/p/other", tok.Compact)
	assert.ErrorIs(t, err, ErrInvalidToken, "another issuer")
	_, err = v.VerifyIDTokenHint(context.Background(), uuid.New(), "https://othra.example.com/p/acme", tok.Compact)
	assert.Error(t, err, "another project's keys")
	access := mint(t, h, "https://othra.example.com/p/acme", time.Now(), time.Now().Add(time.Minute))
	_, err = v.VerifyIDTokenHint(context.Background(), h.project, "https://othra.example.com/p/acme", access.Compact)
	assert.ErrorIs(t, err, ErrInvalidToken, "an access token is not an ID token")
	_, err = v.VerifyIDTokenHint(context.Background(), h.project, "https://othra.example.com/p/acme", "not.a.jwt")
	assert.ErrorIs(t, err, ErrInvalidToken)
}

// RFC 9068 §2.2.1: an access token carries auth_time when the grant has one.
func TestSigner_AccessTokenCarriesAuthTime(t *testing.T) {
	h := newHarness(t)
	h.generate(t)
	now := time.Now()
	tok, err := NewSigner(h.svc).Mint(context.Background(), h.project, &MintInput{
		Issuer: "i", Subject: "u1", Audience: []string{"https://api.example"}, ClientID: "c1", AuthTime: now.Add(-time.Hour),
		IssuedAt: now, ExpiresAt: now.Add(time.Minute),
	})
	require.NoError(t, err)
	parsed, err := jwt.ParseSigned(tok.Compact, []jose.SignatureAlgorithm{jose.RS256})
	require.NoError(t, err)
	var claims Claims
	require.NoError(t, parsed.UnsafeClaimsWithoutVerification(&claims))
	assert.Equal(t, now.Add(-time.Hour).Unix(), claims.AuthTime)
}

// RFC 7519 §4.1.3: an access token with one audience sends aud as a string.
func TestSigner_AccessTokenAudIsAString(t *testing.T) {
	h := newHarness(t)
	h.generate(t)
	now := time.Now()
	tok, err := NewSigner(h.svc).Mint(context.Background(), h.project, &MintInput{
		Issuer: "i", Subject: "u1", Audience: []string{"https://api.example/mcp"}, ClientID: "c1", IssuedAt: now, ExpiresAt: now.Add(time.Minute),
	})
	require.NoError(t, err)
	parsed, err := jwt.ParseSigned(tok.Compact, []jose.SignatureAlgorithm{jose.RS256})
	require.NoError(t, err)
	var raw map[string]any
	require.NoError(t, parsed.UnsafeClaimsWithoutVerification(&raw))
	assert.Equal(t, "https://api.example/mcp", raw["aud"])

	v := NewVerifier(scopedKeys{h.svc}, WithExpectedType(TypeAccessToken))
	claims, err := v.Verify(context.Background(), h.project, tok.Compact)
	require.NoError(t, err, "the verifier reads a string aud")
	assert.Equal(t, []string{"https://api.example/mcp"}, []string(claims.Audience))
}
