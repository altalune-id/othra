package storage

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"altalune.id/othra/scheduler"
)

const (
	sweepEvery   = 5 * time.Minute
	sweepJitter  = 30 * time.Second
	sweepTimeout = 2 * time.Minute
	sweepBatch   = 100
	sweepJobName = "storage-sweep"
)

// Sweep deletes, in the org on ctx, up to 100 objects and rows of files marked deleting or left pending past their expiry.
func (s *Service) Sweep(ctx context.Context) (int, error) {
	if !s.Enabled() {
		return 0, nil
	}
	files, err := s.store.Sweepable(ctx, s.now(), sweepBatch)
	if err != nil {
		return 0, err
	}
	var errs []error
	n := 0
	for _, f := range files {
		if err := s.driver.Delete(ctx, f.ObjectKey); err != nil {
			errs = append(errs, err)
			continue
		}
		if err := s.store.Delete(ctx, f.ID); err != nil {
			errs = append(errs, err)
			continue
		}
		n++
	}
	return n, errors.Join(errs...)
}

// Scheduler adapts the Service to scheduler.Provider.
type Scheduler struct {
	svc     *Service
	tenants Tenants
	log     *slog.Logger
}

// NewScheduler binds svc to the sweep job, run once per org from tenants.
func NewScheduler(svc *Service, tenants Tenants, log *slog.Logger) *Scheduler {
	return &Scheduler{svc: svc, tenants: tenants, log: log.With("module", "storage")}
}

// SchedulerJobs implements scheduler.Provider; a disabled Service has no job.
func (a *Scheduler) SchedulerJobs() []scheduler.Job {
	if !a.svc.Enabled() {
		return nil
	}
	return []scheduler.Job{{
		Name:      sweepJobName,
		Scope:     scheduler.ScopeSystem,
		Schedule:  scheduler.MustEveryInterval(sweepEvery, sweepJitter),
		Timeout:   sweepTimeout,
		Singleton: true,
		Run: func(ctx context.Context) error {
			total := 0
			var errs []error
			err := a.tenants.Each(ctx, func(ctx context.Context, _ string) error {
				n, err := a.svc.Sweep(ctx)
				total += n
				if err != nil {
					errs = append(errs, err)
				}
				return nil
			})
			if total > 0 {
				a.log.LogAttrs(ctx, slog.LevelInfo, "storage.sweep", slog.Int("deleted", total))
			}
			return errors.Join(append(errs, err)...)
		},
	}}
}
