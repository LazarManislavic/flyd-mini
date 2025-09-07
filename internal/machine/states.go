package machine

import (
	"context"
	"fmt"
	"os"

	"github.com/manuelinfosec/flyd/internal/s3"
	"github.com/manuelinfosec/flyd/internal/storage"
	"github.com/superfly/fsm"
)

// FetchObject:
//   - Check if image exists locally
//   - Read images by s3_key
//   - If it exists -> skip download
//   - If not -> fetch + hash
//   - Write to images table
func FetchObject(ctx context.Context, req *fsm.Request[FSMRequest, FSMResponse], app *AppContext) (*fsm.Response[FSMResponse], error) {
	key := req.Msg.Key
	dest := fmt.Sprintf("/tmp/%s.tar", key)

	// Step 1: Check if image already exists
	existing, err := storage.GetImageByKey(ctx, app.DB, key)
	if err != nil {
		return nil, fmt.Errorf("db lookup failed: %w", err)
	}

	if existing != nil {
		// File already registered, verify it’s still on disk
		if _, err := os.Stat(existing.LocalPath); err == nil {
			// Already present -> skip download
			return &fsm.Response[FSMResponse]{
				Msg: &FSMResponse{
					ImageID: existing.ID,	// imageRowID
					LocalPath: existing.LocalPath,
				},
			}, nil
		}
	}

	// Step 2: Fetch and hash new blob
	digest, err := s3.FetchAndHash(ctx, app.S3, key, dest)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch %s: %w", key, err)
	}

	// Get file size
	stat, err := os.Stat(dest)
	if err != nil {
		return nil, fmt.Errorf("failed to stat downloaded file: %w", err)
	}
	size := stat.Size()

	// Step 3: Insert/Upsert into DB
	imageRowID, err := storage.InsertImage(ctx, app.DB, key, 
			&digest, nil, size, dest)

	if err != nil {
		return nil, fmt.Errorf("failed to upsert image row: %w", err)
	}

	// Return FSM response
	return &fsm.Response[FSMResponse]{
		Msg: &FSMResponse{
			ImageID: imageRowID,
			LocalPath: dest,
		},
	}, nil
}
