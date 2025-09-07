package s3

import (
	"context"
	"crypto/sha256"
	"fmt"
	"hash"
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

// FetchAndHash retrieves an object from an S3 bucket, saves it to a local file,
// and computes its SHA256 hash.
//
// It streams the content to both a file and a hash calculator simultaneously
// to ensure a single pass and efficient memory usage.
func FetchAndHash(ctx context.Context, client *S3Client, key, destPath string) (string, error) {
	// Open remote stream
	stream, err := client.GetObjectStream(ctx, key)
	if err != nil {
		return "", err
	}

	defer stream.Close()

	// Create local file
	file, err := os.Create(destPath)
	if err != nil {
		return "", fmt.Errorf("failed to create file %s: %w", destPath, err)
	}
	defer file.Close()

	// Wrap with the hash calculator
	var h hash.Hash = sha256.New()
	writer := io.MultiWriter(file, h)

	// Stream copy
	if _, err := io.Copy(writer, stream); err != nil {
		return "", fmt.Errorf("failed to copy object data: %w", err)
	}

	// Return hash as hex string
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

// FetchImageLayers parses an XML listing and pulls the layers for a specific family (golang/python/node).
// It checks the DB for existing etags to avoid re-downloading.
func FetchImageLayers(ctx context.Context, client *S3Client, family string, listing ListBucketResult, destDir string, seenETags map[string]bool) ([]string, error) {
	var pulled []string

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
		_, err := FetchAndHash(ctx, client, obj.Key, destPath)
		if err != nil {
			return pulled, err
		}

		// mark this ETag as seen
		seenETags[obj.ETag] = true
		pulled = append(pulled, destPath)
	}

	return pulled, nil
}
