//go:build integration

package boot

import (
	"context"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/altalune-id/othra/config"
	"github.com/altalune-id/othra/internal/kernel/db"
	"github.com/altalune-id/othra/internal/testutil/s3test"
)

func TestBoot_S3FeatureOpensTheRealBucket(t *testing.T) {
	restoreSeams(t)
	h := s3test.New(t)
	poolDB, _ := newDB(t)
	setupTelemetry = stubTelemetry(nil, nil)
	openPool = func(context.Context, config.PostgresConfig, *slog.Logger) (db.Pool, error) {
		return db.Pool{W: poolDB, R: poolDB}, nil
	}
	cfg := s3FeatureConfig(h.Endpoint)
	cfg.S3.Region, cfg.S3.Bucket = h.Region, h.Bucket
	cfg.S3.AccessKeyID, cfg.S3.SecretAccessKey = h.AccessKeyID, h.SecretAccessKey

	app, err := Boot(t.Context(), cfg, testLogger())
	require.NoError(t, err)
	assert.Contains(t, workerNames(app), "s3-health")
	require.NoError(t, app.Close(context.Background()))
}
