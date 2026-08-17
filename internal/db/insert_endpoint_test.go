package db

import (
	"path/filepath"
	"testing"
)

// InsertEndpoint used to return LastInsertId() after an INSERT OR IGNORE that
// ignored. LastInsertId then reports whatever row was last inserted on the
// connection, so every already-existing url resolved to the id of the most
// recently added endpoint — silently writing several endpoints' check results
// under one id.
func TestInsertEndpointReturnsOwnID(t *testing.T) {
	path := filepath.Join(t.TempDir(), "t.db")
	database := SetupDatabaseWithPath(path)
	defer database.Close()

	urls := []string{"https://a.example", "https://b.example", "https://c.example"}

	want := map[string]int64{}
	for _, u := range urls {
		id, err := InsertEndpoint(database, u)
		if err != nil {
			t.Fatalf("InsertEndpoint(%s): %v", u, err)
		}
		want[u] = id
	}

	// a new endpoint arrives, so the connection's last insert rowid is now its id
	if _, err := InsertEndpoint(database, "https://new.example"); err != nil {
		t.Fatalf("InsertEndpoint(new): %v", err)
	}

	// re-resolving the existing urls must still give each its own id
	for _, u := range urls {
		id, err := InsertEndpoint(database, u)
		if err != nil {
			t.Fatalf("InsertEndpoint(%s): %v", u, err)
		}
		if id != want[u] {
			t.Errorf("%s resolved to id %d, want %d", u, id, want[u])
		}
	}
}
