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

type countingProjects struct {
	fakeProjects
	passes int
	err    error
}

func (c *countingProjects) Each(ctx context.Context, fn func(ctx context.Context, org, project uuid.UUID) error) error {
	c.passes++
	if c.err != nil {
		return c.err
	}
	return c.fakeProjects.Each(ctx, fn)
}

func TestScheduler_PromoteSkipsTheDatabaseUntilAKeyIsDue(t *testing.T) {
	h := newHarness(t)
	h.generate(t)
	projects := &countingProjects{fakeProjects: fakeProjects{org: h.org, project: h.project}}
	promote := NewScheduler(h.svc, projects, slog.New(slog.DiscardHandler)).SchedulerJobs()[0]
	run := func() { require.NoError(t, promote.Run(context.Background())) }

	run()
	assert.Equal(t, 1, projects.passes, "the first run loads what is pending")
	h.now = h.now.Add(time.Minute)
	run()
	assert.Equal(t, 1, projects.passes, "nothing pending: no query")

	_, err := h.svc.Rotate(context.Background(), h.org, h.project, false)
	require.NoError(t, err)
	h.now = h.now.Add(9 * time.Minute)
	run()
	assert.Equal(t, 1, projects.passes, "the next key is not due yet")
	h.now = h.now.Add(time.Minute)
	run()
	assert.Equal(t, 2, projects.passes)
	assert.Equal(t, map[Status]int{StatusActive: 1, StatusRetiring: 1}, h.store.status(h.project), "promoted at activates_at")

	h.now = h.now.Add(59 * time.Minute)
	run()
	assert.Equal(t, 2, projects.passes)
	h.now = h.now.Add(time.Minute)
	run()
	assert.Equal(t, 3, projects.passes, "an hourly pass catches keys rotated by another process")
}

func TestScheduler_PromoteRetriesAFailedPassOnTheNextRun(t *testing.T) {
	h := newHarness(t)
	h.generate(t)
	projects := &countingProjects{fakeProjects: fakeProjects{org: h.org, project: h.project}}
	promote := NewScheduler(h.svc, projects, slog.New(slog.DiscardHandler)).SchedulerJobs()[0]

	h.store.failOn = "live"
	require.Error(t, promote.Run(context.Background()))
	h.store.failOn = ""
	h.now = h.now.Add(time.Minute)
	require.NoError(t, promote.Run(context.Background()))
	assert.Equal(t, 2, projects.passes)
}

func TestScheduler_PromoteLearnsAKeyPendingAtStartup(t *testing.T) {
	h := newHarness(t)
	h.generate(t)
	_, err := h.svc.Rotate(context.Background(), h.org, h.project, false)
	require.NoError(t, err)
	restarted := NewService(h.store, h.svc.uow, h.svc.sealer, h.svc.cfg, h.svc.unexpected)
	restarted.now = func() time.Time { return h.now }
	projects := &countingProjects{fakeProjects: fakeProjects{org: h.org, project: h.project}}
	promote := NewScheduler(restarted, projects, slog.New(slog.DiscardHandler)).SchedulerJobs()[0]

	require.NoError(t, promote.Run(context.Background()))
	h.now = h.now.Add(10 * time.Minute)
	require.NoError(t, promote.Run(context.Background()))
	assert.Equal(t, 2, projects.passes)
	assert.Equal(t, map[Status]int{StatusActive: 1, StatusRetiring: 1}, h.store.status(h.project))
}

func TestScheduler_PromoteRetriesWhenTheProjectListFails(t *testing.T) {
	h := newHarness(t)
	h.generate(t)
	_, err := h.svc.Rotate(context.Background(), h.org, h.project, false)
	require.NoError(t, err)
	projects := &countingProjects{fakeProjects: fakeProjects{org: h.org, project: h.project}}
	promote := NewScheduler(h.svc, projects, slog.New(slog.DiscardHandler)).SchedulerJobs()[0]

	h.now = h.now.Add(10 * time.Minute)
	projects.err = errFake
	require.Error(t, promote.Run(context.Background()))
	projects.err = nil
	h.now = h.now.Add(time.Minute)
	require.NoError(t, promote.Run(context.Background()))
	assert.Equal(t, 2, projects.passes, "a failed pass must not forget the due key until the hourly check")
	assert.Equal(t, map[Status]int{StatusActive: 1, StatusRetiring: 1}, h.store.status(h.project))
}

func TestScheduler_PromoteFallbackIsPrePublishWhenRotationsHappenElsewhere(t *testing.T) {
	h := newHarness(t)
	h.generate(t)
	h.svc.WithRemoteRotations()
	projects := &countingProjects{fakeProjects: fakeProjects{org: h.org, project: h.project}}
	promote := NewScheduler(h.svc, projects, slog.New(slog.DiscardHandler)).SchedulerJobs()[0]

	require.NoError(t, promote.Run(context.Background()))
	h.now = h.now.Add(9 * time.Minute)
	require.NoError(t, promote.Run(context.Background()))
	assert.Equal(t, 1, projects.passes)
	h.now = h.now.Add(time.Minute)
	require.NoError(t, promote.Run(context.Background()))
	assert.Equal(t, 2, projects.passes, "a key rotated by another process is promoted within rotation.prePublish")
}
