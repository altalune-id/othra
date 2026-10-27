package boot

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	noopmetric "go.opentelemetry.io/otel/metric/noop"

	"altalune.id/othra/apperror"
	"altalune.id/othra/config"
	"altalune.id/othra/internal/kernel"
	"altalune.id/othra/internal/kernel/session"
	"altalune.id/othra/internal/kernel/tenant"
	"altalune.id/othra/internal/keys"
	"altalune.id/othra/internal/org"
	"altalune.id/othra/internal/project"
	"altalune.id/othra/internal/testutil/fakes"
	"altalune.id/othra/sealer"
)

func TestKeysConfig(t *testing.T) {
	assert.Equal(t, keys.Config{RSABits: 3072, PrePublish: 10 * time.Minute, Overlap: 24 * time.Hour},
		keysConfig(config.KeysConfig{RSABits: 3072, Rotation: config.KeysRotationConfig{PrePublish: 10 * time.Minute, Overlap: 24 * time.Hour}}))
}

func TestKeysConfig_DefaultsToA2048BitKey(t *testing.T) {
	assert.Equal(t, 2048, keysConfig(config.KeysConfig{}).RSABits, "a config literal without keys.rsaBits still generates a valid key")
}

func TestKeyProjects_ScopesEachProjectOfEachOrg(t *testing.T) {
	a, b := uuid.New(), uuid.New()
	oa, err := org.NewOrg("acme", "Acme", uuid.Nil)
	require.NoError(t, err)
	oa.ID = a
	ob, err := org.NewOrg("beta", "Beta", uuid.Nil)
	require.NoError(t, err)
	ob.ID = b
	p1, err := project.NewProject(a, "web", "Web", uuid.Nil)
	require.NoError(t, err)
	orgs := org.NewService(&fakes.OrgStore{Orgs: []*org.Org{oa, ob}}, fakes.Transactor{}, fakes.NoMemberRoles{}, nil, nil)
	projects := project.NewService(&fakes.ProjectStore{Projects: []*project.Project{p1}}, fakes.Transactor{}, fakes.NoMemberships{}, nil, "", nil)

	var seen []tenant.Context
	require.NoError(t, projectTenants{orgs: orgs, projects: projects}.Each(context.Background(), func(ctx context.Context, orgID, projectID uuid.UUID) error {
		tc := tenant.Peek(ctx)
		assert.Equal(t, orgID, tc.OrgID)
		assert.Equal(t, projectID, tc.ProjectID)
		seen = append(seen, tc)
		return nil
	}))
	assert.Len(t, seen, 2, "the fake lists the same project for each org")
}

func TestBuildScheduler_RegistersTheKeyJobs(t *testing.T) {
	k := kernel.Kernel{Meter: noopmetric.NewMeterProvider().Meter("test"), Reporter: apperror.NewReporter(nil, false)}
	svcs := &Services{
		Sessions: session.NewMemoryStore(),
		Keys:     buildKeys(k, config.Config{}, fakes.Transactor{}, sealer.Disabled()),
	}
	r, err := buildScheduler(k, config.Config{Scheduler: config.SchedulerConfig{Enabled: true}}, svcs, testLogger())
	require.NoError(t, err)
	var names []string
	for _, j := range r.Jobs() {
		names = append(names, j.Name)
	}
	assert.ElementsMatch(t, []string{"session-sweep", "keys-promote", "keys-prune"}, names)
}
