package invite

import (
	"github.com/google/uuid"

	"altalune.id/othra/internal/kernel/authz"
	"altalune.id/othra/internal/org/member"
	"altalune.id/othra/scope"
)

// Authorize returns nil when a may invite someone with role, or renew, revoke or confirm an invite with role; it never touches storage.
func Authorize(a member.Actor, role member.Role) error {
	if !role.IsValid() {
		return &member.InvalidRoleError{Role: string(role)}
	}
	if a.Operator && a.UserID == uuid.Nil {
		return nil
	}
	if !scope.Grants(a.Permissions, scope.MemberWrite) {
		return &authz.ForbiddenError{Permission: scope.MemberWrite}
	}
	// SECURITY: no one grants more than they hold.
	if !scope.Covers(a.Permissions, member.BuiltinPermissions(role)) {
		return &member.RoleNotAllowedError{Role: string(role)}
	}
	return nil
}
