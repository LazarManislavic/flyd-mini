package storage

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	"github.com/sirupsen/logrus"

	_ "github.com/mattn/go-sqlite3"
)

func InitDB(schemaPath, dbPath string) (*sql.DB, error) {
	// Ensure parent directory exists
	parentDir := filepath.Dir(dbPath)
	if err := os.MkdirAll(parentDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create db directory %s: %w", parentDir, err)
	}

	// Check if DB file already exists
	if _, err := os.Stat(dbPath); err == nil {
		logrus.Infof("Database already exists at %s", dbPath)

		// Database exists, just open and return
		db, err := sql.Open("sqlite3", dbPath)
		if err != nil {
			return nil, fmt.Errorf("failed to open existing db: %w", err)
		}
		return db, nil
	} else if !os.IsNotExist(err) {
		// If error is not "file not exists", surface it
		return nil, fmt.Errorf("failed to stat db file %s: %w", dbPath, err)
	}

	// Read schema file
	schemaBytes, err := os.ReadFile(schemaPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read schema file: %w", err)
	}

	// Open new SQLite connection
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open db: %w", err)
	}

	// Apply schema
	if _, err := db.Exec(string(schemaBytes)); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to apply schema: %w", err)
	}

	logrus.Infof("Initialized new database at %s", dbPath)
	return db, nil
}

// EnsureDir ensures that a given directory path exists.
func EnsureDir(path string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err := os.MkdirAll(path, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", path, err)
		}
	}
	return nil
}