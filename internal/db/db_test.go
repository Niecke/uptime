package db

import (
	"testing"
)

func TestInsertEndpoint(t *testing.T) {
	database := SetupDatabaseWithPath(":memory:")
	url := "https://test.local"
	endpointIdFirst, err := InsertEndpoint(database, url)
	if err != nil {
		t.Errorf("error while inserting data 1")
	}

	if endpointIdFirst <= 0 {
		t.Errorf("endpointId should be greater 0")
	}

	endpointIdSecond, err := InsertEndpoint(database, url)
	if err != nil {
		t.Errorf("error while inserting data 2")
	}

	if endpointIdFirst != endpointIdSecond {
		t.Errorf("EndpointIDs should be the same but were %d and %d", endpointIdFirst, endpointIdSecond)
	}

}
