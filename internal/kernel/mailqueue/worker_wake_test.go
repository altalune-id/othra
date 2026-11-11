package mailqueue

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/altalune-id/othra/mailer"
)

type countingStore struct {
	*fakeStore
	claims    atomic.Int32
	last      atomic.Int64
	claimErrs atomic.Int32
}

func (c *countingStore) ClaimDue(ctx context.Context, now time.Time, limit int) ([]Entry, error) {
	c.claims.Add(1)
	c.last.Store(time.Now().UnixNano())
	if c.claimErrs.Add(-1) >= 0 {
		return nil, errors.New("cold start")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.fakeStore.ClaimDue(ctx, now, limit)
}

func (c *countingStore) lastClaim() time.Time { return time.Unix(0, c.last.Load()) }

func runWorker(t *testing.T, w *Worker) {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() { done <- w.Run(ctx) }()
	t.Cleanup(func() {
		cancel()
		assert.NoError(t, <-done)
	})
}

func TestWorkerRunSweepsAtStartup(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		store := &countingStore{fakeStore: &fakeStore{}}
		runWorker(t, NewWorker(store, &fakeMailer{}, nil, WorkerOpts{IdlePoll: time.Hour}))
		synctest.Wait()
		assert.EqualValues(t, 1, store.claims.Load())
	})
}

func TestWorkerWakeSweepsBeforeTheIdlePoll(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		store := &countingStore{fakeStore: &fakeStore{}}
		w := NewWorker(store, &fakeMailer{}, nil, WorkerOpts{IdlePoll: time.Hour})
		runWorker(t, w)
		synctest.Wait()
		w.Wake()
		synctest.Wait()
		assert.EqualValues(t, 2, store.claims.Load(), "a wake must sweep at once, not after the idle poll")
	})
}

func TestWorkerWakeNeverBlocks(t *testing.T) {
	w := NewWorker(&fakeStore{}, &fakeMailer{}, nil, WorkerOpts{})
	w.Wake()
	w.Wake()
}

func TestWorkerIdlePollDefaultsToTheTick(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		store := &countingStore{fakeStore: &fakeStore{}}
		runWorker(t, NewWorker(store, &fakeMailer{}, nil, WorkerOpts{}))
		synctest.Wait()
		time.Sleep(defaultTick)
		synctest.Wait()
		assert.EqualValues(t, 2, store.claims.Load())
	})
}

func TestWorkerRetriesAFailedSendBeforeTheIdlePoll(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		store := &countingStore{fakeStore: &fakeStore{claimed: []Entry{claimed(1)}, msg: testMessage()}}
		runWorker(t, NewWorker(store, &fakeMailer{err: errors.New("down")}, nil, WorkerOpts{IdlePoll: time.Hour}))
		synctest.Wait()
		assert.EqualValues(t, 1, store.claims.Load())
		time.Sleep(backoffBase(2) + backoffBase(2)/10 + time.Second)
		synctest.Wait()
		assert.EqualValues(t, 2, store.claims.Load(), "the retry is due long before the idle poll")
	})
}

func TestWorkerRetriesAFailedClaimAfterATick(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		store := &countingStore{fakeStore: &fakeStore{}}
		store.claimErrs.Store(1)
		runWorker(t, NewWorker(store, &fakeMailer{}, nil, WorkerOpts{IdlePoll: time.Hour}))
		synctest.Wait()
		time.Sleep(defaultTick)
		synctest.Wait()
		assert.EqualValues(t, 2, store.claims.Load(), "a failed claim is retried after a tick, not the idle poll")
	})
}

func TestWorkerRetriesAtTheLeaseWhenASettleFails(t *testing.T) {
	for name, tc := range map[string]struct {
		attempt int
		sendErr error
	}{
		"succeed": {attempt: 1},
		"fail":    {attempt: 1, sendErr: errors.New("down")},
		"give up": {attempt: MaxAttempts, sendErr: errors.New("down")},
	} {
		t.Run(name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				start := time.Now()
				store := &countingStore{fakeStore: &fakeStore{claimed: []Entry{claimed(tc.attempt)}, msg: testMessage(), settleErr: errors.New("lost")}}
				runWorker(t, NewWorker(store, &fakeMailer{err: tc.sendErr}, nil, WorkerOpts{IdlePoll: time.Hour}))
				synctest.Wait()
				time.Sleep(ClaimLease + defaultTick)
				synctest.Wait()
				assert.False(t, store.lastClaim().Before(start.Add(ClaimLease)), "the entry is claimable again once its lease ends")
			})
		})
	}
}

func TestWorkerTracksEveryRetryNotOnlyTheEarliest(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		store := &countingStore{fakeStore: &fakeStore{claimed: []Entry{claimed(1), claimed(3)}, msg: testMessage()}}
		runWorker(t, NewWorker(store, &fakeMailer{err: errors.New("down")}, nil, WorkerOpts{IdlePoll: 2 * time.Hour}))
		synctest.Wait()
		time.Sleep(backoffBase(4) + backoffBase(4)/10 + time.Second)
		synctest.Wait()
		assert.EqualValues(t, 3, store.claims.Load(), "the later retry is swept at its own time, not after the idle poll")
	})
}

type slowSecondMailer struct{ calls atomic.Int32 }

func (m *slowSecondMailer) Send(context.Context, mailer.Message) error {
	if m.calls.Add(1) == 1 {
		return errors.New("down")
	}
	time.Sleep(time.Minute)
	return nil
}

func TestWorkerKeepsARetryThatFallsDueDuringASweep(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		store := &countingStore{fakeStore: &fakeStore{claimed: []Entry{claimed(1), claimed(1)}, msg: testMessage()}}
		runWorker(t, NewWorker(store, &slowSecondMailer{}, nil, WorkerOpts{IdlePoll: time.Hour}))
		time.Sleep(time.Minute + 2*defaultTick)
		synctest.Wait()
		assert.EqualValues(t, 2, store.claims.Load(), "a retry due while the sweep ran is swept a tick later, not after the idle poll")
	})
}
