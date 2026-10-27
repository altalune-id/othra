package tenant

import (
	"context"

	"github.com/google/uuid"

	"altalune.id/othra/apperror"
)

// ProjectResolver finds the org a project belongs to.
type ProjectResolver interface {
	OrgOf(ctx context.Context, projectID uuid.UUID) (uuid.UUID, error)
}

// RequireProjectInOrg fails unless projectID belongs to orgID; nil r skips it. SECURITY: a missing project gives the same error.
func RequireProjectInOrg(ctx context.Context, r ProjectResolver, orgID, projectID uuid.UUID) error {
	if r == nil {
		return nil
	}
	owner, err := r.OrgOf(ctx, projectID)
	if ae, ok := apperror.AsAppError(err); ok && apperror.KindOf(ae.Code()) == apperror.KindNotFound {
		return &ProjectOutsideOrgError{}
	}
	if err != nil {
		return err
	}
	if owner != orgID {
		return &ProjectOutsideOrgError{}
	}
	return nil
}
