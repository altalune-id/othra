package keys

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	keysv1 "altalune.id/othra/gen/go/keys/v1"
	"altalune.id/othra/internal/kernel/session"
	"altalune.id/othra/internal/testutil/fakes"
)

type cpFixture struct {
	h          *harness
	cp         *SigningKeyService
	projectPub string
	foreignPub string
	member     context.Context
}

func newCPFixture(t *testing.T) *cpFixture {
	t.Helper()
	h := newHarness(t)
	refs := fakes.NewPublicIDs()
	refs.Org(h.org)
	f := &cpFixture{h: h, cp: NewControlPlane(h.svc, refs), projectPub: refs.Project(h.project, h.org)}
	foreignOrg := uuid.New()
	refs.Org(foreignOrg)
	f.foreignPub = refs.Project(uuid.New(), foreignOrg)
	f.member = session.PrincipalInto(context.Background(), session.Principal{
		Source: session.SourceLocal, UserID: uuid.New(), ActiveOrgID: h.org, Scopes: []string{"*"},
	})
	h.generate(t)
	return f
}

func TestListSigningKeys(t *testing.T) {
	f := newCPFixture(t)
	resp, err := f.cp.ListSigningKeys(f.member, connect.NewRequest(&keysv1.ListSigningKeysRequest{ProjectId: f.projectPub}))
	require.NoError(t, err)
	require.Len(t, resp.Msg.GetKeys(), 1)
	k := resp.Msg.GetKeys()[0]
	assert.Equal(t, keysv1.SigningKeyStatus_SIGNING_KEY_STATUS_ACTIVE, k.GetStatus())
	assert.Equal(t, "RS256", k.GetAlg())
	assert.Len(t, k.GetPublicId(), publicIDLen)
	assert.NotNil(t, k.GetActivatedAt())
	assert.Nil(t, k.GetActivatesAt(), "only a next key has activates_at")
}

func TestRotateSigningKey(t *testing.T) {
	f := newCPFixture(t)
	resp, err := f.cp.RotateSigningKey(f.member, connect.NewRequest(&keysv1.RotateSigningKeyRequest{ProjectId: f.projectPub}))
	require.NoError(t, err)
	k := resp.Msg.GetKey()
	assert.Equal(t, keysv1.SigningKeyStatus_SIGNING_KEY_STATUS_NEXT, k.GetStatus())
	assert.Equal(t, f.h.now.Add(f.h.svc.cfg.PrePublish), k.GetActivatesAt().AsTime())

	_, err = f.cp.RotateSigningKey(f.member, connect.NewRequest(&keysv1.RotateSigningKeyRequest{ProjectId: f.projectPub}))
	assert.True(t, IsRotationPendingError(err), "%v", err)

	resp, err = f.cp.RotateSigningKey(f.member, connect.NewRequest(&keysv1.RotateSigningKeyRequest{ProjectId: f.projectPub, Immediate: true}))
	require.NoError(t, err)
	assert.Equal(t, keysv1.SigningKeyStatus_SIGNING_KEY_STATUS_ACTIVE, resp.Msg.GetKey().GetStatus())

	list, err := f.cp.ListSigningKeys(f.member, connect.NewRequest(&keysv1.ListSigningKeysRequest{ProjectId: f.projectPub}))
	require.NoError(t, err)
	statuses := map[keysv1.SigningKeyStatus]int{}
	for _, k := range list.Msg.GetKeys() {
		statuses[k.GetStatus()]++
		if k.GetStatus() == keysv1.SigningKeyStatus_SIGNING_KEY_STATUS_RETIRING {
			assert.NotNil(t, k.GetRetireAfter())
		}
	}
	assert.Equal(t, map[keysv1.SigningKeyStatus]int{
		keysv1.SigningKeyStatus_SIGNING_KEY_STATUS_ACTIVE: 1, keysv1.SigningKeyStatus_SIGNING_KEY_STATUS_RETIRING: 1,
	}, statuses)
}

