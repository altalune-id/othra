package client

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/altalune-id/othra/scheduler"
)

const (
	pruneEvery   = time.Hour
	pruneJitter  = 5 * time.Minute
	pruneTimeout = 5 * time.Minute
	pruneJobName = "client-prune-shadows"
)

// Scheduler adapts the Service to scheduler.Provider.
type Scheduler struct {
	svc      *Service
	projects Projects
	log      *slog.Logger
}

// NewScheduler binds svc to the shadow prune job, run once per project.
func NewScheduler(svc *Service, projects Projects, log *slog.Logger) *Scheduler {
	return &Scheduler{svc: svc, projects: projects, log: log.With("module", "client")}
}

// SchedulerJobs implements scheduler.Provider.
func (a *Scheduler) SchedulerJobs() []scheduler.Job {
	return []scheduler.Job{{
		Name:      pruneJobName,
		Scope:     scheduler.ScopeSystem,
		Schedule:  scheduler.MustEveryInterval(pruneEvery, pruneJitter),
		Timeout:   pruneTimeout,
		Singleton: true,
		Run: func(ctx context.Context) error {
			total := 0
			var errs []error
			err := a.projects.Each(ctx, func(ctx context.Context, org, project uuid.UUID) error {
				n, err := a.svc.PruneShadows(ctx, org, project)
				total += n
				if err != nil {
					errs = append(errs, err)
				}
				return nil
			})
			if total > 0 {
				a.log.LogAttrs(ctx, slog.LevelInfo, pruneJobName, slog.Int("deleted", total))
			}
			return errors.Join(append(errs, err)...)
		},
	}}
}
