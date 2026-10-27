package org

import (
	"context"

	"github.com/google/uuid"
)

// MemberRoles gives a person's role in an org and the permissions it grants there; ok is false when they are not a member.
type MemberRoles interface {
	Role(ctx context.Context, orgID, userID uuid.UUID) (role string, permissions []string, ok bool, err error)
}
