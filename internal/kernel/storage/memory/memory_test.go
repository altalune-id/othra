package memory

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/altalune-id/othra/internal/kernel/storage"
)

func TestDriver_StandsInForABucket(t *testing.T) {
	ctx := context.Background()
	d := New()
	up, err := d.PresignPut(ctx, "public/othra/projects/p/logo/f.png", "image/png", 3, time.Minute)
	require.NoError(t, err)
	assert.Equal(t, "memory://upload/public%2Fothra%2Fprojects%2Fp%2Flogo%2Ff.png", up.URL)
	assert.Equal(t, "PUT", up.Method)
	assert.Equal(t, map[string]string{"Content-Type": "image/png", "Content-Length": "3"}, up.Headers)
	assert.WithinDuration(t, time.Now().Add(time.Minute), up.ExpiresAt, 5*time.Second)

	_, err = d.Head(ctx, "k")
	assert.True(t, storage.IsObjectNotFoundError(err))
	_, err = d.ReadPrefix(ctx, "k", 2)
	assert.True(t, storage.IsObjectNotFoundError(err))

	d.Put("k", "image/png", []byte("abc"))
	assert.True(t, d.Has("k"))
	obj, err := d.Head(ctx, "k")
	require.NoError(t, err)
	assert.Equal(t, storage.Object{Size: 3, ContentType: "image/png"}, obj)
	b, err := d.ReadPrefix(ctx, "k", 2)
	require.NoError(t, err)
	assert.Equal(t, []byte("ab"), b)
	b, err = d.ReadPrefix(ctx, "k", 16)
	require.NoError(t, err)
	assert.Equal(t, []byte("abc"), b)

	u, err := d.PresignGet(ctx, "k", time.Minute)
	require.NoError(t, err)
	assert.Contains(t, u, "memory://get/k?expires=")

	require.NoError(t, d.Delete(ctx, "k"))
	require.NoError(t, d.Delete(ctx, "k"), "a missing object is not an error")
	assert.False(t, d.Has("k"))
}
