package pgstore_test

import (
	"io/fs"
	"os"
	"strings"
	"testing"
)

// Phase 08 identity migrations (plan section 1 reserves 0006 to 0009).
const (
	migration0006 = "0006_identity_users_and_tokens.sql"
	migration0007 = "0007_identity_memberships_invitations_staff.sql"
	migration0008 = "0008_identity_api_keys_kinds.sql"
	migration0009 = "0009_identity_audit_events.sql"
)

var phase08Migrations = []string{migration0006, migration0007, migration0008, migration0009}

func readMigration(t *testing.T, name string) string {
	t.Helper()
	body, err := fs.ReadFile(os.DirFS("migrations"), name)
	if err != nil {
		t.Fatalf("read migration %s: %v", name, err)
	}
	return string(body)
}

// statements drops comment lines and blank lines, so what is left is the SQL
// that Postgres runs.
func statements(sql string) []string {
	var lines []string
	for line := range strings.Lines(sql) {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "--") {
			continue
		}
		lines = append(lines, trimmed)
	}
	return lines
}

// TestPhase08Migrations_RunInOneTransaction guards the plan rule without a
// database: MigrateEmbedded sends each file in a single Exec, so a file with
// several statements wraps them in BEGIN and COMMIT.
func TestPhase08Migrations_RunInOneTransaction(t *testing.T) {
	t.Parallel()
	for _, name := range phase08Migrations {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			lines := statements(readMigration(t, name))
			if len(lines) == 0 {
				t.Fatal("migration has no statements")
			}
			if first := lines[0]; first != "BEGIN;" {
				t.Errorf("first statement = %q, want BEGIN;", first)
			}
			if last := lines[len(lines)-1]; last != "COMMIT;" {
				t.Errorf("last statement = %q, want COMMIT;", last)
			}
		})
	}
}

// TestPhase08Migrations_AddNoRowLevelSecurity guards Option B (plan section
// 2): identity tables are looked up before a merchant is known, so no phase 08
// migration enables row-level security or adds a policy.
func TestPhase08Migrations_AddNoRowLevelSecurity(t *testing.T) {
	t.Parallel()
	for _, name := range phase08Migrations {
		sql := readMigration(t, name)
		for _, forbidden := range []string{"ROW LEVEL SECURITY", "CREATE POLICY"} {
			if strings.Contains(sql, forbidden) {
				t.Errorf("%s contains %q", name, forbidden)
			}
		}
	}
}

// TestMigration0008_BackfillsBeforeRequiringKindAndName checks that existing
// keys get kind 'secret' and name 'legacy' before the columns become NOT NULL,
// and that hash is required for every kind except publishable.
func TestMigration0008_BackfillsBeforeRequiringKindAndName(t *testing.T) {
	t.Parallel()
	sql := readMigration(t, migration0008)

	const backfill = "UPDATE api_keys SET kind = 'secret', name = 'legacy' WHERE kind IS NULL;"
	for _, want := range []string{
		backfill,
		"ALTER COLUMN kind SET NOT NULL",
		"ALTER COLUMN name SET NOT NULL",
		"ALTER COLUMN hash DROP NOT NULL",
		"CHECK (kind IN ('secret', 'publishable', 'restricted'))",
		"CHECK ((kind = 'publishable' AND hash IS NULL)",
		"OR (kind IN ('secret', 'restricted') AND hash IS NOT NULL))",
	} {
		if !strings.Contains(sql, want) {
			t.Errorf("migration lacks %q", want)
		}
	}
	if strings.Index(sql, backfill) > strings.Index(sql, "ALTER COLUMN kind SET NOT NULL") {
		t.Error("existing rows must be backfilled before kind becomes NOT NULL")
	}
	if strings.Contains(sql, "DELETE FROM") || strings.Contains(sql, "DROP COLUMN") {
		t.Error("0008 must not delete rows or drop columns")
	}
}

// TestMigration0009_MakesAuditEventsAppendOnly checks that the migration that
// creates audit_events revokes UPDATE and DELETE from the service roles and
// grants nothing back.
func TestMigration0009_MakesAuditEventsAppendOnly(t *testing.T) {
	t.Parallel()
	sql := readMigration(t, migration0009)

	const revoke = "REVOKE UPDATE, DELETE ON audit_events FROM iris_app, iris_worker;"
	if !strings.Contains(sql, "CREATE TABLE audit_events") {
		t.Error("migration does not create audit_events")
	}
	if !strings.Contains(sql, revoke) {
		t.Errorf("migration lacks %q", revoke)
	}
	if strings.Contains(sql, "GRANT ") {
		t.Error("0009 must not grant any privilege")
	}
}
