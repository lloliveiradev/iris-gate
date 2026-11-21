package pgstore_test

import (
	"io/fs"
	"os"
	"strings"
	"testing"
)

// TestMigration0003_StatesDestructiveRekey guards the contract of the D2
// migration without a database: it must say it deletes rows, and it must key
// the table by (merchant_id, scope, key). The behavior against Postgres is
// covered by the integration test.
func TestMigration0003_StatesDestructiveRekey(t *testing.T) {
	t.Parallel()
	body, err := fs.ReadFile(os.DirFS("migrations"), "0003_idempotency_per_merchant.sql")
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	sql := string(body)

	for _, want := range []string{
		"DESTRUCTIVE",
		"DELETE FROM idempotency_records;",
		"PRIMARY KEY (merchant_id, scope, key)",
		"merchant_id UUID NOT NULL REFERENCES merchants",
	} {
		if !strings.Contains(sql, want) {
			t.Errorf("migration lacks %q", want)
		}
	}
	if strings.Index(sql, "DELETE FROM idempotency_records;") > strings.Index(sql, "ADD COLUMN merchant_id") {
		t.Error("rows must be deleted before the NOT NULL column is added")
	}
}

// TestMigration0004_VersionAndFailureCode guards the D3/D4 migration without
// a database: existing rows get version 0, and a failure code is only allowed
// on a failed intent. The behavior against Postgres is covered by the
// integration test.
func TestMigration0004_VersionAndFailureCode(t *testing.T) {
	t.Parallel()
	body, err := fs.ReadFile(os.DirFS("migrations"), "0004_payment_intent_version_and_failure.sql")
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	sql := string(body)

	for _, want := range []string{
		"ADD COLUMN version INT NOT NULL DEFAULT 0",
		"ADD COLUMN failure_code TEXT;",
		"CHECK (failure_code IS NULL OR status = 'failed')",
	} {
		if !strings.Contains(sql, want) {
			t.Errorf("migration lacks %q", want)
		}
	}
	if strings.Contains(sql, "DELETE FROM") || strings.Contains(sql, "DROP ") {
		t.Error("0004 must not delete rows or drop objects")
	}
}

// TestMigration0005_EnablesForcedRLSOnMerchantTables guards the RLS migration
// without a database: both merchant-scoped tables get enabled and forced RLS
// with a policy for USING and WITH CHECK, and the outbox stays unscoped.
func TestMigration0005_EnablesForcedRLSOnMerchantTables(t *testing.T) {
	t.Parallel()
	body, err := fs.ReadFile(os.DirFS("migrations"), "0005_rls_payments.sql")
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	sql := string(body)

	for _, table := range []string{"payment_intents", "idempotency_records"} {
		for _, want := range []string{
			"ALTER TABLE " + table + " ENABLE ROW LEVEL SECURITY;",
			"ALTER TABLE " + table + " FORCE ROW LEVEL SECURITY;",
			"CREATE POLICY " + table + "_merchant_isolation ON " + table,
		} {
			if !strings.Contains(sql, want) {
				t.Errorf("migration lacks %q", want)
			}
		}
	}
	const predicate = "merchant_id = NULLIF(current_setting('app.merchant_id', true), '')::uuid"
	if strings.Count(sql, predicate) != 4 {
		t.Errorf("want the tenant predicate in USING and WITH CHECK of both policies, got %d", strings.Count(sql, predicate))
	}
	if strings.Contains(sql, "ALTER TABLE outbox_messages") {
		t.Error("outbox_messages has no merchant_id and must not get RLS")
	}
}
