package db

import (
	"database/sql"
	"embed"
	"encoding/json"
	"fmt"
	"os"
	"time"

	_ "modernc.org/sqlite"

	"github.com/pressly/goose/v3"

	models "niecke-it.de/uptime/internal/models"
)

//go:embed migrations/*.sql
var embedMigrations embed.FS

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
	db.SetMaxOpenConns(1)

	goose.SetBaseFS(embedMigrations)

	if err := goose.SetDialect("sqlite3"); err != nil {
		panic(err)
	}

	if err := goose.Up(db, "migrations"); err != nil {
		panic(err)
	}

	return db
}

func CompactDatabase(database *sql.DB) {
	// TODO: add history lenght as api param
	retentionDays := "-7 days"
	ticker := time.NewTicker(30 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		// first delete old records
		if res, err := database.Exec(`
			DELETE FROM check_results
			WHERE checked_at < datetime('now', ?)
			`, retentionDays); err != nil {

			fmt.Printf("Error while deleting old records: %v", err)
		} else {
			if deletedRecords, err := res.RowsAffected(); err != nil {
				fmt.Printf("Error while getting delete records count: %v", err)
			} else {
				fmt.Printf("Deleted %v old records from check_results.", deletedRecords)
			}
		}

		// second run truncation
		if _, err := database.Exec("PRAGMA wal_checkpoint(TRUNCATE)"); err != nil {
			fmt.Printf("WAL checkpoint failed: %v", err)
		} else {
			fmt.Printf("WAL checkpoint completed")
		}
	}
}

func InsertEndpoint(db *sql.DB, url string) (int64, error) {
	var endpointID int64
	resultNew, err := db.Exec("INSERT OR IGNORE INTO endpoints (url) VALUES (?)", url)
	if err != nil {
		return 0, err
	}

	endpointID, err = resultNew.LastInsertId()
	if err != nil {
		return 0, err
	}

	if endpointID == 0 {
		resultExisting := db.QueryRow("SELECT id FROM endpoints WHERE url = ?", url)

		err := resultExisting.Scan(&endpointID)
		if err != nil {
			return 0, err
		}
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
		time.Now().UTC().Format("2006-01-02 15:04:05"),
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

func ListEndpoints(db *sql.DB) ([]models.EndpointStatus, error) {
	// TODO: add history lenght as api param
	lookbackDays := "-5 days"
	result, err := db.Query(`
		SELECT 
			e.id, e.url, 
			cr.status_code, cr.checked_at, cr.duration_ms,
			(SELECT COUNT(*) FROM check_results WHERE endpoint_id = e.id AND status_code >= 200 AND status_code < 400 AND checked_at > datetime('now', ?)) success,
			(SELECT COUNT(*) FROM check_results WHERE endpoint_id = e.id AND checked_at > datetime('now', ?)) total
		FROM endpoints e
		INNER JOIN check_results cr ON e.id = cr.endpoint_id
		WHERE (e.id, cr.checked_at) IN (
			SELECT endpoint_id, MAX(checked_at)
			FROM check_results
			GROUP BY endpoint_id
		)
		ORDER BY e.id
	`, lookbackDays, lookbackDays)

	if err != nil {
		fmt.Printf("Error while fetching endpoint list %v", err.Error())
		return nil, err
	}

	defer result.Close()

	var s []models.EndpointStatus
	for result.Next() {
		var success int64
		var total int64
		var t models.EndpointStatus
		if err := result.Scan(&t.ID, &t.URL, &t.StatusCode, &t.CheckedAt, &t.Duration, &success, &total); err != nil {
			fmt.Printf("Error while processing endpoint list %v", err.Error())
			return nil, err
		}
		uptime := float32(success) / float32(total)
		t.UptimePercentage = uptime
		s = append(s, t)
	}
	return s, nil
}

func HistoryEndpoints(db *sql.DB, endpointID int64) (models.EndpointHistory, error) {
	var endpointHistory models.EndpointHistory
	result := db.QueryRow(`
		SELECT e.id, e.url
		FROM endpoints e
		WHERE e.id = ?
	`, endpointID)

	if err := result.Scan(&endpointHistory.ID, &endpointHistory.URL); err != nil {
		if err == sql.ErrNoRows {
			fmt.Printf("Unkown endpoint id: %v", endpointID)
			return endpointHistory, models.ErrNotFound
		}
		fmt.Printf("Error while processing endpoint data %v", err)
		return endpointHistory, err
	}

	// TODO: add history lenght as api param
	lookbackDays := "-5 days"
	resultHistory, err := db.Query(`
		SELECT cr.status_code, cr.checked_at, cr.duration_ms
		FROM check_results cr
		WHERE cr.endpoint_id = ?
			AND cr.checked_at > datetime('now', ?)
	`, endpointID, lookbackDays)

	if err != nil {
		fmt.Printf("Error while fetching endpoint data %v", err.Error())
		return endpointHistory, err
	}
	defer resultHistory.Close()

	endpointHistory.History = []models.EndpointHistoryEntry{}
	for resultHistory.Next() {
		var e models.EndpointHistoryEntry
		if err := resultHistory.Scan(&e.StatusCode, &e.CheckedAt, &e.Duration); err != nil {
			fmt.Printf("Error while processing endpoint history %v", err.Error())
			return endpointHistory, err
		}
		endpointHistory.History = append(endpointHistory.History, e)
	}

	return endpointHistory, nil
}
