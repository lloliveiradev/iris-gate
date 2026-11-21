//go:build integration

// Integration tests for the database roles that
// infra/docker/postgres/initdb/10-roles.sh creates (phase 07, acceptance
// criteria 1 and 2) and for the append-only audit_events table (phase 08,
// acceptance criterion 9). They need the Compose Postgres initialized by that
// script on an empty volume and migrated with `make migrate`, and the values
// of infra/docker/.env in the environment:
//
//	set -a && . infra/docker/.env && set +a
//	make -C apps/api test-int
//
// Without those variables the tests skip.
package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

const (
	// testHost is where Compose publishes Postgres, on POSTGRES_PORT.
	testHost = "localhost"
	// sqlStateInsufficientPrivilege is the SQLSTATE of every denial asserted here.
	sqlStateInsufficientPrivilege = "42501"
)

// Role names, as 10-roles.sh creates them.
const (
	irisMigrator      = "iris_migrator"
	irisApp           = "iris_app"
	irisWorker        = "iris_worker"
	irisPlatform      = "iris_platform"
	antifraudMigrator = "antifraud_migrator"
	antifraudApp      = "antifraud_app"
)

// service names the database a role belongs to.
type service int

const (
	serviceIris service = iota
	serviceAntifraud
)

// roleSpec is one role of 10-roles.sh: where its password comes from, the one
// database it may connect to, and whether it bypasses row-level security.
type roleSpec struct {
	name        string
	passwordEnv string
	service     service
	bypassRLS   bool
}

var roleSpecs = []roleSpec{
	{name: irisMigrator, passwordEnv: "IRIS_MIGRATOR_PASSWORD", service: serviceIris},
	{name: irisApp, passwordEnv: "IRIS_APP_PASSWORD", service: serviceIris},
	{name: irisWorker, passwordEnv: "IRIS_WORKER_PASSWORD", service: serviceIris, bypassRLS: true},
	{name: irisPlatform, passwordEnv: "IRIS_PLATFORM_PASSWORD", service: serviceIris, bypassRLS: true},
	{name: antifraudMigrator, passwordEnv: "ANTIFRAUD_MIGRATOR_PASSWORD", service: serviceAntifraud},
	{name: antifraudApp, passwordEnv: "ANTIFRAUD_APP_PASSWORD", service: serviceAntifraud},
}

// stack is the Compose Postgres as infra/docker/.env describes it.
type stack struct {
	port        string
	irisDB      string
	antifraudDB string
	passwords   map[string]string
}

// loadStack reads infra/docker/.env from the environment, or skips the test.
func loadStack(t *testing.T) stack {
	t.Helper()
	var missing []string
	lookup := func(name string) string {
		value := os.Getenv(name)
		if value == "" {
			missing = append(missing, name)
		}
		return value
	}
	s := stack{
		port:        lookup("POSTGRES_PORT"),
		irisDB:      lookup("POSTGRES_DB"),
		antifraudDB: lookup("ANTI_FRAUD_DB"),
		passwords:   make(map[string]string, len(roleSpecs)),
	}
	for _, r := range roleSpecs {
		s.passwords[r.name] = lookup(r.passwordEnv)
	}
	if len(missing) > 0 {
		t.Skipf("export infra/docker/.env first (set -a && . infra/docker/.env && set +a); missing: %s",
			strings.Join(missing, ", "))
	}
	return s
}

func (s stack) database(svc service) string {
	if svc == serviceAntifraud {
		return s.antifraudDB
	}
	return s.irisDB
}

// url builds the connection URL of role on database. It carries a password:
// never log it.
func (s stack) url(role, database string) string {
	u := url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(role, s.passwords[role]),
		Host:     net.JoinHostPort(testHost, s.port),
		Path:     "/" + database,
		RawQuery: "sslmode=disable",
	}
	return u.String()
}

func (s stack) connect(t *testing.T, role, database string) *pgx.Conn {
	t.Helper()
	conn, err := pgx.Connect(context.Background(), s.url(role, database))
	if err != nil {
		t.Fatalf("connect as %s to %s: %v", role, database, err)
	}
	t.Cleanup(func() { _ = conn.Close(context.Background()) })
	return conn
}

// createProbeTable creates a table as migrator, the way a migration would, and
// drops it when the test ends. Its bigserial id makes an INSERT use a sequence.
func createProbeTable(t *testing.T, s stack, migrator, database string) string {
	t.Helper()
	ctx := context.Background()
	conn := s.connect(t, migrator, database)
	table := pgx.Identifier{fmt.Sprintf("roles_probe_%d", time.Now().UnixNano())}.Sanitize()
	if _, err := conn.Exec(ctx, `CREATE TABLE `+table+` (id bigserial PRIMARY KEY, note text)`); err != nil {
		t.Fatalf("create probe table as %s: %v", migrator, err)
	}
	t.Cleanup(func() {
		if _, err := conn.Exec(ctx, `DROP TABLE IF EXISTS `+table); err != nil {
			t.Errorf("drop probe table: %v", err)
		}
	})
	return table
}

