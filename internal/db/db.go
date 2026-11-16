package db

import (
	"context"
	"database/sql"
	"embed"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"time"

	_ "modernc.org/sqlite"

	"github.com/pressly/goose/v3"

	models "niecke-it.de/uptime/internal/models"
)

//go:embed migrations/*.sql
var embedMigrations embed.FS

// Layout every checked_at is written with and read back by. Values are stored
// as UTC text, so time.Parse yields UTC without an explicit location.
const checkedAtLayout = "2006-01-02 15:04:05"

func SetupDatabase() *sql.DB {
	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "uptime.db"
	}
	return SetupDatabaseWithPath(dbPath)
}

func SetupDatabaseWithPath(path string) *sql.DB {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		panic(err)
	}

	// ensure SQLite does not produce any locking issues
	db.Exec("PRAGMA journal_mode=WAL")
	db.Exec("PRAGMA busy_timeout=5000") // wait rather than fail on lock contention
	db.SetMaxOpenConns(4)

	// must happen before the migrations so their tables are created in an
	// incremental database; an existing file is converted with one VACUUM
	start := time.Now()
	converted, err := ensureIncrementalAutoVacuum(db)
	if err != nil {
		panic(err)
	}
	if converted {
		slog.Info("Converted database to incremental auto_vacuum", "duration_ms", time.Since(start).Milliseconds())
	}

	goose.SetBaseFS(embedMigrations)

	if err := goose.SetDialect("sqlite3"); err != nil {
		panic(err)
	}

	if err := goose.Up(db, "migrations"); err != nil {
		panic(err)
	}

	return db
}

const autoVacuumIncremental = 2

// incrementalVacuumPages bounds how many free pages one retention run hands
// back to the filesystem (about 8 MB at 4 KiB pages). Whatever is left is
// picked up by the next run. It must stay > 0: incremental_vacuum(0) frees
// the whole freelist.
const incrementalVacuumPages = 2000

// ensureIncrementalAutoVacuum switches the database to auto_vacuum=INCREMENTAL.
// For a file that is not yet incremental this needs one full VACUUM. PRAGMA
// state is per connection and VACUUM cannot run inside a transaction, so all
// statements run on one pinned connection. It returns true if it converted.
func ensureIncrementalAutoVacuum(db *sql.DB) (converted bool, err error) {
	ctx := context.Background()
	conn, err := db.Conn(ctx)
	if err != nil {
		return false, fmt.Errorf("get connection: %w", err)
	}
	defer conn.Close()

	var mode int
	if err := conn.QueryRowContext(ctx, "PRAGMA auto_vacuum").Scan(&mode); err != nil {
		return false, fmt.Errorf("read auto_vacuum: %w", err)
	}
	if mode == autoVacuumIncremental {
		return false, nil
	}

	if _, err := conn.ExecContext(ctx, "PRAGMA auto_vacuum = INCREMENTAL"); err != nil {
		return false, fmt.Errorf("set auto_vacuum: %w", err)
	}
	if _, err := conn.ExecContext(ctx, "VACUUM"); err != nil {
		return false, fmt.Errorf("vacuum: %w", err)
	}
	if err := conn.QueryRowContext(ctx, "PRAGMA auto_vacuum").Scan(&mode); err != nil {
		return false, fmt.Errorf("read auto_vacuum: %w", err)
	}
	if mode != autoVacuumIncremental {
		return false, fmt.Errorf("auto_vacuum is %d after VACUUM, want %d", mode, autoVacuumIncremental)
	}
	return true, nil
}

type retentionStats struct {
	Deleted        int64
	FreelistBefore int64 // after the DELETE, before the incremental vacuum
	FreelistAfter  int64
	PagesFreed     int64
	SizeBytes      int64 // page_count * page_size after the vacuum
}

