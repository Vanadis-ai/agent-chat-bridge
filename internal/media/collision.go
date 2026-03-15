package media

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ResolveCollision returns a path that does not collide with existing files.
func ResolveCollision(dir, name string) string {
	fullPath := filepath.Join(dir, name)
	if !fileExists(fullPath) {
		return name
	}

	ext := filepath.Ext(name)
	base := strings.TrimSuffix(name, ext)

	for i := 2; i < 10000; i++ {
		candidate := fmt.Sprintf("%s_%d%s", base, i, ext)
		if !fileExists(filepath.Join(dir, candidate)) {
			return candidate
		}
	}

	return name
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
