//go:build integration

package outbox_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/altalune-id/othra/internal/kernel/db"
	"github.com/altalune-id/othra/internal/kernel/outbox"
	"github.com/altalune-id/othra/internal/kernel/tenant"
	"github.com/altalune-id/othra/internal/testutil/pgtest"
)

type oneTenant struct {
	tc     tenant.Context
	sweeps atomic.Int32
}

func (o *oneTenant) Each(ctx context.Context, fn func(context.Context, string) error) error {
	o.sweeps.Add(1)
	return fn(tenant.Into(ctx, tenant.Context{OrgID: o.tc.OrgID}), o.tc.OrgID.String())
}

type countingDeliverer struct{ n atomic.Int32 }

func (c *countingDeliverer) Deliver(context.Context, outbox.Entry) error {
	c.n.Add(1)
	return nil
}

func TestWake_DeliversACommittedEnqueueBeforeTheIdlePoll(t *testing.T) {
	pool, sqlDB, schemaName := pgtest.MigratedRLS(t)
	f := &fixture{db: sqlDB, pool: pool}
	ctx, tc := f.seedTenant(t)
	store := outbox.NewStore(pool, schemaName, "")
	tenants, deliver := &oneTenant{tc: tc}, &countingDeliverer{}
	w := outbox.NewWorker(store, deliver, tenants, nil, outbox.WorkerOpts{Tick: 10 * time.Millisecond, IdlePoll: time.Hour})
	store.OnEnqueue(w.Wake)

	runCtx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() { done <- w.Run(runCtx) }()
	t.Cleanup(func() {
		cancel()
		require.NoError(t, <-done)
	})
	require.Eventually(t, func() bool { return tenants.sweeps.Load() >= 1 }, 10*time.Second, 5*time.Millisecond)

	boom := errors.New("boom")
	_ = db.RunInTx(ctx, pool, func(ctx context.Context) error {
		require.NoError(t, store.Enqueue(ctx, newEntry(tc, "t")))
		return boom
	})
	require.NoError(t, db.RunInTx(ctx, pool, func(ctx context.Context) error {
		return store.Enqueue(ctx, newEntry(tc, "t"))
	}))

	require.Eventually(t, func() bool { return deliver.n.Load() == 1 }, 5*time.Second, 10*time.Millisecond,
		"a committed enqueue must wake the worker instead of waiting an hour")
	assert.EqualValues(t, 2, tenants.sweeps.Load(), "only the commit wakes the worker, not the rolled-back enqueue")
}
