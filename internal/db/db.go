package db

import (
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

	goose.SetBaseFS(embedMigrations)

	if err := goose.SetDialect("sqlite3"); err != nil {
		panic(err)
	}

	if err := goose.Up(db, "migrations"); err != nil {
		panic(err)
	}

	return db
}

func CompactDatabase(database *sql.DB, retentionDays int) {
	retentionDaysString := fmt.Sprintf("-%d days", retentionDays)
	ticker := time.NewTicker(30 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		// first delete old records
		if res, err := database.Exec(`
			DELETE FROM check_results
			WHERE checked_at < datetime('now', ?)
			`, retentionDaysString); err != nil {

			slog.Error("Error while deleting old records", "error", err)
		} else {
			if deletedRecords, err := res.RowsAffected(); err != nil {
				slog.Error("Error while getting delete records count", "error", err)
			} else {
				slog.Info("Deleted old records from check_results", "count", deletedRecords)
			}
		}

		// second run truncation
		if _, err := database.Exec("PRAGMA wal_checkpoint(TRUNCATE)"); err != nil {
			slog.Error("WAL checkpoint failed", "error", err)
		} else {
			slog.Error("WAL checkpoint completed")
		}
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
