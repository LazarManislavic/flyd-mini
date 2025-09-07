package s3

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"strings"
)

// --- XML structures for S3 bucket listings ---
type ListBucketResult struct {
	Contents []S3Object `xml:"Contents"`
}

type S3Object struct {
	Key  string `xml:"Key"`
	Size int64  `xml:"Size"`
	ETag string `xml:"ETag"`
}


// ComputeFileDigest computes the SHA256 digest of a file and returns it as a hex string.
func ComputeFileDigest(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("failed to open file %s: %w", path, err)
	}
	defer file.Close()

	h := sha256.New()
	if _, err := io.Copy(h, file); err != nil {
		return "", fmt.Errorf("failed to hash file %s: %w", path, err)
	}

	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

// Fetch retrieves an object from an S3 bucket, saves it to a local file,
// but skips download if the file already exists at destPath.
func Fetch(ctx context.Context, client *S3Client, key, destPath string) error {
	// Check if file already exists
	if _, err := os.Stat(destPath); err == nil {
		// File exists → skip download
		return nil
	} else if !os.IsNotExist(err) {
		// Some other error (permissions, etc.)
		return fmt.Errorf("failed to stat %s: %w", destPath, err)
	}

	// File does not exist → download from S3
	stream, err := client.GetObjectStream(ctx, key)
	if err != nil {
		return err
	}
	defer stream.Close()

	file, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("failed to create file %s: %w", destPath, err)
	}
	defer file.Close()

	// Stream copy (no digest calc since function returns only error)
	if _, err := io.Copy(file, stream); err != nil {
		return fmt.Errorf("failed to copy object data: %w", err)
	}

	return nil
}

// FetchImageLayers parses an XML listing and pulls the layers for a specific family (golang/python/node).
// It checks the DB for existing etags to avoid re-downloading.
func FetchImageLayers(ctx context.Context, client *S3Client, family string, listing ListBucketResult, destDir string, seenETags map[string]bool)  ([][]string, error) {
	var pulled [][]string

	for _, obj := range listing.Contents {
		// only consider objects belonging to the family (images/golang/, images/node/, etc.)
		if !strings.HasPrefix(obj.Key, "images/"+family+"/") {
			continue
		}

		// skip if already pulled by ETag
		if seenETags[obj.ETag] {
			continue
		}

		destPath := fmt.Sprintf("%s/%s", destDir, strings.ReplaceAll(obj.Key, "/", "_"))
		if err := Fetch(ctx, client, obj.Key, destPath); err != nil {
			return pulled, err
		}

		// mark this ETag as seen
		seenETags[obj.ETag] = true

		pathWithEtag := []string{destPath, obj.ETag}
		pulled = append(pulled, pathWithEtag)
	}

	return pulled, nil
}
