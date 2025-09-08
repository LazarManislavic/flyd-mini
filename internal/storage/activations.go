package storage

import (
	"context"
	"database/sql"
)

type Activation struct {
	ID         int64
	ImageID    int64
	SnapLvID   int64
	MountPath  string
	ActivatedAt string
}

// InsertActivation inserts a new activation row
func InsertActivation(ctx context.Context, db *sql.DB, imageID, snapLvID int64, mountPath string) (int64, error) {
	res, err := db.ExecContext(ctx, `
		INSERT INTO activations (image_id, snap_lv_id, mount_path, activated_at)
		VALUES (?, ?, ?, CURRENT_TIMESTAMP)
	`, imageID, snapLvID, mountPath)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// GetActivationBySnapLvID retrieves activation by snap_lv_id
func GetActivationBySnapLvID(ctx context.Context, db *sql.DB, snapLvID int64) (*Activation, error) {
	row := db.QueryRowContext(ctx,
		`SELECT id, image_id, snap_lv_id, mount_path, activated_at FROM activations WHERE snap_lv_id = ?`, snapLvID,
	)
	var a Activation
	if err := row.Scan(&a.ID, &a.ImageID, &a.SnapLvID, &a.MountPath, &a.ActivatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &a, nil
}