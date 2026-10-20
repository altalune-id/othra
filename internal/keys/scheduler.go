package keys

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"altalune.id/othra/scheduler"
)

const (
	promoteEvery   = time.Minute
	promoteJitter  = 5 * time.Second
	pruneEvery     = time.Hour
	pruneJitter    = 5 * time.Minute
	jobTimeout     = 2 * time.Minute
	promoteJobName = "keys-promote"
	pruneJobName   = "keys-prune"
)

// Scheduler adapts the Service to scheduler.Provider.
type Scheduler struct {
	svc      *Service
	projects Projects
	log      *slog.Logger
}

// NewScheduler binds svc to the promote and prune jobs, run once per project.
func NewScheduler(svc *Service, projects Projects, log *slog.Logger) *Scheduler {
	return &Scheduler{svc: svc, projects: projects, log: log.With("module", "keys")}
}

// SchedulerJobs implements scheduler.Provider.
func (a *Scheduler) SchedulerJobs() []scheduler.Job {
	return []scheduler.Job{
		a.job(promoteJobName, scheduler.MustEveryInterval(promoteEvery, promoteJitter), func(
			ctx context.Context, org, project uuid.UUID,
		) (int, error) {
			ok, err := a.svc.Promote(ctx, org, project)
			if ok {
				return 1, err
			}
			return 0, err
		}),
		a.job(pruneJobName, scheduler.MustEveryInterval(pruneEvery, pruneJitter), a.svc.Prune),
	}
}

func (a *Scheduler) job(
	name string, every scheduler.Schedule, run func(ctx context.Context, org, project uuid.UUID) (int, error),
) scheduler.Job {
	return scheduler.Job{
		Name:      name,
		Scope:     scheduler.ScopeSystem,
		Schedule:  every,
		Timeout:   jobTimeout,
		Singleton: true,
		Run: func(ctx context.Context) error {
			total := 0
			var errs []error
			err := a.projects.Each(ctx, func(ctx context.Context, org, project uuid.UUID) error {
				n, err := run(ctx, org, project)
				total += n
				if err != nil {
					errs = append(errs, err)
				}
				return nil
			})
			if total > 0 {
				a.log.LogAttrs(ctx, slog.LevelInfo, name, slog.Int("changed", total))
			}
			return errors.Join(append(errs, err)...)
		},
	}
}
