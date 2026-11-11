//go:build integration

package mailqueue_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/altalune-id/othra/internal/kernel/db"
	"github.com/altalune-id/othra/internal/kernel/mailqueue"
	"github.com/altalune-id/othra/internal/testutil/pgtest"
	"github.com/altalune-id/othra/sealer"
)

type countingClaims struct {
	mailqueue.WorkerStore
	n atomic.Int32
}

func (c *countingClaims) ClaimDue(ctx context.Context, now time.Time, limit int) ([]mailqueue.Entry, error) {
	c.n.Add(1)
	return c.WorkerStore.ClaimDue(ctx, now, limit)
}

func TestWake_SendsACommittedEnqueueBeforeTheIdlePoll(t *testing.T) {
	sqlDB, schemaName := pgtest.Migrated(t)
	key, err := sealer.GenerateKey()
	require.NoError(t, err)
	sl, err := sealer.New(key)
	require.NoError(t, err)
	pool := db.Pool{W: sqlDB, R: sqlDB}
	store := mailqueue.NewStore(pool, schemaName, "", sl, false)
	claims := &countingClaims{WorkerStore: store}
	m := &recordingMailer{}
	w := mailqueue.NewWorker(claims, m, nil, mailqueue.WorkerOpts{Tick: 10 * time.Millisecond, IdlePoll: time.Hour})
	store.OnEnqueue(w.Wake)

	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() { done <- w.Run(ctx) }()
	t.Cleanup(func() {
		cancel()
		require.NoError(t, <-done)
	})
	require.Eventually(t, func() bool { return claims.n.Load() >= 1 }, 10*time.Second, 5*time.Millisecond)

	boom := errors.New("boom")
	_ = db.RunInTx(t.Context(), pool, func(ctx context.Context) error {
		_, err := store.Enqueue(ctx, message())
		require.NoError(t, err)
		return boom
	})
	require.NoError(t, db.RunInTx(t.Context(), pool, func(ctx context.Context) error {
		_, err := store.Enqueue(ctx, message())
		return err
	}))

	require.Eventually(t, func() bool { return m.count() == 1 }, 5*time.Second, 10*time.Millisecond,
		"a committed enqueue must wake the worker instead of waiting an hour")
	assert.EqualValues(t, 2, claims.n.Load(), "only the commit wakes the worker, not the rolled-back enqueue")
}
