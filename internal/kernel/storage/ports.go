package storage

import (
	"context"
	"time"
)

// Driver is the object storage behind the feature: an S3-compatible bucket, or memory in tests.
type Driver interface {
	// PresignPut signs a PUT of exactly size bytes of contentType to key, valid for ttl.
	PresignPut(ctx context.Context, key, contentType string, size int64, ttl time.Duration) (Upload, error)
	// Head reports the object at key, or *ObjectNotFoundError.
	Head(ctx context.Context, key string) (Object, error)
	// ReadPrefix returns up to the first n bytes of the object at key, or *ObjectNotFoundError.
	ReadPrefix(ctx context.Context, key string, n int64) ([]byte, error)
	// Delete removes the object at key; a missing object is not an error.
	Delete(ctx context.Context, key string) error
	// PresignGet signs a GET of key, valid for ttl.
	PresignGet(ctx context.Context, key string, ttl time.Duration) (string, error)
}

// Tenants runs fn once per org, with ctx scoped to it.
type Tenants interface {
	Each(ctx context.Context, fn func(ctx context.Context, tenantID string) error) error
}
