//go:build integration

package integration

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	insufficientPrivilege = "42501"
	checkViolation        = "23514"
	uniqueViolation       = "23505"
)

func migratedDB(t testing.TB, c *cluster) string {
	t.Helper()
	db := c.newDB(t)
	if code, out := runMigrate(t, writeDSNFile(t, c.migratorDSN(db)), "up"); code != 0 {
		t.Fatalf("migrate: exit %d: %s", code, out)
	}
	return db
}

func TestAppRoleCannotChangeSchema(t *testing.T) {
	c := sharedPG
	db := migratedDB(t, c)
	app := mustConnect(t, c.appDSN(db))
	ctx := context.Background()

	denied := []string{
		"CREATE TABLE sakuraops.evil (id int)",
		"CREATE TABLE public.evil (id int)",
		"CREATE TEMP TABLE evil (id int)",
		"CREATE SCHEMA evil",
		"CREATE FUNCTION sakuraops.evil() RETURNS int LANGUAGE sql AS 'SELECT 1'",
		"ALTER TABLE sakuraops.instance_settings ADD COLUMN evil int",
		"ALTER TABLE sakuraops.instance_settings DROP CONSTRAINT instance_settings_id_check",
		"DROP TABLE sakuraops.instance_settings",
		"TRUNCATE sakuraops.instance_settings",
		"INSERT INTO sakuraops.instance_settings (id, instance_name) VALUES (false, 'x')",
		"DELETE FROM sakuraops.instance_settings",
		"CREATE OR REPLACE FUNCTION sakuraops.schema_version() RETURNS bigint LANGUAGE sql AS 'SELECT 999::bigint'",
		"SET ROLE " + roleMigrator,
	}
	for _, q := range denied {
		if _, err := app.Exec(ctx, q); pgCode(err) != insufficientPrivilege {
			t.Errorf("app role: %q -> %v, want %s", q, describe(err), insufficientPrivilege)
		}
	}

	// The goose bookkeeping table is closed to the application role entirely.
	for _, q := range []string{
		"SELECT * FROM sakuraops.goose_db_version",
		"INSERT INTO sakuraops.goose_db_version (version_id, is_applied) VALUES (99, true)",
		"UPDATE sakuraops.goose_db_version SET is_applied = false",
		"DELETE FROM sakuraops.goose_db_version",
		"TRUNCATE sakuraops.goose_db_version",
	} {
		if _, err := app.Exec(ctx, q); pgCode(err) != insufficientPrivilege {
			t.Errorf("app role: %q -> %v, want %s", q, describe(err), insufficientPrivilege)
		}
	}

	// What it may do: read and update the settings, ask for the schema version.
	if _, err := app.Exec(ctx, "UPDATE sakuraops.instance_settings SET instance_name = 'Allowed', revision = revision + 1, updated_at = now()"); err != nil {
		t.Errorf("app UPDATE must work: %v", describe(err))
	}
	var name string
	if err := app.QueryRow(ctx, "SELECT instance_name FROM sakuraops.instance_settings").Scan(&name); err != nil || name != "Allowed" {
		t.Errorf("app SELECT: %q %v", name, describe(err))
	}
	var v int64
	if err := app.QueryRow(ctx, "SELECT sakuraops.schema_version()").Scan(&v); err != nil || v != 1 {
		t.Errorf("schema_version via app: %d %v", v, describe(err))
	}
}

