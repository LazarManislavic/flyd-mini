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
	// 1. Create parent directory if it doesn't exist
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create db directory: %w", err)
	}

	// 2. Check if DB file already exists
	if _, err := os.Stat(dbPath); err == nil {
		logrus.Infof("Database already exists at %s", dbPath)

		// Database exists, just open and return
		db, err := sql.Open("sqlite3", dbPath)
		if err != nil {
			return nil, fmt.Errorf("failed to open existing db: %w", err)
		}
		return db, nil
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to stat db file: %w", err)
	}

	// 3. Read schema file
	schemaBytes, err := os.ReadFile(schemaPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read schema file: %w", err)
	}

	// 4. Open new SQLite connection
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open db: %w", err)
	}

	// 5. Apply schema
	if _, err := db.Exec(string(schemaBytes)); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to apply schema: %w", err)
	}

	return db, nil
}
