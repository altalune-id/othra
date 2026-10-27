package keys

import (
	"context"
	"crypto/rsa"
	"encoding/json"
	"math/big"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/altalune-id/othra/apperror"
	"github.com/altalune-id/othra/sealer"
)

func TestPublicJWK_KidIsTheRFC7638Thumbprint(t *testing.T) {
	raw := "0vx7agoebGcQSuuPiLJXZptN9nndrQmbXEps2aiAFbWhM78LhWx4cbbfAAtVT86zwu1RK7aPFFxuhDR1L6tSoc_BJECPebWKRXjBZCiFV4n3oknjhMstn64tZ_2W-5JsGY4Hc5n9yBXArwl93lqt7_RN5w6Cf0h4QyQ5v-65YGjQR0_FDW2QvzqY368QQMicAtaSqzs8KJZgnYb9c7d0zgdAZHzu6qMQvRL5hajrn1n91CbOpbISD08qNLyrdkt-bFTWhAI4vMQFh6WeZu0fM4lFd2NcRwr3XPksINHaQ-G_xBniIqbw0Ls1jF44-csFCur-kEgU8awapJzKnqDKgw"
	b, err := decode(raw)
	require.NoError(t, err)
	n := new(big.Int).SetBytes(b)
	jwk := PublicJWK(&rsa.PublicKey{N: n, E: 65537})
	assert.Equal(t, "NzbLsXh8uDCcd-6MNwXF4W_7noWXFZAfHkxZsRGC9Xs", jwk.Kid, "RFC 7638 §3.1 example")
	assert.Equal(t, JWK{Kty: "RSA", Kid: jwk.Kid, Use: "sig", Alg: "RS256", N: raw, E: "AQAB"}, jwk)
}

func TestGenerate_GivesTheProjectAnActiveSealedKey(t *testing.T) {
	h := newHarness(t)
	k := h.generate(t)
	assert.Equal(t, StatusActive, k.Status)
	assert.Equal(t, h.now, *k.ActivatedAt)
	assert.Equal(t, AlgRS256, k.Alg)
	assert.Equal(t, k.JWK.Kid, k.KID)
	assert.Len(t, k.PublicID, publicIDLen)
	assert.Equal(t, []uuid.UUID{h.project}, h.changed)

	sealed := h.store.rows[k.ID].sealed
	assert.NotContains(t, string(sealed), "PRIVATE KEY")
	sk, err := h.svc.ActiveKey(context.Background(), h.project)
	require.NoError(t, err)
	assert.Equal(t, k.KID, sk.KID)
}

// SECURITY: a sealed private key opens only on its own row; moved to another project or kid it fails.
func TestActiveKey_ASealedKeyMovedToAnotherRowDoesNotOpen(t *testing.T) {
	h := newHarness(t)
	a := h.generate(t)
	other := uuid.New()
	h.store.rows[a.ID].key.ProjectID = other
	_, err := h.svc.ActiveKey(context.Background(), other)
	assert.Equal(t, apperror.CodeUnexpected, codeOf(err), "%v", err)

	h2 := newHarness(t)
	b := h2.generate(t)
	h2.store.rows[b.ID].key.KID = "another-kid"
	_, err = h2.svc.ActiveKey(context.Background(), h2.project)
	assert.Equal(t, apperror.CodeUnexpected, codeOf(err))
	assert.NotContains(t, err.Error(), "BEGIN")
}

func TestGenerate_FailsWithoutASealer(t *testing.T) {
	h := newHarness(t)
	h.svc.sealer = sealer.Disabled()
	_, err := h.svc.Generate(context.Background(), h.org, h.project)
	assert.Equal(t, apperror.CodeUnexpected, codeOf(err))
	assert.Empty(t, h.store.rows)
}