func TestRoleAttributes(t *testing.T) {
	c := sharedPG
	db := migratedDB(t, c)
	su := c.superConn(t, db)
	for _, role := range []string{roleMigrator, roleApp} {
		var super, createdb, createrole, repl, bypass, login bool
		if err := su.QueryRow(context.Background(),
			`SELECT rolsuper, rolcreatedb, rolcreaterole, rolreplication, rolbypassrls, rolcanlogin FROM pg_roles WHERE rolname = $1`, role,
		).Scan(&super, &createdb, &createrole, &repl, &bypass, &login); err != nil {
			t.Fatal(err)
		}
		if super || createdb || createrole || repl || bypass || !login {
			t.Errorf("%s attributes: super=%v createdb=%v createrole=%v repl=%v bypassrls=%v login=%v", role, super, createdb, createrole, repl, bypass, login)
		}
	}
	// The migrator owns the schema but cannot create objects in public either.
	mig := mustConnect(t, c.migratorDSN(db))
	if _, err := mig.Exec(context.Background(), "CREATE TABLE public.evil (id int)"); pgCode(err) != insufficientPrivilege {
		t.Errorf("migrator CREATE in public -> %v", describe(err))
	}
	// PUBLIC has no CONNECT: a role without grants cannot enter the database.
	su2 := c.superConn(t, "postgres")
	if _, err := su2.Exec(context.Background(), "DO $$ BEGIN IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname='it_stranger') THEN CREATE ROLE it_stranger LOGIN PASSWORD 'it-stranger-pw'; END IF; END $$"); err != nil {
		t.Fatal(err)
	}
	if conn, err := pgxConnect(c.dsn("it_stranger", db, "it-stranger-pw")); pgCode(err) != insufficientPrivilege {
		if conn != nil {
			_ = conn.Close(context.Background())
		}
		t.Errorf("stranger connect -> %v, want %s", describe(err), insufficientPrivilege)
	}
}

func TestSingletonAndChecks(t *testing.T) {
	c := sharedPG
	db := migratedDB(t, c)
	mig := mustConnect(t, c.migratorDSN(db))
	ctx := context.Background()
	cases := []struct {
		name, q, code string
	}{
		{"second row with id true", "INSERT INTO sakuraops.instance_settings (instance_name) VALUES ('second')", uniqueViolation},
		{"row with id false", "INSERT INTO sakuraops.instance_settings (id, instance_name) VALUES (false, 'x')", checkViolation},
		{"update id to false", "UPDATE sakuraops.instance_settings SET id = false", checkViolation},
		{"empty name", "UPDATE sakuraops.instance_settings SET instance_name = ''", checkViolation},
		{"overlong name", fmt.Sprintf("UPDATE sakuraops.instance_settings SET instance_name = '%s'", strings.Repeat("a", 101)), checkViolation},
		{"zero revision", "UPDATE sakuraops.instance_settings SET revision = 0", checkViolation},
	}
	for _, tc := range cases {
		if _, err := mig.Exec(ctx, tc.q); pgCode(err) != tc.code {
			t.Errorf("%s: %v, want %s", tc.name, describe(err), tc.code)
		}
	}
	var n int
	if err := mig.QueryRow(ctx, "SELECT count(*) FROM sakuraops.instance_settings").Scan(&n); err != nil || n != 1 {
		t.Fatalf("row count = %d, %v", n, err)
	}
}

func TestBootstrapIdempotent(t *testing.T) {
	c := sharedPG
	db := migratedDB(t, c)
	// Re-running the bootstrap on a migrated database must not break anything.
	c.bootstrap(t, db)
	app := mustConnect(t, c.appDSN(db))
	var name string
	if err := app.QueryRow(context.Background(), "SELECT instance_name FROM sakuraops.instance_settings").Scan(&name); err != nil {
		t.Fatalf("after bootstrap rerun: %v", describe(err))
	}
}

// TestBootstrapPasswordFiles covers the variant that keeps passwords off every
// command line: roles.sql reads them from files named by environment variables.
func TestBootstrapPasswordFiles(t *testing.T) {
	c := sharedPG
	db := "it_" + randHex(6)
	if _, err := c.superConn(t, "postgres").Exec(context.Background(), "CREATE DATABASE "+db); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	write := func(name, content string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		return c.copyFile(t, p, name+"-"+randHex(3))
	}
	m, a := write("m", c.migPW+"\n"), write("a", c.appPW+"\n")
	if out, err := c.bootstrapRaw(db, "", []string{"SAKURAOPS_MIGRATOR_PASSWORD_FILE=" + m, "SAKURAOPS_APP_PASSWORD_FILE=" + a}); err != nil {
		t.Fatalf("bootstrap from files: %v\n%s", err, out)
	}
	conn := mustConnect(t, c.migratorDSN(db))
	if _, err := conn.Exec(context.Background(), "SELECT 1"); err != nil {
		t.Fatal(err)
	}

	// Without any password source the script must refuse to run (not create a
	// passwordless or empty-password role).
	empty := write("empty", "")
	if _, err := c.bootstrapRaw(db, "", []string{"SAKURAOPS_MIGRATOR_PASSWORD_FILE=" + empty, "SAKURAOPS_APP_PASSWORD_FILE=" + a}); err == nil {
		t.Fatal("bootstrap with an empty password must fail")
	}
	if _, err := c.bootstrapRaw(db, "", nil); err == nil {
		t.Fatal("bootstrap without passwords must fail")
	}
}

