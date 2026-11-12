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
