package internal

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// UnpackTar extracts a single tar file into destDir using the system `tar` command.
func UnpackTar(tarPath, destDir string) error {
	// Ensure destDir exists
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("failed to create dest dir %s: %w", destDir, err)
	}

	// Run: tar -xf <tarPath> -C <destDir>
	cmd := exec.Command("tar", "-xf", tarPath, "-C", destDir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to extract %s into %s: %w", filepath.Base(tarPath), destDir, err)
	}

	return nil
}