// runRetention deletes old check results and gives the freed pages back.
func runRetention(database *sql.DB, retentionDays int) (retentionStats, error) {
	var stats retentionStats
	retentionDaysString := fmt.Sprintf("-%d days", retentionDays)

	res, err := database.Exec(`
		DELETE FROM check_results
		WHERE checked_at < datetime('now', ?)
		`, retentionDaysString)
	if err != nil {
		return stats, fmt.Errorf("delete old records: %w", err)
	}
	if stats.Deleted, err = res.RowsAffected(); err != nil {
		return stats, fmt.Errorf("count deleted records: %w", err)
	}

	ctx := context.Background()
	conn, err := database.Conn(ctx)
	if err != nil {
		return stats, fmt.Errorf("get connection: %w", err)
	}
	defer conn.Close()

	if err := conn.QueryRowContext(ctx, "PRAGMA freelist_count").Scan(&stats.FreelistBefore); err != nil {
		return stats, fmt.Errorf("read freelist_count: %w", err)
	}

	// Query and drain instead of Exec: the pragma works one page per step and
	// the driver must be stepped until it is done.
	rows, err := conn.QueryContext(ctx, fmt.Sprintf("PRAGMA incremental_vacuum(%d)", incrementalVacuumPages))
	if err != nil {
		return stats, fmt.Errorf("incremental vacuum: %w", err)
	}
	for rows.Next() {
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return stats, fmt.Errorf("incremental vacuum: %w", err)
	}
	if err := rows.Close(); err != nil {
		return stats, fmt.Errorf("incremental vacuum: %w", err)
	}

	var pageCount, pageSize int64
	if err := conn.QueryRowContext(ctx, "PRAGMA freelist_count").Scan(&stats.FreelistAfter); err != nil {
		return stats, fmt.Errorf("read freelist_count: %w", err)
	}
	if err := conn.QueryRowContext(ctx, "PRAGMA page_count").Scan(&pageCount); err != nil {
		return stats, fmt.Errorf("read page_count: %w", err)
	}
	if err := conn.QueryRowContext(ctx, "PRAGMA page_size").Scan(&pageSize); err != nil {
		return stats, fmt.Errorf("read page_size: %w", err)
	}
	stats.PagesFreed = stats.FreelistBefore - stats.FreelistAfter
	stats.SizeBytes = pageCount * pageSize

	// last, so the truncation reaches the main file
	if _, err := conn.ExecContext(ctx, "PRAGMA wal_checkpoint(TRUNCATE)"); err != nil {
		slog.Error("WAL checkpoint failed", "error", err)
	} else {
		slog.Error("WAL checkpoint completed")
	}

	return stats, nil
}

func CompactDatabase(database *sql.DB, retentionDays int) {
	ticker := time.NewTicker(30 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		stats, err := runRetention(database, retentionDays)
		if err != nil {
			slog.Error("Error during retention cleanup", "error", err)
			continue
		}
		slog.Info("Retention cleanup finished",
			"deleted", stats.Deleted,
			"pages_freed", stats.PagesFreed,
			"freelist_pages", stats.FreelistAfter,
			"db_size_bytes", stats.SizeBytes)
	}
}

func InsertEndpoint(db *sql.DB, url string) (int64, error) {
	if _, err := db.Exec("INSERT OR IGNORE INTO endpoints (url) VALUES (?)", url); err != nil {
		return 0, err
	}

	// Always read the id back rather than trusting LastInsertId: when OR IGNORE
	// skips the insert, LastInsertId still reports the connection's previous
	// insert rowid, which would silently map this url onto another endpoint.
	var endpointID int64
	if err := db.QueryRow("SELECT id FROM endpoints WHERE url = ?", url).Scan(&endpointID); err != nil {
		return 0, err
	}

	return endpointID, nil
}

func InsertCheckResult(db *sql.DB, endpointID int64, result models.HealthResult) error {
	errorString := ""
	if result.Err != nil {
		errorString = result.Err.Error()
	}

	// TODO: refactor?
	var headersValue any
	if len(result.Headers) > 0 {
		b, err := json.Marshal(result.Headers)
		if err != nil {
			return fmt.Errorf("marshal headers: %w", err)
		}
		headersValue = string(b)
	} else {
		headersValue = nil // store SQL NULL, not "{}" or ""
	}

	_, err := db.Exec(`INSERT INTO check_results (endpoint_id, checked_at, status_code, duration_ms, headers, err) 
		VALUES (?, ?, ?, ?, ?, ?)`,
		endpointID,
		time.Now().UTC().Format(checkedAtLayout),
		result.StatusCode,
		result.Duration.Milliseconds(),
		headersValue,
		errorString,
	)

	if err != nil {
		return err
	}

	return nil
}

