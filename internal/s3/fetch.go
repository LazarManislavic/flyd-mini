package s3

import (
	"context"
	"crypto/sha256"
	"fmt"
	"hash"
	"io"
	"os"
)

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
