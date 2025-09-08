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
	Digest    string
	BaseLvID  sql.NullInt64
	SizeBytes sql.NullInt64
	LocalPath sql.NullString
	Complete  bool
	CreatedAt string
}

// InsertImage inserts or updates an image row and returns its ID.
func InsertImage(ctx context.Context, db *sql.DB, name string, digest string, baseLvID *int64, sizeBytes int64, localPath string) (int64, error) {
	var id int64

	// Upsert with RETURNING to get the actual row ID
	err := db.QueryRowContext(ctx, `
        INSERT INTO images (name, digest, base_lv_id, size_bytes, local_path, complete, created_at)
        VALUES (?, ?, ?, ?, ?, 0, CURRENT_TIMESTAMP)
        ON CONFLICT(digest) DO UPDATE SET
            name=excluded.name,
            size_bytes=excluded.size_bytes,
            local_path=excluded.local_path
        RETURNING id
    `, name, digest, baseLvID, sizeBytes, localPath).Scan(&id)
	if err != nil {
		return 0, err
	}

	return id, nil
}

// GetImageByName retrieves an image row by its name.
func GetImageByName(ctx context.Context, db *sql.DB, name string) (*Image, error) {
	row := db.QueryRowContext(ctx,
		`SELECT id, name, digest, base_lv_id, size_bytes, local_path, complete, created_at 
         FROM images WHERE name = ?`, name,
	)

	var img Image
	if err := row.Scan(&img.ID, &img.Name, &img.Digest, &img.BaseLvID, &img.SizeBytes, &img.LocalPath, &img.Complete, &img.CreatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	return &img, nil
}

// GetImageByID retrieves an image row by its ID.
func GetImageByID(ctx context.Context, db *sql.DB, imageID int64) (*Image, error) {
	row := db.QueryRowContext(ctx,
		`SELECT id, name, digest, base_lv_id, size_bytes, local_path, complete, created_at
		 FROM images
		 WHERE id = ?`, imageID,
	)

	var img Image
	err := row.Scan(&img.ID, &img.Name, &img.Digest, &img.BaseLvID, &img.SizeBytes, &img.LocalPath, &img.Complete, &img.CreatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // no image found
		}
		return nil, err
	}
	fmt.Printf("%+v\n", img)

	return &img, nil
}

// GetImageByBaseLvID returns an image row matching the given base_lv_id.
func GetImageByBaseLvID(ctx context.Context, db *sql.DB, baseLvID int64) (*Image, error) {
	row := db.QueryRowContext(ctx,
		`SELECT id, name, digest, base_lv_id, size_bytes, local_path, complete, created_at
		 FROM images
		 WHERE base_lv_id = ?`, baseLvID,
	)

	var img Image
	err := row.Scan(&img.ID, &img.Name, &img.Digest, &img.BaseLvID, &img.SizeBytes, &img.LocalPath, &img.Complete, &img.CreatedAt)
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