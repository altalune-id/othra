package boot

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"altalune.id/othra/config"
	"altalune.id/othra/internal/kernel"
	"altalune.id/othra/internal/kernel/db"
	s3conn "altalune.id/othra/s3"
)

const s3TestSecret = "hunter2-s3-secret-access-key"

func s3TestConfig(endpoint string) config.S3Config {
	return config.S3Config{
		Endpoint: endpoint, BrowserEndpoint: "http://localhost:3900", Region: "garage", Bucket: "altalune-files",
		AccessKeyID: "GK000000000000000000000001", SecretAccessKey: s3TestSecret, UsePathStyle: true, Timeout: time.Second,
	}
}

func s3FeatureConfig(endpoint string) config.Config {
	return config.Config{
		Postgres: config.PostgresConfig{DSN: "x"},
		Storage:  config.StorageConfig{Driver: config.StorageDriverS3, PublicBaseURL: "https://files.altalune.id"},
		S3:       s3TestConfig(endpoint),
	}
}

func TestS3Config(t *testing.T) {
	assert.Equal(t, s3conn.Config{
		Endpoint: "http://garage:3900", BrowserEndpoint: "http://localhost:3900", Region: "garage", Bucket: "altalune-files",
		AccessKeyID: "GK000000000000000000000001", SecretAccessKey: s3TestSecret, UsePathStyle: true, Timeout: time.Second,
	}, s3Config(s3TestConfig("http://garage:3900")))
}

func stubS3(t *testing.T) (calls *int, client *s3conn.Client) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }))
	t.Cleanup(srv.Close)
	client, err := s3conn.Open(t.Context(), s3Config(s3TestConfig(srv.URL)))
	require.NoError(t, err)
	calls = new(int)
	openS3Fn = func(context.Context, s3conn.Config) (*s3conn.Client, error) {
		*calls++
		return client, nil
	}
	return calls, client
}

// Review Focus 1: with nothing selecting S3, boot never opens it, even when s3.endpoint is set.
func TestBuildS3_UnusedNeverConnects(t *testing.T) {
	restoreSeams(t)
	calls, _ := stubS3(t)
	var logs bytes.Buffer
	log := slog.New(slog.NewTextHandler(&logs, nil))

	k := &kernel.Kernel{}
	c, sh, err := buildS3(t.Context(), k, config.Config{}, log)
	require.NoError(t, err)
	assert.Nil(t, c)
	assert.Nil(t, sh)
	assert.Empty(t, logs.String(), "no endpoint, nothing to say")

	cfg := config.Config{S3: s3TestConfig("https://acct.r2.cloudflarestorage.com")}
	cfg.Storage.Driver = config.StorageDriverMemory
	c, sh, err = buildS3(t.Context(), k, cfg, log)
	require.NoError(t, err)
	assert.Nil(t, c)
	assert.Nil(t, sh, "the memory driver needs no connection")
	assert.Zero(t, *calls)
	assert.Contains(t, logs.String(), "no feature uses S3")
	assert.NotContains(t, logs.String(), s3TestSecret)
	assert.NotContains(t, logs.String(), "acct.r2")
}

func TestBuildS3_FeatureConnectsOnceWithTheMappedConfig(t *testing.T) {
	restoreSeams(t)
	_, client := stubS3(t)
	var got s3conn.Config
	calls := 0
	openS3Fn = func(_ context.Context, c s3conn.Config) (*s3conn.Client, error) {
		calls++
		got = c
		return client, nil
	}
	var logs bytes.Buffer
	k := &kernel.Kernel{}
	c, sh, err := buildS3(t.Context(), k, s3FeatureConfig("http://garage:3900"), slog.New(slog.NewTextHandler(&logs, nil)))
	require.NoError(t, err)
	require.NotNil(t, sh)
	assert.Equal(t, 1, calls)
	assert.Same(t, client, c)
	assert.Equal(t, s3Config(s3TestConfig("http://garage:3900")), got)
	assert.True(t, sh.Ready(), "the boot check succeeded")
	assert.Equal(t, "s3-health", sh.Name())
	assert.Contains(t, logs.String(), "used_by=storage.driver")
	assert.NotContains(t, logs.String(), s3TestSecret)
	require.NoError(t, k.Close())
}