func ListEndpoints(tracingLog bool, db *sql.DB, retentionDays int) ([]models.EndpointStatus, error) {
	retentionDaysString := fmt.Sprintf("-%d days", retentionDays)
	slog.Debug("ListEndpoints called", "retentionDays", retentionDays, "retentionDaysString", retentionDaysString)
	tracing_start := time.Now()
	result, err := db.Query(`
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
	`, retentionDaysString)

	tracing_end := time.Now()
	tracing_duration := tracing_end.Sub(tracing_start)
	if tracingLog {
		slog.Info("Tracing duration of the SQL.", "duration_ms", tracing_duration.Milliseconds(), "tracing_type", "sql", "command", "ListEndpoints")
	}
	if err != nil {
		slog.Error("Error while fetching endpoint list", "error", err.Error())
		return nil, err
	}

	defer result.Close()

	var s []models.EndpointStatus
	for result.Next() {
		var success int64
		var total int64
		// MAX(checked_at) comes out of the CTE without the column's declared
		// DATETIME type, so the driver hands it over as a string and it has to
		// be parsed here. NULL means the endpoint has never been checked.
		var checkedAt sql.NullString
		var t models.EndpointStatus
		if err := result.Scan(&t.ID, &t.URL, &t.StatusCode, &checkedAt, &t.Duration, &success, &total); err != nil {
			slog.Error("Error while processing endpoint list", "error", err.Error())
			return nil, err
		}
		if checkedAt.Valid {
			parsed, err := time.Parse(checkedAtLayout, checkedAt.String)
			if err != nil {
				slog.Error("Unparsable checked_at", "endpoint_id", t.ID, "value", checkedAt.String, "error", err)
			} else {
				t.CheckedAt = parsed
			}
		}
		// guard the division: an endpoint with no results in the retention window
		// would otherwise yield NaN, which encoding/json refuses to marshal
		if total > 0 {
			t.UptimePercentage = float32(success) / float32(total)
		}
		s = append(s, t)
	}
	return s, nil
}

func HistoryEndpoints(tracingLog bool, db *sql.DB, endpointID int64) (models.EndpointHistory, error) {
	var endpointHistory models.EndpointHistory
	tracing_start := time.Now()
	result := db.QueryRow(`
		SELECT e.id, e.url
		FROM endpoints e
		WHERE e.id = ?
	`, endpointID)
	tracing_end := time.Now()
	tracing_duration := tracing_end.Sub(tracing_start)
	if tracingLog {
		slog.Info("Tracing duration of the SQL.", "duration_ms", tracing_duration.Milliseconds(), "tracing_type", "sql", "command", "HistoryEndpoints_1")
	}

	if err := result.Scan(&endpointHistory.ID, &endpointHistory.URL); err != nil {
		if err == sql.ErrNoRows {
			slog.Error("Unkown endpoint id", "endpoint_id", endpointID)
			return endpointHistory, models.ErrNotFound
		}
		slog.Error("Error while processing endpoint data", "error", err)
		return endpointHistory, err
	}

	// TODO: add history lenght as api param
	lookbackDays := "-5 days"
	tracing_start = time.Now()
	resultHistory, err := db.Query(`
		SELECT strftime('%Y-%m-%dT%H', cr.checked_at) AS hour,
       		COUNT(*) AS total,
       		SUM(CASE WHEN cr.status_code = 0 OR cr.status_code >= 400 THEN 1 ELSE 0 END) AS failures,
       		CAST(AVG(cr.duration_ms) AS INTEGER) AS avg_duration_ms
		FROM check_results cr
		WHERE cr.endpoint_id = ?
			AND cr.checked_at > datetime('now', ?)
		GROUP BY hour
		ORDER BY hour
	`, endpointID, lookbackDays)
	tracing_end = time.Now()
	tracing_duration = tracing_end.Sub(tracing_start)
	if tracingLog {
		slog.Info("Tracing duration of the SQL.", "duration_ms", tracing_duration.Milliseconds(), "tracing_type", "sql", "command", "HistoryEndpoints_2")
	}

	if err != nil {
		slog.Error("Error while fetching endpoint data", "error", err.Error())
		return endpointHistory, err
	}
	defer resultHistory.Close()

	endpointHistory.History = []models.EndpointHistoryBucket{}
	for resultHistory.Next() {
		var b models.EndpointHistoryBucket
		if err := resultHistory.Scan(&b.Hour, &b.Total, &b.Failures, &b.AvgDuration); err != nil {
			slog.Error("Error while processing endpoint history", "error", err.Error())
			return endpointHistory, err
		}
		endpointHistory.History = append(endpointHistory.History, b)
	}

	return endpointHistory, nil
}
