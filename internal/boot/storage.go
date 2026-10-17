package boot

import (
	"errors"
	"fmt"

	"altalune.id/othra/config"
	"altalune.id/othra/internal/kernel"
	"altalune.id/othra/internal/kernel/storage"
	"altalune.id/othra/internal/kernel/storage/memory"
	storages3 "altalune.id/othra/internal/kernel/storage/s3"
	s3conn "altalune.id/othra/s3"
)

func buildStorageDriver(cfg config.Config, s3c *s3conn.Client) (storage.Driver, error) {
	switch cfg.Storage.Driver {
	case "":
		return nil, nil
	case config.StorageDriverS3:
		if s3c == nil {
			return nil, errors.New("boot: storage.driver is s3 but no S3 connection is open")
		}
		return storages3.New(s3c), nil
	case config.StorageDriverMemory:
		return memory.New(), nil
	default:
		return nil, fmt.Errorf("boot: storage.driver %q has no driver", cfg.Storage.Driver)
	}
}

func storageConfig(c config.StorageConfig) storage.Config {
	return storage.Config{
		App:            c.App,
		PublicBaseURL:  c.PublicBaseURL,
		ImageTransform: storage.ImageTransform(c.ImageTransform),
		UploadURLTTL:   c.Upload.URLTTL,
		PendingTTL:     c.Upload.PendingTTL,
		PrivateURLTTL:  c.Private.URLTTL,
	}
}

func buildStorage(k kernel.Kernel, cfg config.Config) *storage.Service {
	store := storage.NewStore(k.Pool, cfg.Postgres.Schema, cfg.Postgres.TablePrefix)
	return storage.NewService(k.Storage, store, storageConfig(cfg.Storage), k.Reporter.Unexpected)
}