// SECURITY: an unknown project, a UUID, and a project of another org all answer the same not-found, and nothing rotates.
func TestSigningKeys_OutOfReachIsNotFound(t *testing.T) {
	f := newCPFixture(t)
	key := session.PrincipalInto(context.Background(), session.Principal{Source: session.SourceAPIKey, ActiveOrgID: f.h.org, AllProjects: true})
	limited := session.PrincipalInto(context.Background(), session.Principal{Source: session.SourceAPIKey, ActiveOrgID: f.h.org, ProjectIDs: []uuid.UUID{uuid.New()}})
	for name, ctx := range map[string]context.Context{"member": f.member, "org key": key, "key for another project": limited} {
		for _, id := range []string{f.foreignPub, "abcdefghjkmnpqrstuvwx", f.h.project.String()} {
			if name == "key for another project" && id == f.foreignPub {
				continue
			}
			_, err := f.cp.ListSigningKeys(ctx, connect.NewRequest(&keysv1.ListSigningKeysRequest{ProjectId: id}))
			assert.True(t, IsProjectNotFoundError(err), "%s list %s: %v", name, id, err)
			_, err = f.cp.RotateSigningKey(ctx, connect.NewRequest(&keysv1.RotateSigningKeyRequest{ProjectId: id}))
			assert.True(t, IsProjectNotFoundError(err), "%s rotate %s: %v", name, id, err)
		}
		_, err := f.cp.ListSigningKeys(limited, connect.NewRequest(&keysv1.ListSigningKeysRequest{ProjectId: f.projectPub}))
		assert.True(t, IsProjectNotFoundError(err), "a key limited to another project: %v", err)
	}
	assert.Equal(t, map[Status]int{StatusActive: 1}, f.h.store.status(f.h.project), "nothing rotated")
}

// R-6: every rotation and promotion leaves one structured line, never key material.
func TestRotateAndPromote_LogOneLineEach(t *testing.T) {
	f := newCPFixture(t)
	var buf bytes.Buffer
	f.h.svc.WithLogger(slog.New(slog.NewJSONHandler(&buf, nil)))

	resp, err := f.cp.RotateSigningKey(f.member, connect.NewRequest(&keysv1.RotateSigningKeyRequest{ProjectId: f.projectPub}))
	require.NoError(t, err)
	line := buf.String()
	assert.Equal(t, 1, strings.Count(line, "\n"), line)
	assert.Contains(t, line, `"msg":"keys: rotated"`)
	assert.Contains(t, line, `"project":"`+f.projectPub+`"`)
	assert.Contains(t, line, `"kid":"`+resp.Msg.GetKey().GetKid()+`"`)
	assert.Contains(t, line, `"immediate":false`)
	assert.Contains(t, line, `"actor":"local"`)

	buf.Reset()
	f.h.now = f.h.now.Add(f.h.svc.cfg.PrePublish)
	ok, err := f.h.svc.Promote(context.Background(), f.h.org, f.h.project)
	require.NoError(t, err)
	require.True(t, ok)
	line = buf.String()
	assert.Equal(t, 1, strings.Count(line, "\n"), line)
	assert.Contains(t, line, `"msg":"keys: promoted"`)
	assert.Contains(t, line, `"kid":"`+resp.Msg.GetKey().GetKid()+`"`)
	assert.NotContains(t, line, "PRIVATE")
}

func TestToProto_MapsEveryStatus(t *testing.T) {
	for s, want := range map[Status]keysv1.SigningKeyStatus{
		StatusNext:     keysv1.SigningKeyStatus_SIGNING_KEY_STATUS_NEXT,
		StatusActive:   keysv1.SigningKeyStatus_SIGNING_KEY_STATUS_ACTIVE,
		StatusRetiring: keysv1.SigningKeyStatus_SIGNING_KEY_STATUS_RETIRING,
		StatusRetired:  keysv1.SigningKeyStatus_SIGNING_KEY_STATUS_RETIRED,
		"bogus":        keysv1.SigningKeyStatus_SIGNING_KEY_STATUS_UNSPECIFIED,
	} {
		assert.Equal(t, want, statusToProto(s))
	}
}
