package outbox

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

type queuedStore struct {
	fakeStore
	mu        sync.Mutex
	batches   [][]Entry
	claimErrs int
	settleErr error
}

func (q *queuedStore) ClaimDue(context.Context, time.Time, int) ([]Entry, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.claimErrs > 0 {
		q.claimErrs--
		return nil, errors.New("cold start")
	}
	if len(q.batches) == 0 {
		return nil, nil
	}
	b := q.batches[0]
	q.batches = q.batches[1:]
	return b, nil
}

func (q *queuedStore) Fail(context.Context, Entry, time.Time, string) error { return q.settleErr }

func (q *queuedStore) Succeed(context.Context, Entry, time.Time) error { return q.settleErr }

type countingTenants struct {
	sweeps atomic.Int32
	last   atomic.Int64
	errs   atomic.Int32
}

func (c *countingTenants) Each(ctx context.Context, fn func(context.Context, string) error) error {
	c.sweeps.Add(1)
	c.last.Store(time.Now().UnixNano())
	if c.errs.Add(-1) >= 0 {
		return errors.New("cold start")
	}
	return fn(ctx, "org")
}

func (c *countingTenants) lastSweep() time.Time { return time.Unix(0, c.last.Load()) }

type downDeliverer struct{}

func (downDeliverer) Deliver(context.Context, Entry) error { return errors.New("down") }

func runWorker(t *testing.T, w *Worker) context.CancelFunc {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() { done <- w.Run(ctx) }()
	t.Cleanup(func() {
		cancel()
		assert.NoError(t, <-done)
	})
	return cancel
}

func TestWorkerRunSweepsAtStartup(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		tenants := &countingTenants{}
		runWorker(t, NewWorker(&queuedStore{}, &fakeDeliverer{}, tenants, nil, WorkerOpts{IdlePoll: time.Hour}))
		synctest.Wait()
		assert.EqualValues(t, 1, tenants.sweeps.Load())
	})
}

func TestWorkerWakeSweepsBeforeTheIdlePoll(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		tenants := &countingTenants{}
		w := NewWorker(&queuedStore{}, &fakeDeliverer{}, tenants, nil, WorkerOpts{IdlePoll: time.Hour})
		runWorker(t, w)
		synctest.Wait()
		w.Wake()
		synctest.Wait()
		assert.EqualValues(t, 2, tenants.sweeps.Load(), "a wake must sweep at once, not after the idle poll")
	})
}

func TestWorkerWakeNeverBlocks(t *testing.T) {
	w := NewWorker(&queuedStore{}, &fakeDeliverer{}, &fakeTenants{}, nil, WorkerOpts{})
	w.Wake()
	w.Wake()
}

func TestWorkerIdlePollDefaultsToTheTick(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		tenants := &countingTenants{}
		runWorker(t, NewWorker(&queuedStore{}, &fakeDeliverer{}, tenants, nil, WorkerOpts{}))
		synctest.Wait()
		time.Sleep(defaultTick)
		synctest.Wait()
		assert.EqualValues(t, 2, tenants.sweeps.Load())
	})
}

func TestWorkerRetriesAFailedEntryBeforeTheIdlePoll(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		tenants := &countingTenants{}
		store := &queuedStore{batches: [][]Entry{{claimedEntry()}}}
		runWorker(t, NewWorker(store, &fakeDeliverer{err: errors.New("down")}, tenants, nil, WorkerOpts{IdlePoll: time.Hour}))
		synctest.Wait()
		assert.EqualValues(t, 1, tenants.sweeps.Load())
		time.Sleep(BackoffBase(1) + BackoffBase(1)/10 + time.Second)
		synctest.Wait()
		assert.EqualValues(t, 2, tenants.sweeps.Load(), "the retry is due long before the idle poll")
	})
}