func requireInsufficientPrivilege(t *testing.T, what string, err error) {
	t.Helper()
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != sqlStateInsufficientPrivilege {
		t.Errorf("%s: want SQLSTATE %s (insufficient_privilege), got %v", what, sqlStateInsufficientPrivilege, err)
	}
}

// TestRoles_HaveNoAdministrativeAttributes checks that no role is a superuser
// or can create roles or databases, and that only the worker and platform
// roles bypass row-level security.
func TestRoles_HaveNoAdministrativeAttributes(t *testing.T) {
	s := loadStack(t)
	ctx := context.Background()
	conn := s.connect(t, irisMigrator, s.irisDB)

	for _, r := range roleSpecs {
		t.Run(r.name, func(t *testing.T) {
			var super, createRole, createDB, replication, bypassRLS, canLogin bool
			err := conn.QueryRow(ctx,
				`SELECT rolsuper, rolcreaterole, rolcreatedb, rolreplication, rolbypassrls, rolcanlogin
				   FROM pg_roles WHERE rolname = $1`, r.name).
				Scan(&super, &createRole, &createDB, &replication, &bypassRLS, &canLogin)
			if err != nil {
				t.Fatalf("read role: %v", err)
			}
			if super || createRole || createDB || replication {
				t.Errorf("superuser=%t createrole=%t createdb=%t replication=%t, want all false",
					super, createRole, createDB, replication)
			}
			if bypassRLS != r.bypassRLS {
				t.Errorf("bypassrls = %t, want %t", bypassRLS, r.bypassRLS)
			}
			if !canLogin {
				t.Error("login = false, want true")
			}
		})
	}
}

// TestRoles_ConnectOnlyToTheirOwnDatabase covers acceptance criterion 1: a
// role connects to its own service's database only.
func TestRoles_ConnectOnlyToTheirOwnDatabase(t *testing.T) {
	s := loadStack(t)
	ctx := context.Background()

	for _, r := range roleSpecs {
		for _, svc := range []service{serviceIris, serviceAntifraud} {
			database := s.database(svc)
			t.Run(r.name+"_to_"+database, func(t *testing.T) {
				conn, err := pgx.Connect(ctx, s.url(r.name, database))
				if conn != nil {
					defer func() { _ = conn.Close(ctx) }()
				}
				if svc == r.service {
					if err != nil {
						t.Fatalf("connect to own database: %v", err)
					}
					return
				}
				requireInsufficientPrivilege(t, "connect to "+database, err)
			})
		}
	}
}

// TestRoles_AppRolesCannotRunDDL covers acceptance criterion 2: the app roles
// cannot create, alter or drop anything, including a table the migrator
// created and they can write to.
func TestRoles_AppRolesCannotRunDDL(t *testing.T) {
	s := loadStack(t)
	ctx := context.Background()

	for _, tt := range []struct{ app, migrator, database string }{
		{app: irisApp, migrator: irisMigrator, database: s.irisDB},
		{app: antifraudApp, migrator: antifraudMigrator, database: s.antifraudDB},
	} {
		t.Run(tt.app, func(t *testing.T) {
			table := createProbeTable(t, s, tt.migrator, tt.database)
			conn := s.connect(t, tt.app, tt.database)

			for _, stmt := range []string{
				`CREATE TABLE roles_probe_by_app (id int)`,
				`CREATE TEMPORARY TABLE roles_probe_temp (id int)`,
				`CREATE SCHEMA roles_probe_schema`,
				`CREATE FUNCTION roles_probe_fn() RETURNS int LANGUAGE sql AS 'SELECT 1'`,
				`ALTER TABLE ` + table + ` ADD COLUMN extra int`,
				`CREATE INDEX ON ` + table + ` (note)`,
				`TRUNCATE ` + table,
				`DROP TABLE ` + table,
				`CREATE ROLE roles_probe_role`,
				`CREATE DATABASE roles_probe_db`,
			} {
				_, err := conn.Exec(ctx, stmt)
				requireInsufficientPrivilege(t, stmt, err)
			}
		})
	}
}

