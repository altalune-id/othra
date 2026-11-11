package db

import (
	"context"
	"database/sql"
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/altalune-id/othra/internal/kernel/tenant"
)

func TestAfterCommit_RunsAfterTheCommit(t *testing.T) {
	pool, mock := mockPool(t)
	mock.ExpectBegin()
	expectScope(mock, tenant.Context{})
	mock.ExpectCommit()

	ran := 0
	err := RunInTx(context.Background(), pool, func(ctx context.Context) error {
		AfterCommit(ctx, func() { ran++ })
		assert.Zero(t, ran, "a hook must wait for the commit")
		return nil
	})
	require.NoError(t, err)
	assert.Equal(t, 1, ran)
}

func TestAfterCommit_SkipsARollback(t *testing.T) {
	pool, mock := mockPool(t)
	mock.ExpectBegin()
	expectScope(mock, tenant.Context{})
	mock.ExpectRollback()

	ran := 0
	_ = RunInTx(context.Background(), pool, func(ctx context.Context) error {
		AfterCommit(ctx, func() { ran++ })
		return errors.New("boom")
	})
	assert.Zero(t, ran)
}

func TestAfterCommit_SkipsAFailedCommit(t *testing.T) {
	pool, mock := mockPool(t)
	mock.ExpectBegin()
	expectScope(mock, tenant.Context{})
	mock.ExpectCommit().WillReturnError(errors.New("lost"))

	ran := 0
	err := RunInTx(context.Background(), pool, func(ctx context.Context) error {
		AfterCommit(ctx, func() { ran++ })
		return nil
	})
	require.Error(t, err)
	assert.Zero(t, ran)
}

func TestAfterCommit_NestedUnitsRunOnceAfterTheOuterCommit(t *testing.T) {
	pool, mock := mockPool(t)
	mock.ExpectBegin()
	expectScope(mock, tenant.Context{})
	mock.ExpectCommit()

	ran := 0
	err := RunInTx(context.Background(), pool, func(ctx context.Context) error {
		if err := RunInTx(ctx, pool, func(ctx context.Context) error {
			AfterCommit(ctx, func() { ran++ })
			return nil
		}); err != nil {
			return err
		}
		if err := InTx(ctx, pool, func(*sql.Tx) error {
			AfterCommit(ctx, func() { ran++ })
			return nil
		}); err != nil {
			return err
		}
		assert.Zero(t, ran, "a joined unit must not run hooks before the outer commit")
		return nil
	})
	require.NoError(t, err)
	assert.Equal(t, 2, ran)
}

func TestAfterCommit_RunsAtOnceWithoutATransaction(t *testing.T) {
	ran := 0
	AfterCommit(context.Background(), func() { ran++ })
	assert.Equal(t, 1, ran)
}

func TestAfterCommit_IsSafeFromConcurrentGoroutines(t *testing.T) {
	pool, mock := mockPool(t)
	mock.ExpectBegin()
	expectScope(mock, tenant.Context{})
	mock.ExpectCommit()

	var ran atomic.Int32
	err := RunInTx(context.Background(), pool, func(ctx context.Context) error {
		var wg sync.WaitGroup
		for range 8 {
			wg.Go(func() { AfterCommit(ctx, func() { ran.Add(1) }) })
		}
		wg.Wait()
		return nil
	})
	require.NoError(t, err)
	assert.EqualValues(t, 8, ran.Load())
}

func TestAfterCommit_HooksOnAContextWithTxNeverRun(t *testing.T) {
	ran := 0
	AfterCommit(ContextWithTx(context.Background(), &sql.Tx{}), func() { ran++ })
	assert.Zero(t, ran, "a caller-owned transaction has no commit for the hook to follow")
}
