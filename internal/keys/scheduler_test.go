package keys

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeProjects struct {
	org, project uuid.UUID
	extra        int
}

func (f fakeProjects) Each(ctx context.Context, fn func(ctx context.Context, org, project uuid.UUID) error) error {
	if err := fn(ctx, f.org, f.project); err != nil {
		return err
	}
	for range f.extra {
		if err := fn(ctx, f.org, uuid.New()); err != nil {
			return err
		}
	}
	return nil
}

func TestScheduler_PromotesAndPrunesEveryProject(t *testing.T) {
	h := newHarness(t)
	h.generate(t)
	_, err := h.svc.Rotate(context.Background(), h.org, h.project, false)
	require.NoError(t, err)
	jobs := NewScheduler(h.svc, fakeProjects{org: h.org, project: h.project, extra: 2}, slog.New(slog.DiscardHandler)).SchedulerJobs()
	require.Len(t, jobs, 2)
	promote, prune := jobs[0], jobs[1]
	assert.Equal(t, "keys-promote", promote.Name)
	assert.Equal(t, "keys-prune", prune.Name)
	assert.True(t, promote.Singleton)

	require.NoError(t, promote.Run(context.Background()))
	assert.Equal(t, map[Status]int{StatusActive: 1, StatusNext: 1}, h.store.status(h.project), "too early")

	h.now = h.now.Add(10 * time.Minute)
	require.NoError(t, promote.Run(context.Background()))
	assert.Equal(t, map[Status]int{StatusActive: 1, StatusRetiring: 1}, h.store.status(h.project))

	h.now = h.now.Add(7 * 24 * time.Hour)
	require.NoError(t, prune.Run(context.Background()))
	assert.Equal(t, map[Status]int{StatusActive: 1, StatusRetired: 1}, h.store.status(h.project))

	h.store.failOn = "live"
	assert.ErrorIs(t, promote.Run(context.Background()), errFake, "a failing project is reported, the others still run")
}
