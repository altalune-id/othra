// Package kernel holds the cross-cutting primitives every service receives.
package kernel

import (
	"errors"
	"io"
	"log/slog"
	"slices"

	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"

	"altalune.id/othra/apperror"
	"altalune.id/othra/internal/kernel/db"
	"altalune.id/othra/internal/kernel/queue"
	"altalune.id/othra/internal/kernel/storage"
)

// Scope is the instrumentation scope for the kernel's tracer and meter.
const Scope = "altalune.id/othra"

// Kernel is the platform bag handed to every service.
type Kernel struct {
	Log      *slog.Logger
	Pool     db.Pool
	Tracer   trace.Tracer
	Meter    metric.Meter
	Reporter *apperror.Reporter
	// Queue is always non-nil: Disabled when queue.enabled is false.
	Queue queue.Queue
	// Storage is the file storage driver, nil when storage.driver is unset.
	Storage storage.Driver

	closers []io.Closer
}

// AddCloser registers a resource for reverse-order shutdown by Close.
func (k *Kernel) AddCloser(c io.Closer) {
	if k == nil || c == nil {
		return
	}
	k.closers = append(k.closers, c)
}

// Close closes every registered resource in reverse-creation order.
func (k *Kernel) Close() error {
	if k == nil {
		return nil
	}
	var errs []error
	for _, c := range slices.Backward(k.closers) {
		if err := c.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
