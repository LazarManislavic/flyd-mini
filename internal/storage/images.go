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

func InsertImage(ctx context.Context, db *sql.DB, s3Key string, digest *string, baseLvID *int64, sizeBytes int64, localPath string, etag *string) (int64, error) {
	// upsert
	res, err := db.ExecContext(ctx, `
		INSERT INTO images (s3_key, digest, base_lv_id, size_bytes, local_path, created_at, etag)
		VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP, ?)
		ON CONFLICT(s3_key) DO UPDATE SET
			digest=excluded.digest,
			base_lv_id=excluded.base_lv_id,
			size_bytes=excluded.size_bytes,
			etag=excluded.etag
		`, s3Key, digest, baseLvID, sizeBytes, localPath, etag)
	if err != nil {
		return 0, err
	}

	return res.LastInsertId()
}

func GetImageByKey(ctx context.Context, db *sql.DB, s3Key string) (*Image, error) {
	row := db.QueryRowContext(ctx,
		`SELECT id, s3_key, digest, base_lv_id, size_bytes, local_path, created_at FROM images WHERE s3_key = ?`, s3Key,
	)

	var img Image
	if err := row.Scan(&img.ID, &img.S3Key, &img.Digest, &img.BaseLvID, &img.SizeBytes, &img.LocalPath, &img.CreatedAt); err != nil {
		return nil, err
	}
	
	return &img, nil
}
