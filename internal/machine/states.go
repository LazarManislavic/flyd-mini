package machine

import (
	"context"
	"fmt"
	"os"
	"encoding/xml"
	"io"
	"net/http"
	"github.com/manuelinfosec/flyd/internal/s3"
	"github.com/manuelinfosec/flyd/internal/storage"
	"github.com/superfly/fsm"
)

// FetchObject:
//   - Check if image exists locally
//   - Read images by s3_key
//   - If it exists -> skip download
//   - If not -> fetch + hash
//   - Write to blobs & images table
func FetchObject(ctx context.Context, req *fsm.Request[FSMRequest, FSMResponse], app *AppContext) (*fsm.Response[FSMResponse], error) {
	fmt.Println("happenendddd")
	key := req.Msg.Key
	destDir := "blobs"

	// Step 1: Get XML listing
	url := fmt.Sprintf("https://%s.s3.us-east-1.amazonaws.com/", req.Msg.BucketName)
	reqHTTP, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(reqHTTP)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var listing s3.ListBucketResult
	if err := xml.Unmarshal(data, &listing); err != nil {
		return nil, fmt.Errorf("xml parse failed: %w", err)
	}

	// Step 2: Fetch layers for the requested family (req.Msg.ImageName)
	seenETags := make(map[string]bool)
	paths, err := s3.FetchImageLayers(ctx, app.S3, req.Msg.ImageName, listing, destDir, seenETags)
	if err != nil {
		return nil, fmt.Errorf("fetch image layers failed: %w", err)
	}

	// Step 3: Save each blob into DB
	var lastDigest string
	var lastPath string

	for _, p := range paths {
		stat, err := os.Stat(p)
		if err != nil {
			return nil, fmt.Errorf("failed to stat %s: %w", p, err)
		}
		size := stat.Size()

		// reuse digest from FetchAndHash
		digest, err := s3.ComputeFileDigest(p)
		if err != nil {
			return nil, err
		}

		if err := storage.InsertBlob(ctx, app.DB, digest, size, p, ""); err != nil {
			return nil, fmt.Errorf("failed to insert blob: %w", err)
		}

		lastDigest = digest
		lastPath = p
	}

	// Insert image row referencing last blob
	imageRowID, err := storage.InsertImage(ctx, app.DB, key,
		&lastDigest, nil, 0, lastPath, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to upsert image row: %w", err)
	}

	return &fsm.Response[FSMResponse]{
		Msg: &FSMResponse{
			BaseDir: "rootfs/", // poplulate with where it will be unpacked 
			ImageID:   imageRowID,
			LocalPath: lastPath,
		},
	}, nil
}



func UnpackLayers(ctx context.Context, req *fsm.Request[FSMRequest, FSMResponse], app *AppContext) (*fsm.Response[FSMResponse], error) {
	return &fsm.Response[FSMResponse]{
		Msg: &FSMResponse{
			BaseDir: "rootfs/", // poplulate with where it will be unpacked 
			ImageID:   1,
			LocalPath: "lastPath",
		},
	}, nil
}