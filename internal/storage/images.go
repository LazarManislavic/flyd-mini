package storage

import (
	"context"
	"database/sql"
	"fmt"
)

// A row in the `images` table
type Image struct {
	ID        int64
	Name      string
	Digest    sql.NullString
	BaseLvID  sql.NullInt64
	SizeBytes sql.NullInt64
	LocalPath string
	CreatedAt string
}

func InsertImage(ctx context.Context, db *sql.DB, name string, digest string, baseLvID *int64, sizeBytes int64, localPath string) (int64, error) {
    var id int64

    // Upsert with RETURNING to get the actual row ID
    err := db.QueryRowContext(ctx, `
        INSERT INTO images (name, digest, base_lv_id, size_bytes, local_path, created_at)
        VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
        ON CONFLICT(digest) DO UPDATE SET
            name=excluded.name,
            base_lv_id=excluded.base_lv_id,
            size_bytes=excluded.size_bytes,
            local_path=excluded.local_path
        RETURNING id
    `, name, digest, baseLvID, sizeBytes, localPath).Scan(&id)
    if err != nil {
        return 0, err
    }

    return id, nil
}

func GetImageByKey(ctx context.Context, db *sql.DB, s3Key string) (*Image, error) {
	row := db.QueryRowContext(ctx,
		`SELECT id, name, digest, base_lv_id, size_bytes, local_path, created_at FROM images WHERE s3_key = ?`, s3Key,
	)

	var img Image
	if err := row.Scan(&img.ID, &img.Name, &img.Digest, &img.BaseLvID, &img.SizeBytes, &img.LocalPath, &img.CreatedAt); err != nil {
		return nil, err
	}

	return &img, nil
}

// GetImageByID retrieves an image row by its ID.
// Returns nil if no image exists with the given ID.
func GetImageByID(ctx context.Context, db *sql.DB, imageID int64) (*Image, error) {
	row := db.QueryRowContext(ctx,
		`SELECT id, name, digest, base_lv_id, size_bytes, local_path, created_at
		 FROM images
		 WHERE id = ?`, imageID,
	)

	var img Image
	err := row.Scan(&img.ID, &img.Name, &img.Digest, &img.BaseLvID, &img.SizeBytes, &img.LocalPath, &img.CreatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // no image found
		}
		return nil, err
	}

	return &img, nil
}

// GetImageByBaseLvID returns an image row matching the given base_lv_id.
// Returns nil if no image exists with that base_lv_id.
func GetImageByBaseLvID(ctx context.Context, db *sql.DB, baseLvID int64) (*Image, error) {
	row := db.QueryRowContext(ctx,
		`SELECT id, name, digest, base_lv_id, size_bytes, local_path, created_at
		 FROM images
		 WHERE base_lv_id = ?`, baseLvID,
	)

	var img Image
	err := row.Scan(&img.ID, &img.Name, &img.Digest, &img.BaseLvID, &img.SizeBytes, &img.LocalPath, &img.CreatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // no existing image with this base_lv_id
		}
		return nil, err
	}

	return &img, nil
}

// UpdateBaseLvID sets the base_lv_id for an image given its ID
func UpdateBaseLvID(ctx context.Context, db *sql.DB, imageID int64, baseLvID int64) error {
	fmt.Println("Base Level ID: ", baseLvID)
	_, err := db.ExecContext(ctx,
		`UPDATE images SET base_lv_id = ? WHERE id = ?`,
		baseLvID, imageID,
	)
	return err
}