func TestWorkerWaitsATickAfterAFullBatch(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		tenants := &countingTenants{}
		store := &queuedStore{batches: [][]Entry{{claimedEntry()}, {claimedEntry()}}}
		runWorker(t, NewWorker(store, &fakeDeliverer{}, tenants, nil, WorkerOpts{Batch: 1, IdlePoll: time.Hour}))
		synctest.Wait()
		assert.EqualValues(t, 1, tenants.sweeps.Load(), "a full batch must not re-sweep at once")
		time.Sleep(defaultTick)
		synctest.Wait()
		assert.EqualValues(t, 2, tenants.sweeps.Load(), "a full batch sweeps again after one tick")
	})
}

func TestWorkerRetriesAFailedClaimAfterATick(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		tenants := &countingTenants{}
		store := &queuedStore{claimErrs: 1}
		runWorker(t, NewWorker(store, &fakeDeliverer{}, tenants, nil, WorkerOpts{IdlePoll: time.Hour}))
		synctest.Wait()
		time.Sleep(defaultTick)
		synctest.Wait()
		assert.EqualValues(t, 2, tenants.sweeps.Load(), "a failed claim is retried after a tick, not the idle poll")
	})
}

func TestWorkerRetriesAFailedTenantListAfterATick(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		tenants := &countingTenants{}
		tenants.errs.Store(1)
		runWorker(t, NewWorker(&queuedStore{}, &fakeDeliverer{}, tenants, nil, WorkerOpts{IdlePoll: time.Hour}))
		synctest.Wait()
		time.Sleep(defaultTick)
		synctest.Wait()
		assert.EqualValues(t, 2, tenants.sweeps.Load())
	})
}

func TestWorkerRetriesAtTheLeaseWhenASettleFails(t *testing.T) {
	for name, deliverErr := range map[string]error{"succeed": nil, "fail": errors.New("down")} {
		t.Run(name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				start := time.Now()
				tenants := &countingTenants{}
				store := &queuedStore{batches: [][]Entry{{claimedEntry()}}, settleErr: errors.New("lost")}
				runWorker(t, NewWorker(store, &fakeDeliverer{err: deliverErr}, tenants, nil, WorkerOpts{IdlePoll: time.Hour}))
				synctest.Wait()
				time.Sleep(ClaimLease + defaultTick)
				synctest.Wait()
				assert.False(t, tenants.lastSweep().Before(start.Add(ClaimLease)), "the entry is claimable again once its lease ends")
			})
		})
	}
}

func TestWorkerTracksEveryRetryNotOnlyTheEarliest(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		tenants := &countingTenants{}
		soon, later := claimedEntry(), claimedEntry()
		soon.Attempt, later.Attempt = 1, 3
		store := &queuedStore{batches: [][]Entry{{soon, later}}}
		runWorker(t, NewWorker(store, downDeliverer{}, tenants, nil, WorkerOpts{IdlePoll: 2 * time.Hour}))
		synctest.Wait()
		time.Sleep(BackoffBase(4) + BackoffBase(4)/10 + time.Second)
		synctest.Wait()
		assert.EqualValues(t, 3, tenants.sweeps.Load(), "the later retry is swept at its own time, not after the idle poll")
	})
}

type slowDeliverer struct{ slow uuid.UUID }

func (s slowDeliverer) Deliver(_ context.Context, e Entry) error {
	if e.ID == s.slow {
		time.Sleep(time.Minute)
		return nil
	}
	return errors.New("down")
}

func TestWorkerKeepsARetryThatFallsDueDuringASweep(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		tenants := &countingTenants{}
		failing, slow := claimedEntry(), claimedEntry()
		failing.Attempt = 1
		store := &queuedStore{batches: [][]Entry{{failing, slow}}}
		runWorker(t, NewWorker(store, slowDeliverer{slow: slow.ID}, tenants, nil, WorkerOpts{IdlePoll: time.Hour}))
		time.Sleep(time.Minute + 2*defaultTick)
		synctest.Wait()
		assert.EqualValues(t, 2, tenants.sweeps.Load(), "a retry due while the sweep ran is swept a tick later, not after the idle poll")
	})
}
