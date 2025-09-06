package storage

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"
)


// InitDB initializes and returns a new database connection.
// It reads the SQL schema from a given file path, creates the necessary
// directories for the database file, opens a SQLite connection, and
// applies the schema to the database.
func InitDB(scehmaPath, dbPath string) (*sql.DB, error) {
	schemaBytes, err := os.ReadFile(scehmaPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read schema file: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create db directory: %w", err)
	}

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open db: %w", err)
	}

	if _, err := db.Exec((string(schemaBytes))); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to apply schema: %w", err)
	}

	return db, nil
}