package storage

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/sirupsen/logrus"
)

// Blob represents a row in the `blobs` table.
type Blob struct {
	Digest    string
	LocalPath sql.NullString
	SizeBytes sql.NullInt64
	Etag      string
	Complete  bool
	CreatedAt string
}

// InsertBlob inserts or updates a blob record by digest or etag.
func InsertBlob(ctx context.Context, db *sql.DB, digest string, sizeBytes int64, localPath string, etag string, complete bool) error {
	_, err := db.ExecContext(ctx, `
		INSERT INTO blobs (digest, local_path, size_bytes, etag, complete)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(digest) DO UPDATE SET
			local_path = excluded.local_path,
			size_bytes = excluded.size_bytes,
			etag       = excluded.etag,
			complete   = excluded.complete
		ON CONFLICT(etag) DO UPDATE SET
			digest     = excluded.digest,
			local_path = excluded.local_path,
			size_bytes = excluded.size_bytes,
			complete   = excluded.complete
	`, digest, localPath, sizeBytes, etag, complete)
	return err
}


// GetBlobByETag retrieves a blob by its ETag.
// Returns nil if no blob is found.
func GetBlobByETag(ctx context.Context, db *sql.DB, etag string) (*Blob, error) {
	row := db.QueryRowContext(ctx,
		`SELECT digest, local_path, size_bytes, etag, complete, created_at
		 FROM blobs
		 WHERE etag = ?`, etag,
	)

	var blob Blob
	if err := row.Scan(&blob.Digest, &blob.LocalPath, &blob.SizeBytes, &blob.Etag, &blob.Complete, &blob.CreatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	return &blob, nil
}

// GetAllETags loads all complete blob etags into a map for quick lookup.
func GetAllETags(ctx context.Context, db *sql.DB) (map[string]bool, error) {
	logrus.Infof("Loading existing complete blobs by etag")

	seenETags := make(map[string]bool)

	rows, err := db.QueryContext(ctx, `SELECT etag FROM blobs WHERE complete = 1`)
	if err != nil {
		return nil, fmt.Errorf("failed to load etags: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var etag string
		if err := rows.Scan(&etag); err != nil {
			return nil, err
		}
		seenETags[etag] = true
	}
	return seenETags, rows.Err()
}

// InsertImageBlob creates a mapping between an image and a blob.
// Safe to call multiple times thanks to INSERT OR IGNORE.
func InsertImageBlob(ctx context.Context, db *sql.DB, imageID int64, blobDigest string) error {
	_, err := db.ExecContext(ctx, `
		INSERT OR IGNORE INTO image_blobs (image_id, blob_digest)
		VALUES (?, ?)
	`, imageID, blobDigest)
	return err
}

// CountMissingBlobs returns how many required blobs for an image are not yet complete.
func CountMissingBlobs(ctx context.Context, db *sql.DB, imageID int64) (int, error) {
	row := db.QueryRowContext(ctx, `
		SELECT COUNT(*) 
		FROM image_blobs ib
		LEFT JOIN blobs b ON ib.blob_digest = b.digest AND b.complete = 1
		WHERE ib.image_id = ? AND b.digest IS NULL
	`, imageID)

	var missing int
	if err := row.Scan(&missing); err != nil {
		return 0, err
	}
	return missing, nil
}

// UpdateImageCompletion updates the `complete` field of an image
// based on whether all its blobs are fully fetched.
func UpdateImageCompletion(ctx context.Context, db *sql.DB, imageID int64) error {
	missing, err := CountMissingBlobs(ctx, db, imageID)
	if err != nil {
		return err
	}
	complete := (missing == 0)

	_, err = db.ExecContext(ctx, `
		UPDATE images SET complete = ? WHERE id = ?
	`, complete, imageID)
	return err
}