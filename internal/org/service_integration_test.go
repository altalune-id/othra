//go:build integration

package org_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/altalune-id/othra/apperror"
	"github.com/altalune-id/othra/internal/kernel/db"
	"github.com/altalune-id/othra/internal/kernel/tenant"
	"github.com/altalune-id/othra/internal/org"
	"github.com/altalune-id/othra/internal/testutil/fakes"
)

func TestService_CreateWithAFailedHookLeavesNoOrg(t *testing.T) {
	for _, slug := range []string{"acme", ""} {
		t.Run("slug="+slug, func(t *testing.T) {
			store, pool := newOrgStore(t)
			var hookInTx bool
			svc := org.NewService(store, db.NewTransactor(pool), fakes.NoMemberRoles{}, func(ctx context.Context, _ *org.Org) error {
				_, hookInTx = db.CurrentTx(ctx)
				return errors.New("default project failed")
			}, apperror.NewReporter(nil, false).Unexpected)

			_, err := svc.Create(t.Context(), org.CreateRequest{Slug: slug, Name: "Acme"})
			require.Error(t, err)
			assert.True(t, hookInTx, "the hook runs in the org's unit of work")
			ids, lerr := store.IDs(t.Context())
			require.NoError(t, lerr)
			assert.Empty(t, ids, "an org never exists without what its hook adds")
		})
	}
}

func TestService_CreateRunsTheHookInTheSameUnitOfWork(t *testing.T) {
	store, pool := newOrgStore(t)
	var hookInTx bool
	svc := org.NewService(store, db.NewTransactor(pool), fakes.NoMemberRoles{}, func(ctx context.Context, _ *org.Org) error {
		_, hookInTx = db.CurrentTx(ctx)
		return nil
	}, apperror.NewReporter(nil, false).Unexpected)

	o, err := svc.Create(t.Context(), org.CreateRequest{Name: "Acme"})
	require.NoError(t, err)
	assert.True(t, hookInTx)
	_, err = store.ByID(tenant.Into(t.Context(), tenant.Context{OrgID: o.ID}), o.ID)
	require.NoError(t, err)
}
