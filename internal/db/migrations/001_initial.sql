-- +goose Up
CREATE TABLE endpoints (
    id INTEGER PRIMARY KEY,
    url TEXT NOT NULL
);

CREATE TABLE check_results (
    id INTEGER PRIMARY KEY,
    endpoint_id INT NOT NULL,
    checked_at DATETIME NOT NULL,
    status_code INT,
    duration_ms INT,
    err TEXT,
    FOREIGN KEY(endpoint_id) REFERENCES endpoints(id) ON DELETE CASCADE
);

-- +goose Down
DROP TABLE check_results;
DROP TABLE endpoints;
