package db

import (
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
	"time"

	models "niecke-it.de/uptime/internal/models"
)

func pragmaInt(t *testing.T, database *sql.DB, name string) int64 {
	t.Helper()
	var v int64
	if err := database.QueryRow("PRAGMA " + name).Scan(&v); err != nil {
		t.Fatalf("PRAGMA %s: %v", name, err)
	}
	return v
}

func TestNewDatabaseUsesIncrementalAutoVacuum(t *testing.T) {
	database := SetupDatabaseWithPath(filepath.Join(t.TempDir(), "t.db"))
	defer database.Close()

	if got := pragmaInt(t, database, "auto_vacuum"); got != 2 {
		t.Fatalf("auto_vacuum = %d, want 2", got)
	}
}

func TestExistingDatabaseIsConvertedOnce(t *testing.T) {
	path := filepath.Join(t.TempDir(), "t.db")

	legacy, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	legacy.SetMaxOpenConns(1)
	if _, err := legacy.Exec("PRAGMA auto_vacuum = NONE"); err != nil {
		t.Fatal(err)
	}
	if _, err := legacy.Exec("CREATE TABLE legacy(x)"); err != nil {
		t.Fatal(err)
	}
	if got := pragmaInt(t, legacy, "auto_vacuum"); got != 0 {
		t.Fatalf("legacy auto_vacuum = %d, want 0", got)
	}
	legacy.Close()

	database, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	converted, err := ensureIncrementalAutoVacuum(database)
	if err != nil {
		t.Fatal(err)
	}
	if !converted {
		t.Fatal("first call: converted = false, want true")
	}
	if got := pragmaInt(t, database, "auto_vacuum"); got != 2 {
		t.Fatalf("auto_vacuum = %d, want 2", got)
	}

	converted, err = ensureIncrementalAutoVacuum(database)
	if err != nil {
		t.Fatal(err)
	}
	if converted {
		t.Fatal("second call: converted = true, want false")
	}
	database.Close()

	again := SetupDatabaseWithPath(path)
	defer again.Close()
	if got := pragmaInt(t, again, "auto_vacuum"); got != 2 {
		t.Fatalf("auto_vacuum after setup = %d, want 2", got)
	}
}

func TestRetentionShrinksFreelist(t *testing.T) {
	database := SetupDatabaseWithPath(filepath.Join(t.TempDir(), "t.db"))
	defer database.Close()

	id, err := InsertEndpoint(database, "https://a.example")
	if err != nil {
		t.Fatal(err)
	}

	const old = 5000
	tx, err := database.Begin()
	if err != nil {
		t.Fatal(err)
	}
	headers := strings.Repeat("x", 1000)
	for i := 0; i < old; i++ {
		if _, err := tx.Exec(`INSERT INTO check_results (endpoint_id, checked_at, status_code, duration_ms, headers, err)
			VALUES (?, datetime('now','-40 days'), 200, 10, ?, '')`, id, headers); err != nil {
			t.Fatal(err)
		}
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		if err := InsertCheckResult(database, id, models.HealthResult{StatusCode: 200, Duration: time.Millisecond}); err != nil {
			t.Fatal(err)
		}
	}
	pagesBefore := pragmaInt(t, database, "page_count")

	stats, err := runRetention(database, 30)
	if err != nil {
		t.Fatal(err)
	}
	if stats.Deleted != old {
		t.Fatalf("Deleted = %d, want %d", stats.Deleted, old)
	}
	if stats.FreelistBefore <= 0 {
		t.Fatalf("FreelistBefore = %d, want > 0", stats.FreelistBefore)
	}
	if stats.FreelistAfter >= stats.FreelistBefore {
		t.Fatalf("FreelistAfter = %d, not below FreelistBefore = %d", stats.FreelistAfter, stats.FreelistBefore)
	}
	if stats.PagesFreed <= 1 {
		t.Fatalf("PagesFreed = %d, want > 1", stats.PagesFreed)
	}
	if got := pragmaInt(t, database, "page_count"); got >= pagesBefore {
		t.Fatalf("page_count = %d, want < %d", got, pagesBefore)
	}
	if stats.SizeBytes <= 0 {
		t.Fatalf("SizeBytes = %d, want > 0", stats.SizeBytes)
	}

	var left int
	if err := database.QueryRow("SELECT COUNT(*) FROM check_results").Scan(&left); err != nil {
		t.Fatal(err)
	}
	if left != 3 {
		t.Fatalf("remaining rows = %d, want 3", left)
	}
}
