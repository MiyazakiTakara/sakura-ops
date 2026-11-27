//go:build integration

package integration

import (
	"context"
	"fmt"
	"io/fs"
	"strings"
	"sync"
	"testing"
	"testing/fstest"
	"time"

	"github.com/MiyazakiTakara/sakura-ops/db/migrations"
	"github.com/MiyazakiTakara/sakura-ops/internal/migrate"
	"github.com/MiyazakiTakara/sakura-ops/internal/store"
)

func TestFreshMigrateToLatest(t *testing.T) {
	c := sharedPG
	db := c.newDB(t)
	urlFile := writeDSNFile(t, c.migratorDSN(db))
	latest := migrations.LatestVersion()

	if code, out := runMigrate(t, urlFile, "version"); code != 0 {
		t.Fatalf("version on empty db: exit %d: %s", code, out)
	} else if want := fmt.Sprintf("current 0 latest %d", latest); !strings.Contains(out, want) {
		t.Fatalf("version output %q, want %q", out, want)
	}
	if code, out := runMigrate(t, urlFile, "up"); code != 0 || !strings.Contains(out, "applied 00001") {
		t.Fatalf("up: exit %d: %s", code, out)
	}
	if code, out := runMigrate(t, urlFile, "version"); code != 0 || !strings.Contains(out, fmt.Sprintf("current %d latest %d", latest, latest)) {
		t.Fatalf("version after up: exit %d: %s", code, out)
	}
	if code, out := runMigrate(t, urlFile, "status"); code != 0 || !strings.Contains(out, "00001 applied") {
		t.Fatalf("status: exit %d: %s", code, out)
	}
	// Schema ownership: everything belongs to the migrator, nothing is in public.
	su := c.superConn(t, db)
	rows, err := su.Query(context.Background(),
		`SELECT DISTINCT tableowner FROM pg_tables WHERE schemaname = 'sakuraops'`)
	if err != nil {
		t.Fatal(err)
	}
	var owners []string
	for rows.Next() {
		var o string
		if err := rows.Scan(&o); err != nil {
			t.Fatal(err)
		}
		owners = append(owners, o)
	}
	rows.Close()
	if len(owners) != 1 || owners[0] != roleMigrator {
		t.Fatalf("table owners in sakuraops = %v, want only %s", owners, roleMigrator)
	}
	var inPublic int
	if err := su.QueryRow(context.Background(), `SELECT count(*) FROM pg_tables WHERE schemaname = 'public'`).Scan(&inPublic); err != nil || inPublic != 0 {
		t.Fatalf("tables in public = %d, %v", inPublic, err)
	}

	// The application side sees the schema through sqlc queries.
	st, _, err := store.OpenFromFile(context.Background(), writeDSNFile(t, c.appDSN(db)), store.Options{ApplicationName: "sakuraops-it"})
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if v, err := st.SchemaVersion(ctx); err != nil || v != latest {
		t.Fatalf("SchemaVersion = %d, %v", v, err)
	}
	s, err := st.GetInstanceSettings(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if s.InstanceName != "SakuraOps" || s.Timezone != "UTC" || s.Revision != 1 || s.UpdatedAt.IsZero() {
		t.Fatalf("default settings = %+v", s)
	}
}

func TestRerunIsNoop(t *testing.T) {
	c := sharedPG
	db := c.newDB(t)
	urlFile := writeDSNFile(t, c.migratorDSN(db))
	if code, o := runMigrate(t, urlFile, "up"); code != 0 {
		t.Fatalf("first up: %d %s", code, o)
	}
	su := c.superConn(t, db)
	count := func() (versions int, name string) {
		if err := su.QueryRow(context.Background(), `SELECT count(*) FROM sakuraops.goose_db_version`).Scan(&versions); err != nil {
			t.Fatal(err)
		}
		if err := su.QueryRow(context.Background(), `SELECT instance_name FROM sakuraops.instance_settings`).Scan(&name); err != nil {
			t.Fatal(err)
		}
		return
	}
	// Data changed after the first run must survive a second run.
	if _, err := su.Exec(context.Background(), `UPDATE sakuraops.instance_settings SET instance_name = 'Edited'`); err != nil {
		t.Fatal(err)
	}
	before, _ := count()
	code, o := runMigrate(t, urlFile, "up")
	if code != 0 || !strings.Contains(o, "no migrations to apply") {
		t.Fatalf("second up: exit %d: %s", code, o)
	}
	after, name := count()
	if before != after || name != "Edited" {
		t.Fatalf("rerun changed state: versions %d -> %d, name %q", before, after, name)
	}
}

// baselineOnly returns an FS with just the shipped baseline migration.
func baselineOnly(t testing.TB) fstest.MapFS {
	t.Helper()
	b, err := fs.ReadFile(migrations.FS(), "00001_baseline.sql")
	if err != nil {
		t.Fatal(err)
	}
	return fstest.MapFS{"00001_baseline.sql": {Data: b}}
}

func withFixture(t testing.TB, name, sql string) fstest.MapFS {
	m := baselineOnly(t)
	m[name] = &fstest.MapFile{Data: []byte("-- +goose Up\n" + sql + "\n")}
	return m
}

func newRunner(t testing.TB, c *cluster, db string, files fs.FS) *migrate.Runner {
	t.Helper()
	sqlDB, err := migrate.OpenDB(c.migratorDSN(db), "sakuraops-it-migrate")
	if err != nil {
		t.Fatal(err)
	}
	r, err := migrate.New(sqlDB, files)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = r.Close() })
	return r
}

