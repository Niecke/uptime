package db

import (
	"database/sql"
	"embed"
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
