package storage

import (
	"context"
	"database/sql"
)

type Blob struct {
	Digest    string
	LocalPath string
	SizeBytes int64
	Etag      string
	CreatedAt string
}


func InsertBlob(ctx context.Context, db *sql.DB, digest string, sizeBytes int64, localPath string, etag string) error {
	_, err := db.ExecContext(ctx, `
		INSERT INTO blobs (digest, local_path, size_bytes, etag)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(digest) DO UPDATE SET
			local_path=excluded.local_path,
			size_bytes=excluded.size_bytes,
			etag=excluded.etag
		`, digest, localPath, sizeBytes, etag)
	return err
}


// GetBlobByETag checks if an Blob with a given etag already exists
func GetBlobByETag(ctx context.Context, db *sql.DB, etag string) (*Blob, error) {
	row := db.QueryRowContext(ctx,
		`SELECT id, digest, local_path, size_bytes, etag, created_at FROM blobs WHERE etag = ?`, etag,
	)

	var blob Blob
	if err := row.Scan( &blob.Digest, &blob.LocalPath, &blob.SizeBytes, &blob.Etag, &blob.CreatedAt); err != nil {
		return nil, err
	}

	return &blob, nil
}
