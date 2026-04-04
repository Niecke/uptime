package db

import (
	"database/sql"
	"embed"
	"fmt"
	"time"

	_ "modernc.org/sqlite"

	"github.com/pressly/goose/v3"

	models "niecke-it.de/uptime/internal/models"
)

//go:embed migrations/*.sql
var embedMigrations embed.FS

func SetupDatabase() *sql.DB {
	// TODO: rewrite the panics and return the error instead
	db, err := sql.Open("sqlite", "uptime.db")
	if err != nil {
		panic(err)
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
	_, err := db.Exec(`INSERT INTO check_results (endpoint_id, checked_at, status_code, duration_ms, err) 
		VALUES (?, ?, ?, ?, ?)`,
		endpointID,
		time.Now().UTC().Format("2006-01-02 15:04:05"),
		result.StatusCode,
		result.Duration.Milliseconds(),
		errorString,
	)

	if err != nil {
		return err
	}

	return nil
}

func ListEndpoints(db *sql.DB) ([]models.EndpointStatus, error) {
	result, err := db.Query(`
		SELECT e.id, e.url, cr.status_code, cr.checked_at, cr.duration_ms
		FROM endpoints e
		INNER JOIN check_results cr ON e.id = cr.endpoint_id
		WHERE (e.id, cr.checked_at) IN (
			SELECT endpoint_id, MAX(checked_at)
			FROM check_results
			GROUP BY endpoint_id
		)
		ORDER BY e.id
		
	`)

	if err != nil {
		fmt.Printf("Error while fetching endpoint list %v", err.Error())
		return nil, err
	}

	defer result.Close()

	var s []models.EndpointStatus
	for result.Next() {
		var t models.EndpointStatus
		if err := result.Scan(&t.ID, &t.URL, &t.StatusCode, &t.CheckedAt, &t.Duration); err != nil {
			fmt.Printf("Error while processing endpoint list %v", err.Error())
			return nil, err
		}
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

	resultHistory, err := db.Query(`
		SELECT cr.status_code, cr.checked_at, cr.duration_ms
		FROM check_results cr
		WHERE cr.endpoint_id = ?
			AND cr.checked_at > datetime('now', '-5 days')
	`, endpointID)

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
