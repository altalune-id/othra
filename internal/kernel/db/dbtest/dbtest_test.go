package dbtest_test

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-jet/jet/v2/qrm"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/altalune-id/othra/internal/kernel/db"
	"github.com/altalune-id/othra/internal/kernel/db/dbtest"
	"github.com/altalune-id/othra/internal/kernel/tenant"
)

func TestExpectTx_MatchesTheTransactionKernelDBOpens(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	dbtest.ExpectTx(mock)
	mock.ExpectCommit()

	ctx := tenant.Into(context.Background(), tenant.Context{OrgID: uuid.New()})
	require.NoError(t, db.Read(ctx, db.Pool{W: sqlDB, R: sqlDB}, func(qrm.DB) error { return nil }))
	assert.NoError(t, mock.ExpectationsWereMet())
}
