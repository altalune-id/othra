package schema

import (
	"io/fs"
	"regexp"
	"strings"
	"testing"
)

var (
	createTableRe = regexp.MustCompile(`(?is)CREATE\s+TABLE\s+\{\{\.Schema\}\}\.\{\{\.TablePrefix\}\}(\w+)\s*\((.*?)\n\);`)
	orgIDColumnRe = regexp.MustCompile(`(?m)^\s*org_id\s+UUID`)
)

// SECURITY: an inlined current_setting(...)::uuid throws 22P02 once a pooled connection has served a tenant, because a transaction-local set_config resets to empty string rather than NULL.
func TestMigrations_PoliciesUseTheGuardedScopeHelpers(t *testing.T) {
	sources := migrationSources(t)
	for _, guc := range []string{"app.current_org_id", "app.current_project_id", "app.current_user_id"} {
		helperDef := "SELECT NULLIF(current_setting('" + guc + "', true), '')::uuid"
		defs := 0
		for name, src := range sources {
			defs += strings.Count(src, helperDef)
			if inlined := strings.Count(src, "current_setting('"+guc+"'") - strings.Count(src, helperDef); inlined > 0 {
				t.Errorf("%s inlines current_setting('%s') %d time(s); call its helper instead", name, guc, inlined)
			}
		}
		if defs != 1 {
			t.Errorf("the %s helper is defined %d times, want exactly 1", guc, defs)
		}
	}
}

// SECURITY: every org-owned table forces row level security and carries a policy that checks writes too, so a query that forgets its tenant filter returns nothing.
func TestMigrations_EveryOrgOwnedTableForcesItsPolicy(t *testing.T) {
	all := strings.Join(mapValues(migrationSources(t)), "\n")
	tables := 0
	for _, m := range createTableRe.FindAllStringSubmatch(all, -1) {
		table, body := m[1], m[2]
		if !orgIDColumnRe.MatchString(body) && table != "orgs" {
			continue
		}
		tables++
		target := `{{.Schema}}.{{.TablePrefix}}` + table
		for _, want := range []string{
			"ALTER TABLE " + target + " ENABLE ROW LEVEL SECURITY;",
			"ALTER TABLE " + target + " FORCE ROW LEVEL SECURITY;",
		} {
			if !strings.Contains(all, want) {
				t.Errorf("%s: missing %q", table, want)
			}
		}
		policy := regexp.MustCompile(`(?s)CREATE POLICY \{\{\.TablePrefix\}\}` + table + `_tenant ON ` + regexp.QuoteMeta(target) + `\s+FOR ALL\s+USING \(.*?\)\s+WITH CHECK \(.*?current_org_id\(\).*?\);`)
		if !policy.MatchString(all) {
			t.Errorf("%s: missing a FOR ALL policy %s_tenant with USING and a WITH CHECK on current_org_id()", table, table)
		}
	}
	if tables < 9 {
		t.Fatalf("found %d org-owned tables, want at least 9; the guard would pass vacuously", tables)
	}
}

func migrationSources(t *testing.T) map[string]string {
	t.Helper()
	entries, err := fs.ReadDir(MigrationsFS(), ".")
	if err != nil {
		t.Fatalf("read migrations: %v", err)
	}
	out := map[string]string{}
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		body, err := fs.ReadFile(MigrationsFS(), e.Name())
		if err != nil {
			t.Fatalf("read %s: %v", e.Name(), err)
		}
		out[e.Name()] = string(body)
	}
	if len(out) == 0 {
		t.Fatal("no migrations found; the guard would pass vacuously")
	}
	return out
}

func mapValues(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for _, v := range m {
		out = append(out, v)
	}
	return out
}
