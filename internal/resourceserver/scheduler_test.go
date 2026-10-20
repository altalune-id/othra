package resourceserver

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
	err          error
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
	return f.err
}

func TestScheduler_RechecksEveryProject(t *testing.T) {
	h := newHarness(t, false)
	r := h.create(t, "https://api.example.com")
	jobs := NewScheduler(h.svc, fakeProjects{org: h.org, project: h.project, extra: 2}, slog.New(slog.DiscardHandler)).SchedulerJobs()
	require.Len(t, jobs, 1)
	job := jobs[0]
	assert.Equal(t, "resource-servers-verify", job.Name)
	assert.True(t, job.Singleton)
	assert.Equal(t, verifyJobTimeout, job.Timeout)

	require.NoError(t, job.Run(context.Background()))
	assert.Equal(t, StateVerified, h.store.server(r.ID).State)

	h.now = h.now.Add(25 * time.Hour)
	h.store.failOn = "due"
	assert.Error(t, job.Run(context.Background()), "a failing project is reported, the others still run")

	h.store.failOn = ""
	failing := NewScheduler(h.svc, fakeProjects{org: h.org, project: h.project, err: errFake}, slog.New(slog.DiscardHandler)).SchedulerJobs()[0]
	assert.ErrorIs(t, failing.Run(context.Background()), errFake)
}
