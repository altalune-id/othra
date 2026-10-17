package storage

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"altalune.id/othra/apperror"
	"altalune.id/othra/scheduler"
)

type fakeTenants struct{ n int }

func (f fakeTenants) Each(ctx context.Context, fn func(context.Context, string) error) error {
	for i := range f.n {
		if err := fn(ctx, string(rune('a'+i))); err != nil {
			return err
		}
	}
	return nil
}

func TestSweep_RemovesDeletingAndExpiredUploadsOnly(t *testing.T) {
	h := newHarness(t)
	ready := h.begin(t)
	require.NoError(t, h.svc.MarkReady(context.Background(), ready.ID))
	fresh := h.begin(t)
	expired := h.begin(t)
	replaced := h.begin(t)
	for _, f := range []*File{ready, fresh, expired, replaced} {
		h.driver.put(f.ObjectKey, "image/png", pngBody)
	}
	require.NoError(t, h.svc.MarkDeleting(context.Background(), replaced.ID))
	h.store.files[expired.ID].ExpiresAt = h.now.Add(-time.Second)

	n, err := h.svc.Sweep(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 2, n)
	assert.Nil(t, h.store.get(expired.ID))
	assert.Nil(t, h.store.get(replaced.ID))
	assert.False(t, h.driver.has(expired.ObjectKey))
	assert.False(t, h.driver.has(replaced.ObjectKey))
	assert.NotNil(t, h.store.get(ready.ID))
	assert.NotNil(t, h.store.get(fresh.ID))
	assert.True(t, h.driver.has(ready.ObjectKey))
}

func TestSweep_KeepsTheRowWhenTheObjectCannotBeDeleted(t *testing.T) {
	h := newHarness(t)
	f := h.begin(t)
	require.NoError(t, h.svc.MarkDeleting(context.Background(), f.ID))
	h.driver.failOn = "delete"
	n, err := h.svc.Sweep(context.Background())
	assert.Zero(t, n)
	require.ErrorIs(t, err, errFake)
	assert.NotNil(t, h.store.get(f.ID), "retried on the next run")

	h.driver.failOn, h.store.failOn = "", "delete"
	_, err = h.svc.Sweep(context.Background())
	require.ErrorIs(t, err, errFake)

	h.store.failOn = "sweepable"
	_, err = h.svc.Sweep(context.Background())
	require.ErrorIs(t, err, errFake)
}

func TestScheduler_DisabledHasNoJob(t *testing.T) {
	svc := NewService(nil, newFakeStore(), Config{}, apperror.NewReporter(nil, false).Unexpected)
	assert.Empty(t, NewScheduler(svc, fakeTenants{}, slog.New(slog.DiscardHandler)).SchedulerJobs())
}

func TestScheduler_SweepsEveryOrgAndKeepsGoingAfterAFailure(t *testing.T) {
	h := newHarness(t)
	f := h.begin(t)
	require.NoError(t, h.svc.MarkDeleting(context.Background(), f.ID))
	jobs := NewScheduler(h.svc, fakeTenants{n: 2}, slog.New(slog.DiscardHandler)).SchedulerJobs()
	require.Len(t, jobs, 1)
	j := jobs[0]
	assert.Equal(t, "storage-sweep", j.Name)
	assert.Equal(t, scheduler.ScopeSystem, j.Scope)
	assert.True(t, j.Singleton)

	require.NoError(t, j.Run(context.Background()))
	assert.Nil(t, h.store.get(f.ID))

	h.store.failOn = "sweepable"
	err := j.Run(context.Background())
	require.ErrorIs(t, err, errFake)
}