func TestRotate_PublishesANextKeyThenPromotesIt(t *testing.T) {
	h := newHarness(t)
	first := h.generate(t)
	h.changed = nil

	next, err := h.svc.Rotate(context.Background(), h.org, h.project, false)
	require.NoError(t, err)
	assert.Equal(t, StatusNext, next.Status)
	assert.Equal(t, h.now.Add(10*time.Minute), *next.ActivatesAt)
	assert.Equal(t, map[Status]int{StatusActive: 1, StatusNext: 1}, h.store.status(h.project))
	assert.Equal(t, []uuid.UUID{h.project}, h.changed)

	_, err = h.svc.Rotate(context.Background(), h.org, h.project, false)
	assert.True(t, IsRotationPendingError(err), "%v", err)
	assert.Equal(t, apperror.CodeSigningKeyRotationPending, codeOf(err))

	ok, err := h.svc.Promote(context.Background(), h.org, h.project)
	require.NoError(t, err)
	assert.False(t, ok, "not before activates_at")

	h.now = h.now.Add(10 * time.Minute)
	ok, err = h.svc.Promote(context.Background(), h.org, h.project)
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, map[Status]int{StatusActive: 1, StatusRetiring: 1}, h.store.status(h.project))
	old := h.store.rows[first.ID].key
	assert.Equal(t, h.now.Add(7*24*time.Hour), *old.RetireAfter)
	sk, err := h.svc.ActiveKey(context.Background(), h.project)
	require.NoError(t, err)
	assert.Equal(t, next.KID, sk.KID, "the promoted key signs; the cache was dropped")
}

func TestRotate_ImmediatePromotesAtOnce(t *testing.T) {
	h := newHarness(t)
	h.generate(t)
	k, err := h.svc.Rotate(context.Background(), h.org, h.project, true)
	require.NoError(t, err)
	assert.Equal(t, StatusActive, k.Status)
	assert.Equal(t, map[Status]int{StatusActive: 1, StatusRetiring: 1}, h.store.status(h.project))

	waiting, err := h.svc.Rotate(context.Background(), h.org, h.project, false)
	require.NoError(t, err)
	promoted, err := h.svc.Rotate(context.Background(), h.org, h.project, true)
	require.NoError(t, err)
	assert.Equal(t, waiting.ID, promoted.ID, "--now promotes the key that was already waiting")
	assert.Equal(t, map[Status]int{StatusActive: 1, StatusRetiring: 2}, h.store.status(h.project))
}

func TestRotate_WithNoActiveKeyMakesOneAtOnce(t *testing.T) {
	h := newHarness(t)
	k, err := h.svc.Rotate(context.Background(), h.org, h.project, false)
	require.NoError(t, err)
	assert.Equal(t, StatusActive, k.Status)

	h2 := newHarness(t)
	k2 := h2.generate(t)
	h2.store.rows[k2.ID].key.Status = StatusNext
	repaired, err := h2.svc.Rotate(context.Background(), h2.org, h2.project, false)
	require.NoError(t, err)
	assert.Equal(t, k2.ID, repaired.ID, "a waiting key with no active one takes over at once")
	assert.Equal(t, StatusActive, h2.store.rows[k2.ID].key.Status)
}

func TestPrune_UnpublishesThenDeletes(t *testing.T) {
	h := newHarness(t)
	first := h.generate(t)
	_, err := h.svc.Rotate(context.Background(), h.org, h.project, true)
	require.NoError(t, err)

	n, err := h.svc.Prune(context.Background(), h.org, h.project)
	require.NoError(t, err)
	assert.Zero(t, n, "still within the overlap")

	h.now = h.now.Add(7 * 24 * time.Hour)
	h.changed = nil
	n, err = h.svc.Prune(context.Background(), h.org, h.project)
	require.NoError(t, err)
	assert.Equal(t, 1, n)
	assert.Equal(t, StatusRetired, h.store.rows[first.ID].key.Status)
	assert.Nil(t, h.store.rows[first.ID].sealed, "a retired key keeps no private key")
	assert.Equal(t, []uuid.UUID{h.project}, h.changed)

	h.now = h.now.Add(RetiredKeep + time.Second)
	n, err = h.svc.Prune(context.Background(), h.org, h.project)
	require.NoError(t, err)
	assert.Equal(t, 1, n)
	assert.NotContains(t, h.store.rows, first.ID, "deleted with its sealed key after 30 days")
}