func TestBuildS3_ConnectErrorFailsBoot(t *testing.T) {
	restoreSeams(t)
	openS3Fn = func(context.Context, s3conn.Config) (*s3conn.Client, error) {
		return nil, errors.New(`s3: cannot reach bucket "altalune-files" at garage:3900 (s3.endpoint): refused`)
	}
	_, _, err := buildS3(t.Context(), &kernel.Kernel{}, s3FeatureConfig("http://garage:3900"), testLogger())
	assert.ErrorContains(t, err, "boot: s3: cannot reach bucket")
}

func TestBoot_DefaultConfigNeverOpensS3(t *testing.T) {
	restoreSeams(t)
	calls, _ := stubS3(t)
	poolDB, _ := newDB(t)
	setupTelemetry = stubTelemetry(nil, nil)
	openPool = func(context.Context, config.PostgresConfig, *slog.Logger) (db.Pool, error) {
		return db.Pool{W: poolDB, R: poolDB}, nil
	}

	app, err := Boot(context.Background(), config.Config{Postgres: config.PostgresConfig{DSN: "x"}}, testLogger())
	require.NoError(t, err)
	assert.Zero(t, *calls)
	assert.NotContains(t, workerNames(app), "s3-health")
	require.NoError(t, app.Close(context.Background()))
}

func TestBoot_S3FeatureRegistersItsHealthWorker(t *testing.T) {
	restoreSeams(t)
	calls, _ := stubS3(t)
	poolDB, _ := newDB(t)
	setupTelemetry = stubTelemetry(nil, nil)
	openPool = func(context.Context, config.PostgresConfig, *slog.Logger) (db.Pool, error) {
		return db.Pool{W: poolDB, R: poolDB}, nil
	}

	app, err := Boot(context.Background(), s3FeatureConfig("http://garage:3900"), testLogger())
	require.NoError(t, err)
	assert.Equal(t, 1, *calls)
	assert.Contains(t, workerNames(app), "s3-health")
	require.NoError(t, app.Close(context.Background()))
}

func TestBoot_S3ConnectErrorFailsBootWithoutTheSecret(t *testing.T) {
	restoreSeams(t)
	poolDB, _ := newDB(t)
	setupTelemetry = stubTelemetry(nil, nil)
	openPool = func(context.Context, config.PostgresConfig, *slog.Logger) (db.Pool, error) {
		return db.Pool{W: poolDB, R: poolDB}, nil
	}
	cfg := s3FeatureConfig("http://127.0.0.1:1")
	cfg.S3.Timeout = 300 * time.Millisecond
	_, err := Boot(context.Background(), cfg, testLogger())
	require.Error(t, err)
	assert.ErrorContains(t, err, `s3: cannot reach bucket "altalune-files" at 127.0.0.1:1 (s3.endpoint)`)
	assert.NotContains(t, err.Error(), s3TestSecret)
}

func TestBoot_LaterFailureClosesS3(t *testing.T) {
	restoreSeams(t)
	calls, _ := stubS3(t)
	poolDB, _ := newDB(t)
	setupTelemetry = stubTelemetry(nil, nil)
	openPool = func(context.Context, config.PostgresConfig, *slog.Logger) (db.Pool, error) {
		return db.Pool{W: poolDB, R: poolDB}, nil
	}
	cfg := s3FeatureConfig("http://garage:3900")
	cfg.Mail = config.MailConfig{Driver: config.MailDriverSMTP}
	_, err := Boot(context.Background(), cfg, testLogger())
	require.ErrorContains(t, err, "mail")
	assert.Equal(t, 1, *calls, "boot reached the mailer after opening S3")
}