// TestBootstrapHardensExistingRoles runs on its own cluster because roles are
// cluster-wide and the second case deliberately leaves a forbidden membership.
func TestBootstrapHardensExistingRoles(t *testing.T) {
	c, err := startCluster()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(c.remove)
	ctx := context.Background()
	admin := c.superConn(t, "postgres")

	attrs := func(role string) (super, createdb, createrole, repl, bypass, login bool) {
		t.Helper()
		if err := admin.QueryRow(ctx,
			`SELECT rolsuper, rolcreatedb, rolcreaterole, rolreplication, rolbypassrls, rolcanlogin FROM pg_roles WHERE rolname = $1`, role,
		).Scan(&super, &createdb, &createrole, &repl, &bypass, &login); err != nil {
			t.Fatal(err)
		}
		return
	}
	newEmptyDB := func() string {
		db := "it_" + randHex(6)
		if _, err := admin.Exec(ctx, "CREATE DATABASE "+db); err != nil {
			t.Fatal(err)
		}
		return db
	}

	t.Run("pre-existing privileged roles are demoted", func(t *testing.T) {
		for _, r := range []string{roleApp, roleMigrator} {
			if _, err := admin.Exec(ctx, "CREATE ROLE "+r+" LOGIN CREATEDB CREATEROLE REPLICATION BYPASSRLS"); err != nil {
				t.Fatal(err)
			}
		}
		c.bootstrap(t, newEmptyDB())
		for _, r := range []string{roleApp, roleMigrator} {
			super, createdb, createrole, repl, bypass, login := attrs(r)
			if super || createdb || createrole || repl || bypass || !login {
				t.Errorf("%s after bootstrap: super=%v createdb=%v createrole=%v repl=%v bypassrls=%v login=%v",
					r, super, createdb, createrole, repl, bypass, login)
			}
		}
	})

	t.Run("membership in another role is refused", func(t *testing.T) {
		for name, grant := range map[string]string{
			"app in migrator":         "GRANT " + roleMigrator + " TO " + roleApp,
			"app in predefined role":  "GRANT pg_read_all_data TO " + roleApp,
			"migrator in app":         "GRANT " + roleApp + " TO " + roleMigrator,
			"app in privileged group": "",
		} {
			if grant == "" {
				if _, err := admin.Exec(ctx, "CREATE ROLE it_priv_group NOLOGIN CREATEROLE"); err != nil {
					t.Fatal(err)
				}
				grant = "GRANT it_priv_group TO " + roleApp
			}
			if _, err := admin.Exec(ctx, grant); err != nil {
				t.Fatalf("%s: %v", name, describe(err))
			}
			out, err := c.bootstrapRaw(newEmptyDB(), fmt.Sprintf("\\set migrator_password '%s'\n\\set app_password '%s'\n", c.migPW, c.appPW), nil)
			if err == nil || !strings.Contains(string(out)+err.Error(), "unexpected role membership") {
				t.Errorf("%s: bootstrap must fail with a membership error, got err=%v", name, err)
			}
			for _, leak := range []string{c.migPW, c.appPW} {
				if strings.Contains(string(out)+err.Error(), leak) {
					t.Errorf("%s: bootstrap output leaks a password", name)
				}
			}
			for _, rv := range []string{
				"REVOKE " + roleMigrator + " FROM " + roleApp,
				"REVOKE pg_read_all_data FROM " + roleApp,
				"REVOKE " + roleApp + " FROM " + roleMigrator,
				"REVOKE it_priv_group FROM " + roleApp,
			} {
				_, _ = admin.Exec(ctx, rv)
			}
		}
		// With the memberships gone the bootstrap succeeds again.
		c.bootstrap(t, newEmptyDB())
	})
}
