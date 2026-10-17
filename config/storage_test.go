package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad_StorageDefaults(t *testing.T) {
	c := validConfig(t)
	assert.Equal(t, StorageConfig{
		App: "othra", ImageTransform: StorageImageTransformNone,
		Upload:  StorageUploadConfig{URLTTL: 10 * time.Minute, PendingTTL: time.Hour},
		Private: StoragePrivateConfig{Signer: StoragePrivateSignerPresign, URLTTL: 5 * time.Minute},
	}, c.Storage, "uploads are off until a driver is set")
}

func TestLoad_StorageFromEnv(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("OTHRA_STORAGE_DRIVER", "memory")
	t.Setenv("OTHRA_ENV", "development")
	t.Setenv("OTHRA_STORAGE_APP", "yasaku")
	t.Setenv("OTHRA_STORAGE_PUBLIC_BASE_URL", "http://localhost:3900/altalune-files")
	t.Setenv("OTHRA_STORAGE_IMAGE_TRANSFORM", "cloudflare")
	t.Setenv("OTHRA_STORAGE_UPLOAD_URL_TTL", "2m")
	t.Setenv("OTHRA_STORAGE_UPLOAD_PENDING_TTL", "30m")
	t.Setenv("OTHRA_STORAGE_PRIVATE_URL_TTL", "1m")
	c, err := Load(missingPath(t))
	require.NoError(t, err)
	assert.Equal(t, StorageDriverMemory, c.Storage.Driver)
	assert.Equal(t, "yasaku", c.Storage.App)
	assert.Equal(t, "http://localhost:3900/altalune-files", c.Storage.PublicBaseURL)
	assert.Equal(t, StorageImageTransformCloudflare, c.Storage.ImageTransform)
	assert.Equal(t, 2*time.Minute, c.Storage.Upload.URLTTL)
	assert.Equal(t, 30*time.Minute, c.Storage.Upload.PendingTTL)
	assert.Equal(t, time.Minute, c.Storage.Private.URLTTL)
	assert.Empty(t, c.S3Features(), "memory needs no connection")
}

func TestLoad_StorageRejectsAnUnknownDriver(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("OTHRA_STORAGE_DRIVER", "gcs")
	_, err := Load(missingPath(t))
	assert.ErrorContains(t, err, "storage.driver")
}

func TestValidate_Storage(t *testing.T) {
	useStorage := func(c *Config) {
		c.Storage.Driver = StorageDriverMemory
		c.Env = EnvDevelopment
		c.Storage.PublicBaseURL = "http://localhost:3900/altalune-files"
	}
	for name, tc := range map[string]struct {
		mutate func(c *Config)
		check  func(t *testing.T, err error)
	}{
		"off is ignored, even when broken": {
			mutate: func(c *Config) { c.Storage.App = "Bad App"; c.Storage.Upload.URLTTL = 0 },
			check:  func(t *testing.T, err error) { assert.NoError(t, err) },
		},
		"memory in development ok":    {mutate: useStorage, check: func(t *testing.T, err error) { assert.NoError(t, err) }},
		"s3 in production with https": {mutate: useS3, check: func(t *testing.T, err error) { assert.NoError(t, err) }},
		"memory in production": {
			mutate: func(c *Config) {
				useStorage(c)
				c.Env = EnvProduction
				c.Storage.PublicBaseURL = "https://files.altalune.id"
			},
			check: invalidSetting("storage.driver"),
		},
		"bad app":   {mutate: func(c *Config) { useStorage(c); c.Storage.App = "Othra" }, check: invalidSetting("storage.app")},
		"empty app": {mutate: func(c *Config) { useStorage(c); c.Storage.App = "" }, check: invalidSetting("storage.app")},
		"no public base url": {
			mutate: func(c *Config) { useStorage(c); c.Storage.PublicBaseURL = "" },
			check:  missingSetting("storage.publicBaseURL"),
		},
		"http public base url in production": {
			mutate: func(c *Config) { useS3(c); c.Storage.PublicBaseURL = "http://files.altalune.id" },
			check: func(t *testing.T, err error) {
				invalidSetting("storage.publicBaseURL")(t, err)
				assert.ErrorContains(t, err, "https://")
			},
		},
		"relative public base url": {
			mutate: func(c *Config) { useStorage(c); c.Storage.PublicBaseURL = "/files" },
			check:  invalidSetting("storage.publicBaseURL"),
		},
		"public base url with a query": {
			mutate: func(c *Config) { useStorage(c); c.Storage.PublicBaseURL = "http://localhost:3900/?x=1" },
			check:  invalidSetting("storage.publicBaseURL"),
		},
		"upload url ttl too short": {
			mutate: func(c *Config) { useStorage(c); c.Storage.Upload.URLTTL = time.Millisecond },
			check:  invalidSetting("storage.upload.urlTTL"),
		},
		"upload url ttl beyond 7 days": {
			mutate: func(c *Config) { useStorage(c); c.Storage.Upload.URLTTL = 8 * 24 * time.Hour },
			check:  invalidSetting("storage.upload.urlTTL"),
		},
		"pending shorter than the url": {
			mutate: func(c *Config) { useStorage(c); c.Storage.Upload.PendingTTL = time.Minute },
			check:  invalidSetting("storage.upload.pendingTTL"),
		},
		"private url ttl zero": {
			mutate: func(c *Config) { useStorage(c); c.Storage.Private.URLTTL = 0 },
			check:  invalidSetting("storage.private.urlTTL"),
		},
	} {
		t.Run(name, func(t *testing.T) {
			c := validConfig(t)
			tc.mutate(c)
			tc.check(t, c.validateStorage())
		})
	}
}
