package storage

import (
	"context"
	"database/sql"
)

// A row in the `images` table
type Image struct {
	ID        int64
	S3Key     string
	Digest    sql.NullString
	BaseLvID  sql.NullInt64
	SizeBytes sql.NullInt64
	LocalPath string
	CreatedAt string
}

func InsertImage(ctx context.Context, db *sql.DB, digest *string, baseLvID *int64, sizeBytes int64, localPath string) (int64, error) {
	// upsert
	res, err := db.ExecContext(ctx, `
		INSERT INTO images (digest, base_lv_id, size_bytes, local_path, created_at)
		VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)
		`,  digest, baseLvID, sizeBytes, localPath)
	if err != nil {
		return 0, err
	}

	return res.LastInsertId()
}

func GetImageByKey(ctx context.Context, db *sql.DB, s3Key string) (*Image, error) {
	row := db.QueryRowContext(ctx,
		`SELECT id, digest, base_lv_id, size_bytes, local_path, created_at FROM images WHERE s3_key = ?`, s3Key,
	)

	var img Image
	if err := row.Scan(&img.ID, &img.S3Key, &img.Digest, &img.BaseLvID, &img.SizeBytes, &img.LocalPath, &img.CreatedAt); err != nil {
		return nil, err
	}
	
	return &img, nil
}
