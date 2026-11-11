package outbox

import (
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/altalune-id/othra/internal/kernel/db/dbtest"
)

func TestRequeueWakesTheWorkerAfterCommit(t *testing.T) {
	ctx := tenantedCtx(uuid.New(), uuid.New())
	s, mock, _ := newTestStore(t)
	wakes := 0
	s.OnEnqueue(func() { wakes++ })
	mock.ExpectBegin()
	dbtest.ExpectScope(mock)
	mock.ExpectExec("UPDATE").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	require.NoError(t, s.Requeue(ctx, uuid.New(), "ep"))
	assert.Equal(t, 1, wakes)
}

func TestRequeueFailedWakesOnlyWhenItRequeuedSomething(t *testing.T) {
	for name, tc := range map[string]struct {
		rows  int64
		wakes int
	}{
		"some": {rows: 3, wakes: 1},
		"none": {rows: 0, wakes: 0},
	} {
		t.Run(name, func(t *testing.T) {
			s, mock, _ := newTestStore(t)
			wakes := 0
			s.OnEnqueue(func() { wakes++ })
			mock.ExpectBegin()
			dbtest.ExpectScope(mock)
			mock.ExpectExec("UPDATE").WillReturnResult(sqlmock.NewResult(0, tc.rows))
			mock.ExpectCommit()
			_, err := s.RequeueFailed(tenantedCtx(uuid.New(), uuid.New()), "ep")
			require.NoError(t, err)
			assert.Equal(t, tc.wakes, wakes)
		})
	}
}