// TestRoles_TablePrivilegesFollowTheDefaults checks that the default
// privileges give each role what to-be section 9.2 says on every table the
// migrator creates (the phase 10 anti-fraud tables included), and nothing on
// the migration bookkeeping tables.
func TestRoles_TablePrivilegesFollowTheDefaults(t *testing.T) {
	s := loadStack(t)
	ctx := context.Background()
	irisTable := createProbeTable(t, s, irisMigrator, s.irisDB)
	antifraudTable := createProbeTable(t, s, antifraudMigrator, s.antifraudDB)

	dml := func(table string) []string {
		return []string{
			`INSERT INTO ` + table + ` (note) VALUES ('probe')`,
			`SELECT count(*) FROM ` + table,
			`UPDATE ` + table + ` SET note = 'updated'`,
			`DELETE FROM ` + table,
		}
	}
	const (
		irisBookkeeping      = `SELECT count(*) FROM schema_migrations`
		irisBookkeepingWrite = `INSERT INTO schema_migrations (name) VALUES ('roles_probe')`
		// Exists once `make migrate` has run `prisma migrate deploy`.
		antifraudBookkeeping = `SELECT count(*) FROM "_prisma_migrations"`
	)

	tests := []struct {
		role, database string
		allowed        []string
		denied         []string
	}{
		{
			role: irisApp, database: s.irisDB,
			allowed: dml(irisTable),
			denied:  []string{irisBookkeeping, irisBookkeepingWrite},
		},
		{
			role: irisWorker, database: s.irisDB,
			allowed: dml(irisTable),
			denied:  []string{irisBookkeeping, irisBookkeepingWrite},
		},
		{
			role: irisPlatform, database: s.irisDB,
			allowed: []string{`SELECT count(*) FROM ` + irisTable},
			denied: slices.Concat(
				slices.DeleteFunc(dml(irisTable), func(stmt string) bool { return strings.HasPrefix(stmt, "SELECT") }),
				[]string{irisBookkeeping, irisBookkeepingWrite},
			),
		},
		{
			role: antifraudApp, database: s.antifraudDB,
			allowed: dml(antifraudTable),
			denied:  []string{antifraudBookkeeping},
		},
	}
	for _, tt := range tests {
		t.Run(tt.role, func(t *testing.T) {
			conn := s.connect(t, tt.role, tt.database)
			for _, stmt := range tt.allowed {
				if _, err := conn.Exec(ctx, stmt); err != nil {
					t.Errorf("%s: %v", stmt, err)
				}
			}
			for _, stmt := range tt.denied {
				_, err := conn.Exec(ctx, stmt)
				requireInsufficientPrivilege(t, stmt, err)
			}
		})
	}
}

// execRolledBack runs stmt in a transaction that it always rolls back.
func execRolledBack(ctx context.Context, conn *pgx.Conn, stmt string) error {
	tx, err := conn.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	_, err = tx.Exec(ctx, stmt)
	return err
}

// TestRoles_AuditEventsAreAppendOnly covers phase 08 acceptance criterion 9:
// migration 0009 revokes UPDATE and DELETE on audit_events from iris_app and
// iris_worker, which keep INSERT and SELECT (the id sequence included), and
// iris_platform only reads. BYPASSRLS does not give table privileges back.
// Every statement is rolled back, so the test leaves no row in the
// append-only table and changes nothing if a revoke ever regresses.
func TestRoles_AuditEventsAreAppendOnly(t *testing.T) {
	s := loadStack(t)
	ctx := context.Background()

	const (
		insertAuditEvent = `INSERT INTO audit_events
			(action, entity_type, entity_id, actor_type, actor_id, request_id, prev_hash, hash)
			VALUES ('roles_probe', 'roles_probe', 'roles_probe', 'system', 'roles_probe', 'roles_probe',
			        decode('00', 'hex'), decode('00', 'hex'))`
		selectAuditEvents = `SELECT count(*) FROM audit_events`
		updateAuditEvents = `UPDATE audit_events SET action = action`
		deleteAuditEvents = `DELETE FROM audit_events`
	)
	appendOnly := struct{ allowed, denied []string }{
		allowed: []string{insertAuditEvent, selectAuditEvents},
		denied:  []string{updateAuditEvents, deleteAuditEvents},
	}

	tests := []struct {
		role    string
		allowed []string
		denied  []string
	}{
		{role: irisApp, allowed: appendOnly.allowed, denied: appendOnly.denied},
		{role: irisWorker, allowed: appendOnly.allowed, denied: appendOnly.denied},
		{
			role:    irisPlatform,
			allowed: []string{selectAuditEvents},
			denied:  []string{insertAuditEvent, updateAuditEvents, deleteAuditEvents},
		},
	}
	for _, tt := range tests {
		t.Run(tt.role, func(t *testing.T) {
			conn := s.connect(t, tt.role, s.irisDB)
			for _, stmt := range tt.allowed {
				if err := execRolledBack(ctx, conn, stmt); err != nil {
					t.Errorf("%s: %v", stmt, err)
				}
			}
			for _, stmt := range tt.denied {
				requireInsufficientPrivilege(t, stmt, execRolledBack(ctx, conn, stmt))
			}
		})
	}
}
