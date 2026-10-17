package boot

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"go.opentelemetry.io/otel/trace"

	"altalune.id/othra/config"
	"altalune.id/othra/internal/health"
	"altalune.id/othra/internal/kernel"
	s3conn "altalune.id/othra/s3"
)

type s3Opener func(context.Context, s3conn.Config, trace.TracerProvider) (*s3conn.Client, error)

//nolint:gochecknoglobals // test seam.
var openS3Fn s3Opener = s3conn.Open

var _ health.Check = (*s3conn.Health)(nil)

func s3Config(c config.S3Config) s3conn.Config {
	return s3conn.Config{
		Endpoint:        c.Endpoint,
		BrowserEndpoint: c.BrowserEndpoint,
		Region:          c.Region,
		Bucket:          c.Bucket,
		AccessKeyID:     c.AccessKeyID,
		SecretAccessKey: c.SecretAccessKey,
		UsePathStyle:    c.UsePathStyle,
		Timeout:         c.Timeout,
	}
}

func buildS3(
	ctx context.Context, k *kernel.Kernel, cfg config.Config, tp trace.TracerProvider, log *slog.Logger,
) (*s3conn.Client, *s3conn.Health, error) {
	features := cfg.S3Features()
	if len(features) == 0 {
		if cfg.S3.Endpoint != "" && log != nil {
			log.InfoContext(ctx, "boot: s3.endpoint is set but no feature uses S3; not connecting")
		}
		return nil, nil, nil
	}
	client, err := openS3Fn(ctx, s3Config(cfg.S3), tp)
	if err != nil {
		return nil, nil, fmt.Errorf("boot: %w", err)
	}
	k.AddCloser(client)
	if log != nil {
		log.InfoContext(ctx, "boot: s3 connected", slog.String("bucket", cfg.S3.Bucket), slog.String("used_by", strings.Join(features, ", ")))
	}
	return client, s3conn.NewHealth(client, cfg.S3.Timeout, log), nil
}
