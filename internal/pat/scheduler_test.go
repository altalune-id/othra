package pat

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
	ctx := context.Background()
	old := h.create(t)
	require.NoError(t, h.svc.Revoke(ctx, h.p, h.user, old.Token.PublicID))
	h.now = h.now.Add(31 * 24 * time.Hour)

	var logs bytes.Buffer
	jobs := NewScheduler(h.svc, projectList{h.p, {OrgID: uuid.New(), ID: uuid.New()}}, slog.New(slog.NewJSONHandler(&logs, nil))).SchedulerJobs()
	require.Len(t, jobs, 1)
	job := jobs[0]
	assert.Equal(t, "pat-prune", job.Name)
	assert.Equal(t, scheduler.ScopeSystem, job.Scope)
	assert.True(t, job.Singleton)

	require.NoError(t, job.Run(ctx))
	assert.Contains(t, logs.String(), `"deleted":1`)

	h.store.err = errBoom
	assert.Error(t, job.Run(ctx))
}
