package resourceserver

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/altalune-id/othra/scheduler"
)

const (
	verifyJobName    = "resource-servers-verify"
	verifyJobTimeout = 15 * time.Minute
)

// Scheduler adapts the Service to scheduler.Provider.
type Scheduler struct {
	svc      *Service
	projects Projects
	log      *slog.Logger
}

// NewScheduler binds svc to the re-check job, run once per project.
func NewScheduler(svc *Service, projects Projects, log *slog.Logger) *Scheduler {
	return &Scheduler{svc: svc, projects: projects, log: log.With("module", "resourceserver")}
}

// SchedulerJobs implements scheduler.Provider.
func (a *Scheduler) SchedulerJobs() []scheduler.Job {
	every := a.svc.cfg.Interval
	return []scheduler.Job{{
		Name:      verifyJobName,
		Scope:     scheduler.ScopeSystem,
		Schedule:  scheduler.MustEveryInterval(every, every/12),
		Timeout:   verifyJobTimeout,
		Singleton: true,
		Run: func(ctx context.Context) error {
			total := 0
			var errs []error
			err := a.projects.Each(ctx, func(ctx context.Context, org, project uuid.UUID) error {
				n, err := a.svc.Recheck(ctx, org, project)
				total += n
				if err != nil {
					errs = append(errs, err)
				}
				return nil
			})
			if total > 0 {
				a.log.LogAttrs(ctx, slog.LevelInfo, verifyJobName, slog.Int("changed", total))
			}
			return errors.Join(append(errs, err)...)
		},
	}}
}
