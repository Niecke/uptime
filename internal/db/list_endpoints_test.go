package db

import (
	"database/sql"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// legacyListEndpointsQuery is the ListEndpoints query as of 59432b5, before the
// latest-check part was rewritten. It is the reference the new query is
// compared against, and the baseline of the benchmark.
const legacyListEndpointsQuery = `
	WITH latest AS (
		-- one row per endpoint: SQLite takes the bare columns from the same
		-- row that produced MAX(), so ties cannot fan out into duplicates
		SELECT endpoint_id,
		       MAX(checked_at) AS checked_at,
		       status_code,
		       duration_ms
		FROM check_results
		GROUP BY endpoint_id
	),
	stats AS (
		SELECT endpoint_id,
		       COUNT(*) AS total,
		       SUM(CASE WHEN status_code >= 200 AND status_code < 400 THEN 1 ELSE 0 END) AS success
		FROM check_results
		WHERE checked_at > datetime('now', ?)
		GROUP BY endpoint_id
	)
	SELECT e.id, e.url,
	       COALESCE(l.status_code, 0),
	       l.checked_at,
	       COALESCE(l.duration_ms, 0),
	       COALESCE(s.success, 0),
	       COALESCE(s.total, 0)
	FROM endpoints e
	-- LEFT so a configured endpoint that has not been checked yet still shows up
	LEFT JOIN latest l ON l.endpoint_id = e.id
	LEFT JOIN stats  s ON s.endpoint_id = e.id
	ORDER BY e.id
	`

// seedChecks inserts n checks for an endpoint with one recursive-CTE statement,
// so 50k rows stay fast. Check k is (k*30 + offsetSeconds) seconds old, so the
// timestamps of one endpoint are distinct and use checkedAtLayout. status_code
// cycles 200/500/0 and duration_ms varies.
func seedChecks(tb testing.TB, database *sql.DB, endpointID int64, n int, offsetSeconds int) {
	tb.Helper()
	_, err := database.Exec(`+"`"+`
		WITH RECURSIVE k(n) AS (
			SELECT 0
			UNION ALL
			SELECT n + 1 FROM k WHERE n < ? - 1
		)
		INSERT INTO check_results (endpoint_id, checked_at, status_code, duration_ms)
		SELECT ?,
		       strftime('%Y-%m-%d %H:%M:%S', 'now', '-' || (n * 30 + ?) || ' seconds'),
		       CASE n % 3 WHEN 0 THEN 200 WHEN 1 THEN 500 ELSE 0 END,
		       10 + n % 97
		FROM k`+"`"+`, n, endpointID, offsetSeconds)
	if err != nil {
		tb.Fatalf("seedChecks(endpoint %d): %v", endpointID, err)
	}
}

func newTestDB(tb testing.TB) *sql.DB {
	tb.Helper()
	database := SetupDatabaseWithPath(filepath.Join(tb.TempDir(), "t.db"))
	tb.Cleanup(func() { database.Close() })
	return database
}

// runListQuery runs one of the list queries and returns every row as
// id, url, status_code, checked_at (sql.NullString), duration_ms, success, total.
func runListQuery(tb testing.TB, database *sql.DB, query string) [][]any {
	tb.Helper()
	rows, err := database.Query(query, "-30 days")
	if err != nil {
		tb.Fatalf("query: %v", err)
	}
	defer rows.Close()
	var out [][]any
	for rows.Next() {
		var id, status, duration, success, total int64
		var url string
		var checkedAt sql.NullString
		if err := rows.Scan(&id, &url, &status, &checkedAt, &duration, &success, &total); err != nil {
			tb.Fatalf("scan: %v", err)
		}
		out = append(out, []any{id, url, status, checkedAt, duration, success, total})
	}
	if err := rows.Err(); err != nil {
		tb.Fatalf("rows: %v", err)
	}
	return out
}

func TestListEndpointsMatchesLegacyQuery(t *testing.T) {
	database := newTestDB(t)

	var ids []int64
	for _, u := range []string{"https://a.example", "https://b.example", "https://c.example", "https://never.example"} {
		id, err := InsertEndpoint(database, u)
		if err != nil {
			t.Fatalf("InsertEndpoint(%s): %v", u, err)
		}
		ids = append(ids, id)
	}
	const day = 24 * 3600
	seedChecks(t, database, ids[0], 200, 0)
	seedChecks(t, database, ids[1], 50, 600)
	// all checks older than the 30 day window
	seedChecks(t, database, ids[2], 20, 40*day)
	// ids[3] has no checks at all

	want := runListQuery(t, database, legacyListEndpointsQuery)
	got := runListQuery(t, database, listEndpointsQuery)
	if len(want) != 4 {
		t.Fatalf("legacy query returned %d rows, want 4", len(want))
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("new query differs from legacy query\n got: %v\nwant: %v", got, want)
	}

	list, err := ListEndpoints(false, database, 30)
	if err != nil {
		t.Fatalf("ListEndpoints: %v", err)
	}
	if len(list) != 4 {
		t.Fatalf("ListEndpoints returned %d rows, want 4", len(list))
	}

	// endpoint without checks
	never := list[3]
	if never.StatusCode != 0 || !never.CheckedAt.IsZero() || never.UptimePercentage != 0 {
		t.Errorf("endpoint without checks = %+v, want status 0, zero CheckedAt, uptime 0", never)
	}

	// endpoint whose checks are all outside the retention window: the latest
	// check is still reported, the uptime is 0
	old := list[2]
	if old.CheckedAt.IsZero() || old.UptimePercentage != 0 {
		t.Errorf("endpoint with old checks = %+v, want a CheckedAt and uptime 0", old)
	}

	// CheckedAt must be the seeded latest timestamp (check k=0 of endpoint a)
	var latest string
	if err := database.QueryRow(`+"`"+`SELECT MAX(checked_at) FROM check_results WHERE endpoint_id = ?`+"`"+`, ids[0]).Scan(&latest); err != nil {
		t.Fatalf("latest: %v", err)
	}
	wantAt, err := time.Parse(checkedAtLayout, latest)
	if err != nil {
		t.Fatalf("parse %q: %v", latest, err)
	}
	if !list[0].CheckedAt.Equal(wantAt) {
		t.Errorf("CheckedAt = %v, want %v", list[0].CheckedAt, wantAt)
	}
	if list[0].UptimePercentage <= 0 {
		t.Errorf("uptime of endpoint with recent checks = %v, want > 0", list[0].UptimePercentage)
	}
}

// Two checks of one endpoint can share a checked_at second. The endpoint must
// still yield exactly one row, carrying the check that was written last.
func TestListEndpointsLatestTieBreak(t *testing.T) {
	database := newTestDB(t)

	id, err := InsertEndpoint(database, "https://tie.example")
	if err != nil {
		t.Fatalf("InsertEndpoint: %v", err)
	}
	for _, status := range []int{200, 500} {
		if _, err := database.Exec(`+"`"+`INSERT INTO check_results (endpoint_id, checked_at, status_code, duration_ms)
			VALUES (?, '2026-01-01 12:00:00', ?, 5)`+"`"+`, id, status); err != nil {
			t.Fatalf("insert check: %v", err)
		}
	}

	list, err := ListEndpoints(false, database, 30)
	if err != nil {
		t.Fatalf("ListEndpoints: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("got %d rows, want 1", len(list))
	}
	if list[0].StatusCode != 500 {
		t.Errorf("StatusCode = %d, want 500 (the check with the higher id)", list[0].StatusCode)
	}
}

func TestListEndpointsQueryPlan(t *testing.T) {
	database := newTestDB(t)

	rows, err := database.Query("EXPLAIN QUERY PLAN "+listEndpointsQuery, "-30 days")
	if err != nil {
		t.Fatalf("explain: %v", err)
	}
	defer rows.Close()

	var details []string
	for rows.Next() {
		var id, parent, notUsed int
		var detail string
		if err := rows.Scan(&id, &parent, &notUsed, &detail); err != nil {
			t.Fatalf("scan: %v", err)
		}
		details = append(details, detail)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows: %v", err)
	}
	plan := strings.Join(details, "\n")

	var seek, subquery bool
	scans := 0
	for _, d := range details {
		if strings.Contains(d, "USING COVERING INDEX idx_check_results_history (endpoint_id=?)") {
			seek = true
		}
		if strings.Contains(d, "CORRELATED SCALAR SUBQUERY") {
			subquery = true
		}
		if strings.HasPrefix(d, "SCAN") && strings.Contains(d, "idx_check_results_history") {
			scans++
		}
	}
	if !seek {
		t.Errorf("no endpoint_id=? search on the covering index in plan:\n%s", plan)
	}
	if !subquery {
		t.Errorf("no correlated scalar subquery in plan:\n%s", plan)
	}
	// the only full index scan left is the 30-day stats CTE (the old query had two)
	if scans != 1 {
		t.Errorf("got %d scans of idx_check_results_history, want 1:\n%s", scans, plan)
	}
}

func BenchmarkListEndpoints(b *testing.B) {
	database := newTestDB(b)
	for _, u := range []string{"https://a.example", "https://b.example"} {
		id, err := InsertEndpoint(database, u)
		if err != nil {
			b.Fatalf("InsertEndpoint(%s): %v", u, err)
		}
		seedChecks(b, database, id, 50000, 0)
	}

	for _, bc := range []struct{ name, query string }{
		{"new", listEndpointsQuery},
		{"legacy", legacyListEndpointsQuery},
	} {
		b.Run(bc.name, func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				runListQuery(b, database, bc.query)
			}
		})
	}
}
