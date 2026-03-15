package media

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// Save writes data from reader to the target directory with a safe filename.
func Save(dir, rawName string, size int64, r io.Reader, quotaDirs []string) (string, error) {
	safeName := Sanitize(rawName)
	safeName = ResolveCollision(dir, safeName)

	if err := CheckQuota(quotaDirs, size); err != nil {
		return "", err
	}

	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	fullPath := filepath.Join(dir, safeName)
	return fullPath, writeFile(fullPath, r)
}

func writeFile(path string, r io.Reader) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create file %s: %w", path, err)
	}
	defer f.Close()

	if _, err := io.Copy(f, r); err != nil {
		return fmt.Errorf("failed to write file %s: %w", path, err)
	}
	return nil
}
