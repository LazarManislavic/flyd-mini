package storage

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
)


func AcquireLock(ctx context.Context, db *sql.DB, key, value string, timeout time.Duration) (bool, error) {
	deadline := time.Now().Add(timeout)
	logrus.Infof("Attempting to acquire lock: key=%s, value=%s, timeout=%s", key, value, timeout)

	for time.Now().Before(deadline) {
		_, err := db.ExecContext(ctx, `
			INSERT INTO locks (k, v) VALUES (?, ?)
			ON CONFLICT(k) DO NOTHING
		`, key, value)

		if err == nil {
			logrus.Infof("Lock acquired successfully: key=%s, value=%s", key, value)
			return true, nil // lock acquired
		}

		logrus.Infof("Lock %s is held by another process, retrying in 100ms...", key)
		time.Sleep(100 * time.Millisecond)
	}

	logrus.Errorf("Failed to acquire lock %s within %s", key, timeout)
	return false, fmt.Errorf("failed to acquire lock %s within %s", key, timeout)
}

func ReleaseLock(ctx context.Context, db *sql.DB, key, value string) error {
	
	logrus.Infof("Attempting to acquire lock: key=%s, value=%s", key, value)
	_, err := db.ExecContext(ctx, `
		DELETE FROM locks where k = ? AND v = ?
	`, key, value)

	if err == nil {
		logrus.Infof("Lock released successfully: key=%s, value=%s", key, value)
	} else {
		logrus.Errorf("Failed to release lock %s", key)
	}

	return err
}