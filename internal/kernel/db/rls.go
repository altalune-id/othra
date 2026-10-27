package db

import (
	"fmt"
	"strings"

	"altalune.id/othra/pg"
)

// RLSConfig tunes the boot row-level-security guard.
type RLSConfig struct {
	AllowBypass bool
}

// CheckRLS refuses a pool that row-level security would not hold; AllowBypass turns only a bypassing role into a warning.
func CheckRLS(pool string, r pg.RLSReport, cfg RLSConfig) (warn string, err error) {
	roles := roleReasons(r)
	reasons := schemaReasons(r)
	if len(roles) > 0 {
		if cfg.AllowBypass {
			warn = fmt.Sprintf("db: the %s pool bypasses row-level security (%s); "+
				"tenant isolation is NOT enforced because db.rls.allowBypass is set", pool, strings.Join(roles, "; "))
		} else {
			reasons = append(roles, reasons...)
		}
	}
	if len(reasons) > 0 {
		return "", &RLSError{Pool: pool, Reasons: reasons}
	}
	return warn, nil
}

func roleReasons(r pg.RLSReport) []string {
	const fix = "log in as the service role (scripts/db/provision.sh)"
	var out []string
	for _, rr := range r.Reach {
		switch {
		case rr.Super:
			out = append(out, fmt.Sprintf("role %q is a superuser: %s", rr.Role, fix))
		case rr.BypassRLS:
			out = append(out, fmt.Sprintf("role %q has BYPASSRLS: %s", rr.Role, fix))
		case rr.OwnsSchema:
			out = append(out, fmt.Sprintf("role %q owns the schema: %s", rr.Role, fix))
		case rr.OwnsTable:
			out = append(out, fmt.Sprintf("role %q owns tables: %s", rr.Role, fix))
		}
	}
	return out
}

func schemaReasons(r pg.RLSReport) []string {
	var out []string
	if r.RowSecurity == "off" {
		out = append(out, "row_security is off: remove the row_security setting from the role or database")
	}
	for _, t := range r.Tables {
		const fix = "fix it in the table's create migration"
		switch {
		case !t.Enabled:
			out = append(out, fmt.Sprintf("table %s: ROW LEVEL SECURITY is not enabled: %s", t.Name, fix))
		case !t.Force:
			out = append(out, fmt.Sprintf("table %s: FORCE ROW LEVEL SECURITY is missing: %s", t.Name, fix))
		case !t.ScopedRead:
			out = append(out, fmt.Sprintf("table %s: no policy reads the tenant scope: %s", t.Name, fix))
		case !t.WriteCovered:
			out = append(out, fmt.Sprintf("table %s: no policy covers writes: %s", t.Name, fix))
		case t.Unscoped:
			out = append(out, fmt.Sprintf("table %s: a policy does not read the tenant scope: %s", t.Name, fix))
		}
	}
	for _, d := range r.Definers {
		if !d.OwnedBySchemaOwner {
			out = append(out, fmt.Sprintf("function %s: not owned by the schema owner: create it in a migration", d.Signature))
		}
		if d.PublicExecute {
			out = append(out, fmt.Sprintf("function %s: PUBLIC may execute it: REVOKE ALL ON FUNCTION … FROM PUBLIC", d.Signature))
		}
	}
	return out
}