func TestJWKS_IsStableCachedAndDroppedOnChange(t *testing.T) {
	h := newHarness(t)
	first := h.generate(t)
	h.now = h.now.Add(time.Minute)
	next, err := h.svc.Rotate(context.Background(), h.org, h.project, false)
	require.NoError(t, err)

	body, err := h.svc.JWKS(context.Background(), h.project)
	require.NoError(t, err)
	var set JWKS
	require.NoError(t, json.Unmarshal(body, &set))
	require.Len(t, set.Keys, 2)
	assert.Equal(t, first.KID, set.Keys[0].Kid, "oldest first")
	assert.Equal(t, next.KID, set.Keys[1].Kid)
	assert.NotContains(t, string(body), "\"d\"", "no private member")

	reads := h.store.reads
	again, err := h.svc.JWKS(context.Background(), h.project)
	require.NoError(t, err)
	assert.Equal(t, body, again)
	assert.Equal(t, reads, h.store.reads, "served from the cache")

	h.now = h.now.Add(CacheTTL)
	fresh, err := h.svc.JWKS(context.Background(), h.project)
	require.NoError(t, err)
	assert.Equal(t, body, fresh, "the same keys give the same bytes")
	assert.Equal(t, reads+1, h.store.reads)

	h.now = h.now.Add(10 * time.Minute)
	_, err = h.svc.Promote(context.Background(), h.org, h.project)
	require.NoError(t, err)
	h.now = h.now.Add(7 * 24 * time.Hour)
	_, err = h.svc.Prune(context.Background(), h.org, h.project)
	require.NoError(t, err)
	after, err := h.svc.JWKS(context.Background(), h.project)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(after, &set))
	require.Len(t, set.Keys, 1, "the retired key leaves the set")
	assert.Equal(t, next.KID, set.Keys[0].Kid)

	empty, err := h.svc.JWKS(context.Background(), uuid.New())
	require.NoError(t, err)
	assert.JSONEq(t, `{"keys":[]}`, string(empty))
}

func TestList_NewestFirst(t *testing.T) {
	h := newHarness(t)
	first := h.generate(t)
	h.now = h.now.Add(time.Minute)
	next, err := h.svc.Rotate(context.Background(), h.org, h.project, false)
	require.NoError(t, err)
	keys, err := h.svc.List(context.Background(), h.org, h.project)
	require.NoError(t, err)
	require.Len(t, keys, 2)
	assert.Equal(t, next.ID, keys[0].ID)
	assert.Equal(t, first.ID, keys[1].ID)
}

func TestService_ReportsStoreFailures(t *testing.T) {
	for _, op := range []string{"live", "insert", "update", "list", "published", "active", "retiring", "delete"} {
		t.Run(op, func(t *testing.T) {
			h := newHarness(t)
			h.generate(t)
			_, err := h.svc.Rotate(context.Background(), h.org, h.project, true)
			require.NoError(t, err)
			h.now = h.now.Add(8 * 24 * time.Hour)
			h.store.failOn = op
			ctx := context.Background()
			_, errRotate := h.svc.Rotate(ctx, h.org, h.project, true)
			_, errPromote := h.svc.Promote(ctx, h.org, h.project)
			_, errPrune := h.svc.Prune(ctx, h.org, h.project)
			_, errList := h.svc.List(ctx, h.org, h.project)
			_, errJWKS := h.svc.JWKS(ctx, uuid.New())
			_, errActive := h.svc.ActiveKey(ctx, uuid.New())
			failed := 0
			for _, err := range []error{errRotate, errPromote, errPrune, errList, errJWKS, errActive} {
				if err != nil {
					assert.Contains(t, []apperror.Code{apperror.CodeUnexpected, apperror.CodeSigningKeyNoActive}, codeOf(err), "%v", err)
					failed++
				}
			}
			assert.Positive(t, failed, "some call reads %s", op)
		})
	}
}
