package keys

import (
	"context"
	"log/slog"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"

	keysv1 "github.com/altalune-id/othra/gen/go/keys/v1"
	keysv1connect "github.com/altalune-id/othra/gen/go/keys/v1/keysv1connect"
	"github.com/altalune-id/othra/internal/kernel/authz"
	"github.com/altalune-id/othra/internal/kernel/session"
)

// SigningKeyService is the SigningKeyService Connect handler.
type SigningKeyService struct {
	keysv1connect.UnimplementedSigningKeyServiceHandler
	keys *Service
	refs Refs
}

var _ keysv1connect.SigningKeyServiceHandler = (*SigningKeyService)(nil)

// NewControlPlane builds the SigningKeyService handler.
func NewControlPlane(keys *Service, refs Refs) *SigningKeyService {
	return &SigningKeyService{keys: keys, refs: refs}
}

// ListSigningKeys lists a project's keys, newest first.
func (s *SigningKeyService) ListSigningKeys(
	ctx context.Context, req *connect.Request[keysv1.ListSigningKeysRequest],
) (*connect.Response[keysv1.ListSigningKeysResponse], error) {
	orgID, projectID, err := s.project(ctx, req.Msg.GetProjectId(), authz.ReachProject)
	if err != nil {
		return nil, err
	}
	keys, err := s.keys.List(ctx, orgID, projectID)
	if err != nil {
		return nil, err
	}
	out := make([]*keysv1.SigningKey, 0, len(keys))
	for _, k := range keys {
		out = append(out, toProto(k))
	}
	return connect.NewResponse(&keysv1.ListSigningKeysResponse{Keys: out}), nil
}

// RotateSigningKey publishes a new key for a project, or promotes it at once when immediate.
func (s *SigningKeyService) RotateSigningKey(
	ctx context.Context, req *connect.Request[keysv1.RotateSigningKeyRequest],
) (*connect.Response[keysv1.RotateSigningKeyResponse], error) {
	orgID, projectID, err := s.project(ctx, req.Msg.GetProjectId(), authz.ReachWholeProject)
	if err != nil {
		return nil, err
	}
	k, err := s.keys.Rotate(ctx, orgID, projectID, req.Msg.GetImmediate())
	if err != nil {
		return nil, err
	}
	s.keys.log.LogAttrs(ctx, slog.LevelInfo, "keys: rotated",
		slog.String("project", req.Msg.GetProjectId()), slog.String("kid", k.KID), slog.String("key", k.PublicID),
		slog.Bool("immediate", req.Msg.GetImmediate()), slog.String("status", string(k.Status)),
		slog.String("actor", string(session.PrincipalFrom(ctx).Source)))
	return connect.NewResponse(&keysv1.RotateSigningKeyResponse{Key: toProto(k)}), nil
}

// SECURITY: an unknown project and a project outside the caller's reach answer the same not-found.
func (s *SigningKeyService) project(
	ctx context.Context, publicID string, reach func(ctx context.Context, orgID, projectID uuid.UUID) bool,
) (orgID, projectID uuid.UUID, err error) {
	projectID, orgID, ok, err := s.refs.ProjectID(ctx, publicID)
	if err != nil {
		return uuid.Nil, uuid.Nil, err
	}
	if !ok || !reach(ctx, orgID, projectID) {
		return uuid.Nil, uuid.Nil, &ProjectNotFoundError{}
	}
	return orgID, projectID, nil
}

func toProto(k *Key) *keysv1.SigningKey {
	return &keysv1.SigningKey{
		PublicId:    k.PublicID,
		Kid:         k.KID,
		Alg:         k.Alg,
		Status:      statusToProto(k.Status),
		CreatedAt:   timestamppb.New(k.CreatedAt),
		ActivatesAt: ts(activatesWhileNext(k)),
		ActivatedAt: ts(k.ActivatedAt),
		RetireAfter: ts(k.RetireAfter),
	}
}

func activatesWhileNext(k *Key) *time.Time {
	if k.Status != StatusNext {
		return nil
	}
	return k.ActivatesAt
}

func statusToProto(s Status) keysv1.SigningKeyStatus {
	switch s {
	case StatusNext:
		return keysv1.SigningKeyStatus_SIGNING_KEY_STATUS_NEXT
	case StatusActive:
		return keysv1.SigningKeyStatus_SIGNING_KEY_STATUS_ACTIVE
	case StatusRetiring:
		return keysv1.SigningKeyStatus_SIGNING_KEY_STATUS_RETIRING
	case StatusRetired:
		return keysv1.SigningKeyStatus_SIGNING_KEY_STATUS_RETIRED
	}
	return keysv1.SigningKeyStatus_SIGNING_KEY_STATUS_UNSPECIFIED
}

func ts(t *time.Time) *timestamppb.Timestamp {
	if t == nil {
		return nil
	}
	return timestamppb.New(*t)
}
