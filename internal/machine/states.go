package machine

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/manuelinfosec/flyd/internal"
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
	// TODO: Expand seenETags to come from database
	seenETags := make(map[string]bool)
	paths, err := s3.FetchImageLayers(ctx, app.S3, req.Msg.ImageName, listing, destDir, seenETags)
	if err != nil {
		return nil, fmt.Errorf("fetch image layers failed: %w", err)
	}

	// Step 3: Save each blob into DB
	var lastDigest string
	var lastPath string

	for _, p := range paths {
		stat, err := os.Stat(p[0])
		if err != nil {
			return nil, fmt.Errorf("failed to stat %s: %w", p, err)
		}
		size := stat.Size()

		// reuse digest from FetchAndHash
		digest, err := s3.ComputeFileDigest(p[0])
		if err != nil {
			return nil, err
		}

		if err := storage.InsertBlob(ctx, app.DB, digest, size, p[0], p[1]); err != nil {
			return nil, fmt.Errorf("failed to insert blob: %w", err)
		}

		lastDigest = digest
		lastPath = p[0]
	}

	// Insert image row referencing last blob
	imageRowID, err := storage.InsertImage(ctx, app.DB,
		&lastDigest, nil, 0, lastPath)
	if err != nil {
		return nil, fmt.Errorf("failed to upsert image row: %w", err)
	}

	return &fsm.Response[FSMResponse]{
		Msg: &FSMResponse{
			BaseDir:   "rootfs/", // where they will be unpacked to
			ImageID:   imageRowID,
			LocalPath: "blobs/", // dynamically get it from `lastPath`
		},
	}, nil
}

// UnpackLayers unpacks all tarballs in LocalPath dir into BaseDir
func UnpackLayers(ctx context.Context, req *fsm.Request[FSMRequest, FSMResponse], app *AppContext) (*fsm.Response[FSMResponse], error) {
	srcDir := req.W.Msg.LocalPath // e.g., "blob/"
	destDir := req.W.Msg.BaseDir  // e.g., "rootfs/"

	if err := os.MkdirAll(destDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create base dir: %w", err)
	}

	// Loop through 1.tar … 5.tar
	for i := 1; i <= 5; i++ {
		tarPath := filepath.Join(srcDir, fmt.Sprintf("images_%s_%d.tar", req.Msg.ImageName, i))
		if _, err := os.Stat(tarPath); err == nil {
			if err := internal.UnpackTar(tarPath, destDir); err != nil {
				return nil, fmt.Errorf("failed to unpack %s: %w", tarPath, err)
			}
		}
	}

	return &fsm.Response[FSMResponse]{
		Msg: &FSMResponse{
			BaseDir:   destDir,
			ImageID:   req.W.Msg.ImageID,
			LocalPath: srcDir,
		},
	}, nil
}
