package enduser

import (
	"bytes"
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/altalune-id/othra/scheduler"
)

type projectList []Project

func (p projectList) Each(ctx context.Context, fn func(ctx context.Context, orgID, projectID uuid.UUID) error) error {
	for _, pr := range p {
		if err := fn(ctx, pr.OrgID, pr.ID); err != nil {
			return err
		}
	}
	return nil
}

func TestScheduler_PrunesEveryProject(t *testing.T) {
	h := newHarness(t)
	h.send(t, "ada@example.com")
	var logs bytes.Buffer
	jobs := NewScheduler(h.svc, projectList{h.project, {OrgID: uuid.New(), ID: uuid.New()}}, slog.New(slog.NewJSONHandler(&logs, nil))).SchedulerJobs()
	require.Len(t, jobs, 1)
	job := jobs[0]
	assert.Equal(t, "enduser-prune", job.Name)
	assert.Equal(t, scheduler.ScopeSystem, job.Scope)
	assert.True(t, job.Singleton)

	h.now = h.now.Add(time.Hour)
	require.NoError(t, job.Run(context.Background()))
	assert.Empty(t, h.store.links)
	assert.Contains(t, logs.String(), `"deleted":1`)

	h.store.failOn = "Prune"
	assert.Error(t, job.Run(context.Background()))
}
