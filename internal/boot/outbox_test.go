package boot

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"altalune.id/othra/apperror"
	"altalune.id/othra/internal/kernel/db"
	"altalune.id/othra/internal/kernel/outbox"
	"altalune.id/othra/internal/kernel/tenant"
	"altalune.id/othra/internal/org"
	"altalune.id/othra/internal/testutil/fakes"
	"altalune.id/othra/internal/webhook"
	"altalune.id/othra/sealer"
)

func testUnexpected() apperror.UnexpectedFunc { return apperror.NewReporter(nil, false).Unexpected }

func TestOutboxTenantsEach(t *testing.T) {
	o := &org.Org{ID: uuid.New(), Slug: "acme", Name: "Acme"}
	svc := org.NewService(&fakes.OrgStore{Orgs: []*org.Org{o}}, fakes.Transactor{}, fakes.NoMemberRoles{}, nil, testUnexpected())

	var seen uuid.UUID
	err := outboxTenants{orgs: svc}.Each(context.Background(), func(ctx context.Context, _ string) error {
		tc, ferr := tenant.From(ctx)
		require.NoError(t, ferr)
		seen = tc.OrgID
		return nil
	})
	require.NoError(t, err)
	assert.Equal(t, o.ID, seen)
}

func TestOutboxTenantsListError(t *testing.T) {
	svc := org.NewService(&fakes.OrgStore{Err: errors.New("boom")}, fakes.Transactor{}, fakes.NoMemberRoles{}, nil, testUnexpected())
	err := outboxTenants{orgs: svc}.Each(context.Background(), func(context.Context, string) error { return nil })
	assert.Error(t, err)
}

func TestOutboxTenantsFnError(t *testing.T) {
	o := &org.Org{ID: uuid.New()}
	svc := org.NewService(&fakes.OrgStore{Orgs: []*org.Org{o}}, fakes.Transactor{}, fakes.NoMemberRoles{}, nil, testUnexpected())
	err := outboxTenants{orgs: svc}.Each(context.Background(), func(context.Context, string) error { return errors.New("boom") })
	assert.Error(t, err)
}

func TestBuildOutboxWorker(t *testing.T) {
	svcs := &Services{
		Outbox:       outbox.NewStore(db.Pool{}, "public", ""),
		WebhookStore: webhook.NewStore(db.Pool{}, "public", ""),
		Sealer:       sealer.Disabled(),
		Orgs:         org.NewService(&fakes.OrgStore{}, fakes.Transactor{}, fakes.NoMemberRoles{}, nil, testUnexpected()),
	}
	w := buildOutboxWorker(svcs, nil)
	require.NotNil(t, w)
	assert.Equal(t, outbox.WorkerName, w.Name())
}
