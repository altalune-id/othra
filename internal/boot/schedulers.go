package boot

import (
	"context"
	"fmt"
	"log/slog"

	"altalune.id/othra/apperror"
	"altalune.id/othra/config"
	"altalune.id/othra/internal/enduser"
	"altalune.id/othra/internal/kernel"
	"altalune.id/othra/internal/kernel/session"
	"altalune.id/othra/internal/kernel/storage"
	"altalune.id/othra/internal/keys"
	"altalune.id/othra/internal/resourceserver"
	"altalune.id/othra/scheduler"
	"altalune.id/othra/worker"
)

func schedulerOff(o *options, cfg config.Config) string {
	switch {
	case !o.scheduler:
		return "WithScheduler(false)"
	case o.consumerOnly:
		return "WithConsumerOnly"
	case !cfg.Scheduler.Enabled:
		return "scheduler.enabled=false"
	}
	return ""
}

func schedulerConfig(c config.SchedulerConfig, k kernel.Kernel, log *slog.Logger) scheduler.Options {
	if log == nil {
		log = slog.Default()
	}
	return scheduler.Options{
		Logger:        log,
		Reporter:      reporterAdapter{report: k.Reporter.Unexpected, log: log},
		Meter:         k.Meter,
		ShutdownGrace: c.ShutdownGrace,
	}
}

func buildScheduler(k kernel.Kernel, cfg config.Config, s *Services, log *slog.Logger) (*scheduler.Runner, error) {
	r, err := scheduler.New(schedulerConfig(cfg.Scheduler, k, log))
	if err != nil {
		return nil, err
	}
	jobs := session.NewScheduler(s.Sessions, orDiscard(log)).SchedulerJobs()
	jobs = append(jobs, storage.NewScheduler(s.Storage, outboxTenants{orgs: s.Orgs}, orDiscard(log)).SchedulerJobs()...)
	if s.Keys != nil {
		jobs = append(jobs, keys.NewScheduler(s.Keys, projectTenants{orgs: s.Orgs, projects: s.Projects}, orDiscard(log)).SchedulerJobs()...)
	}
	if s.EndUsers != nil {
		eu := enduser.NewScheduler(s.EndUsers, projectTenants{orgs: s.Orgs, projects: s.Projects}, orDiscard(log))
		jobs = append(jobs, eu.SchedulerJobs()...)
	}
	if s.ResourceServers != nil {
		rs := resourceserver.NewScheduler(s.ResourceServers, projectTenants{orgs: s.Orgs, projects: s.Projects}, orDiscard(log))
		jobs = append(jobs, rs.SchedulerJobs()...)
	}
	for _, j := range jobs {
		if err := r.Register(j); err != nil {
			return nil, fmt.Errorf("boot: register %s: %w", j.Name, err)
		}
	}
	return r, nil
}

var _ worker.Worker = (*scheduler.Runner)(nil)
var _ scheduler.ErrorReporter = reporterAdapter{}

type reporterAdapter struct {
	report apperror.UnexpectedFunc
	log    *slog.Logger
}

func (a reporterAdapter) Report(ctx context.Context, message string, cause error, attrs ...any) {
	if alreadyReported(cause) {
		a.log.ErrorContext(ctx, message, append([]any{slog.Any("error", cause)}, attrs...)...)
		return
	}
	_ = a.report(ctx, message, cause, attrs...)
}

func alreadyReported(err error) bool {
	if err == nil {
		return false
	}
	if _, ok := apperror.AsAppError(err); ok {
		return true
	}
	joined, ok := err.(interface{ Unwrap() []error })
	if !ok {
		return false
	}
	causes := joined.Unwrap()
	if len(causes) == 0 {
		return false
	}
	for _, c := range causes {
		if !alreadyReported(c) {
			return false
		}
	}
	return true
}
