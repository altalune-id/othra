package boot

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	noopmetric "go.opentelemetry.io/otel/metric/noop"
	nooptrace "go.opentelemetry.io/otel/trace/noop"

	"altalune.id/othra/apperror"
	"altalune.id/othra/config"
	"altalune.id/othra/internal/kernel"
	"altalune.id/othra/internal/kernel/db"
	"altalune.id/othra/internal/kernel/session"
	"altalune.id/othra/internal/kernel/storage"
	"altalune.id/othra/internal/kernel/storage/memory"
	storages3 "altalune.id/othra/internal/kernel/storage/s3"
	s3conn "altalune.id/othra/s3"
)

func TestBuildStorageDriver(t *testing.T) {
	d, err := buildStorageDriver(config.Config{}, nil)
	require.NoError(t, err)
	assert.Nil(t, d, "no driver: uploads are off")

	d, err = buildStorageDriver(config.Config{Storage: config.StorageConfig{Driver: config.StorageDriverMemory}}, nil)
	require.NoError(t, err)
	assert.IsType(t, &memory.Driver{}, d)

	s3cfg := config.Config{Storage: config.StorageConfig{Driver: config.StorageDriverS3}}
	_, err = buildStorageDriver(s3cfg, nil)
	assert.ErrorContains(t, err, "no S3 connection")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }))
	t.Cleanup(srv.Close)
	c, err := s3conn.Open(t.Context(), s3Config(s3TestConfig(srv.URL)), nooptrace.NewTracerProvider())
	require.NoError(t, err)
	d, err = buildStorageDriver(s3cfg, c)
	require.NoError(t, err)
	assert.IsType(t, &storages3.Driver{}, d)

	_, err = buildStorageDriver(config.Config{Storage: config.StorageConfig{Driver: "gcs"}}, nil)
	assert.ErrorContains(t, err, `storage.driver "gcs" has no driver`)
}

func TestStorageConfig(t *testing.T) {
	assert.Equal(t, storage.Config{
		App: "othra", PublicBaseURL: "https://files.altalune.id", ImageTransform: storage.ImageTransformCloudflare,
		UploadURLTTL: 10 * time.Minute, PendingTTL: time.Hour, PrivateURLTTL: 5 * time.Minute,
	}, storageConfig(config.StorageConfig{
		App: "othra", PublicBaseURL: "https://files.altalune.id", ImageTransform: config.StorageImageTransformCloudflare,
		Upload:  config.StorageUploadConfig{URLTTL: 10 * time.Minute, PendingTTL: time.Hour},
		Private: config.StoragePrivateConfig{Signer: config.StoragePrivateSignerPresign, URLTTL: 5 * time.Minute},
	}))
}

func TestBuildStorage_FollowsTheKernelDriver(t *testing.T) {
	k := kernel.Kernel{Reporter: apperror.NewReporter(nil, false)}
	assert.False(t, buildStorage(k, config.Config{}).Enabled(), "Disabled without a driver")
	k.Storage = memory.New()
	assert.True(t, buildStorage(k, config.Config{}).Enabled())
}

func TestBuildScheduler_RegistersTheSweepWhenStorageIsOn(t *testing.T) {
	k := kernel.Kernel{Meter: noopmetric.NewMeterProvider().Meter("test"), Reporter: apperror.NewReporter(nil, false), Storage: memory.New()}
	svcs := &Services{Sessions: session.NewMemoryStore(), Storage: buildStorage(k, config.Config{})}
	r, err := buildScheduler(k, config.Config{Scheduler: config.SchedulerConfig{Enabled: true}}, svcs, testLogger())
	require.NoError(t, err)
	names := []string{}
	for _, j := range r.Jobs() {
		names = append(names, j.Name)
	}
	assert.ElementsMatch(t, []string{"session-sweep", "storage-sweep"}, names)
}

func TestBoot_MemoryStorageInDevelopment(t *testing.T) {
	restoreSeams(t)
	poolDB, _ := newDB(t)
	setupTelemetry = stubTelemetry(nil, nil)
	openPool = func(context.Context, config.PostgresConfig, *slog.Logger) (db.Pool, error) {
		return db.Pool{W: poolDB, R: poolDB}, nil
	}
	cfg := config.Config{Postgres: config.PostgresConfig{DSN: "x"}, Storage: config.StorageConfig{Driver: config.StorageDriverMemory}}
	app, err := Boot(t.Context(), cfg, testLogger())
	require.NoError(t, err)
	assert.True(t, app.Services.Storage.Enabled())
	require.NoError(t, app.Close(t.Context()))
}
