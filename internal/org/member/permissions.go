package member

import "github.com/altalune-id/othra/scope"

// BuiltinPermissions returns the permissions r grants inside its org.
func BuiltinPermissions(r Role) []string {
	switch r {
	case RoleOwner:
		return []string{"*"}
	case RoleAdmin:
		return []string{scope.OrgRead, "project:*", "webhook:*", "member:*", "apikey:*", "end_user:*"}
	case RoleMember:
		return []string{scope.OrgRead, scope.ProjectRead, "webhook:*", scope.MemberRead, scope.ApikeyRead}
	}
	return nil
}
