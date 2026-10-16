package apikey_test

import (
	"context"
	"maps"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"altalune.id/othra/internal/apikey"
	"altalune.id/othra/internal/testutil/fakes"
)

func TestUsageWorker_KeepsLatestAndFlushesOnTick(t *testing.T) {
	store := &fakes.APIKeyStore{}
	w := apikey.NewUsageWorker(store, 10*time.Millisecond, nil)
	id := uuid.New()
	t0 := time.Date(2026, 10, 11, 10, 0, 0, 0, time.UTC)
	org := uuid.New()
	w.Record(org, id, t0.Add(time.Minute))
	w.Record(org, id, t0)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- w.Run(ctx) }()
	require.Eventually(t, func() bool { return len(store.Batches()) == 1 }, time.Second, 5*time.Millisecond)
	cancel()
	require.ErrorIs(t, <-done, context.Canceled)

	batches := store.Batches()
	require.Len(t, batches, 1)
	assert.True(t, t0.Add(time.Minute).Equal(batches[0][id]))
}

func TestUsageWorker_DrainsOnShutdown(t *testing.T) {
	store := &fakes.APIKeyStore{}
	w := apikey.NewUsageWorker(store, time.Hour, nil)
	id := uuid.New()
	w.Record(uuid.New(), id, time.Now())

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	require.ErrorIs(t, w.Run(ctx), context.Canceled)
	require.Len(t, store.Batches(), 1)
	assert.Contains(t, store.Batches()[0], id)
}

func TestUsageWorker_NothingPendingWritesNothing(t *testing.T) {
	store := &fakes.APIKeyStore{}
	w := apikey.NewUsageWorker(store, time.Hour, nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_ = w.Run(ctx)
	assert.Empty(t, store.Batches())
	assert.Equal(t, "apikey-usage", w.Name())
}

// SECURITY: api_keys is org-scoped under row level security, so each org's keys are flushed in that org's scope.
func TestUsageWorker_FlushesEachOrgInItsOwnScope(t *testing.T) {
	store := &fakes.APIKeyStore{}
	w := apikey.NewUsageWorker(store, time.Hour, nil)
	orgA, orgB := uuid.New(), uuid.New()
	keyA, keyB := uuid.New(), uuid.New()
	w.Record(orgA, keyA, time.Now())
	w.Record(orgB, keyB, time.Now())

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	require.ErrorIs(t, w.Run(ctx), context.Canceled)
	require.Len(t, store.Batches(), 2)
	for i, org := range store.TouchedOrgs {
		want := map[uuid.UUID]uuid.UUID{orgA: keyA, orgB: keyB}[org]
		assert.Equal(t, []uuid.UUID{want}, slices.Collect(maps.Keys(store.Batches()[i])), "org %s", org)
	}
	assert.ElementsMatch(t, []uuid.UUID{orgA, orgB}, store.TouchedOrgs)
}
