package nats

import (
	"log/slog"

	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"

	"altalune.id/othra/apperror"
)

// Options configures Open.
type Options struct {
	Log        *slog.Logger
	Tracer     trace.Tracer
	Meter      metric.Meter
	Unexpected apperror.UnexpectedFunc
}