// TestStepwiseUpgradeKeepsData applies the shipped baseline, writes data as the
// application role, then upgrades with a test-only fixture migration (version 2).
// Only one real migration exists yet, so the second step is a fixture; the
// mechanics under test (incremental apply on an existing database, data kept,
// version recorded) are the same as for a real 00002.
func TestStepwiseUpgradeKeepsData(t *testing.T) {
	c := sharedPG
	db := c.newDB(t)
	ctx := context.Background()

	r1 := newRunner(t, c, db, baselineOnly(t))
	if applied, err := r1.Up(ctx); err != nil || len(applied) != 1 || applied[0].Version != 1 {
		t.Fatalf("step 1: %v, %v", applied, err)
	}
	app := mustConnect(t, c.appDSN(db))
	if _, err := app.Exec(ctx, `UPDATE sakuraops.instance_settings SET instance_name = 'Kept', timezone = 'Europe/Warsaw', revision = 5`); err != nil {
		t.Fatalf("app update: %v", describe(err))
	}

	fixture := withFixture(t, "00002_fixture_note.sql",
		`ALTER TABLE sakuraops.instance_settings ADD COLUMN fixture_note text NOT NULL DEFAULT 'n/a';`)
	r2 := newRunner(t, c, db, fixture)
	applied, err := r2.Up(ctx)
	if err != nil || len(applied) != 1 || applied[0].Version != 2 {
		t.Fatalf("step 2 must apply only version 2: %v, %v", applied, err)
	}
	var name, tz, note string
	var rev int64
	if err := app.QueryRow(ctx, `SELECT instance_name, timezone, revision, fixture_note FROM sakuraops.instance_settings`).Scan(&name, &tz, &rev, &note); err != nil {
		t.Fatalf("after upgrade: %v", describe(err))
	}
	if name != "Kept" || tz != "Europe/Warsaw" || rev != 5 || note != "n/a" {
		t.Fatalf("data after upgrade: %q %q %d %q", name, tz, rev, note)
	}
	var v int64
	if err := app.QueryRow(ctx, `SELECT sakuraops.schema_version()`).Scan(&v); err != nil || v != 2 {
		t.Fatalf("schema_version = %d, %v", v, err)
	}
	if again, err := r2.Up(ctx); err != nil || len(again) != 0 {
		t.Fatalf("repeat after upgrade: %v, %v", again, err)
	}
}

// TestConcurrentMigratorsBinary starts two real migrator processes at once on a
// fresh database. Both must exit 0 and exactly one may apply the baseline.
func TestConcurrentMigratorsBinary(t *testing.T) {
	c := sharedPG
	db := c.newDB(t)
	urlFile := writeDSNFile(t, c.migratorDSN(db))

	type result struct {
		code int
		out  string
	}
	var wg sync.WaitGroup
	start := make(chan struct{})
	res := make([]result, 2)
	for i := range res {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			res[i].code, res[i].out = runMigrate(t, urlFile, "up")
		}()
	}
	close(start)
	wg.Wait()

	appliedBy := 0
	for i, r := range res {
		if r.code != 0 {
			t.Fatalf("migrator %d exit %d: %s", i, r.code, r.out)
		}
		if strings.Contains(r.out, "applied 00001") {
			appliedBy++
		}
	}
	if appliedBy != 1 {
		t.Fatalf("baseline applied by %d processes, want exactly 1: %+v", appliedBy, res)
	}
	su := c.superConn(t, db)
	var n int
	if err := su.QueryRow(context.Background(), `SELECT count(*) FROM sakuraops.goose_db_version WHERE version_id = 1 AND is_applied`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("version 1 recorded %d times, %v", n, err)
	}
}

// TestConcurrentMigratorsSerialised proves the advisory lock does the work: the
// slow fixture migration is not idempotent (ADD COLUMN), so two unserialised
// runners would fail.
func TestConcurrentMigratorsSerialised(t *testing.T) {
	c := sharedPG
	db := c.newDB(t)
	fixture := withFixture(t, "00002_slow_alter.sql",
		"SELECT pg_sleep(2);\nALTER TABLE sakuraops.instance_settings ADD COLUMN slow_note text;")

	var wg sync.WaitGroup
	start := make(chan struct{})
	errs := make([]error, 3)
	counts := make([]int, 3)
	for i := range errs {
		r := newRunner(t, c, db, fixture)
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()
			applied, err := r.Up(ctx)
			errs[i], counts[i] = err, len(applied)
		}()
	}
	close(start)
	wg.Wait()

	total := 0
	for i, err := range errs {
		if err != nil {
			t.Fatalf("runner %d: %v", i, err)
		}
		total += counts[i]
	}
	if total != 2 { // versions 1 and 2, each applied exactly once overall
		t.Fatalf("migrations applied %d times in total, want 2 (counts %v)", total, counts)
	}
}
