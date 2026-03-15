package media

import (
	"path/filepath"
	"strings"
	"time"
)

const maxFilenameLen = 255

// Sanitize cleans a filename to prevent path traversal and other issues.
func Sanitize(name string) string {
	// Replace Windows separators.
	name = strings.ReplaceAll(name, "\\", "_")

	// Take only the base name (strips directory components).
	name = filepath.Base(name)

	// Remove path traversal.
	name = strings.ReplaceAll(name, "..", "")

	// Replace leading dots.
	if strings.HasPrefix(name, ".") {
		name = "_" + name[1:]
	}

	// Remove remaining path separators.
	name = strings.ReplaceAll(name, "/", "_")

	// Handle empty result.
	name = strings.TrimSpace(name)
	if name == "" || name == "." {
		name = "file_" + time.Now().Format("20060102_150405")
	}

	// Truncate if too long, preserving extension.
	if len(name) > maxFilenameLen {
		name = truncateWithExtension(name)
	}

	return name
}

func truncateWithExtension(name string) string {
	ext := filepath.Ext(name)
	base := name[:len(name)-len(ext)]
	maxBase := maxFilenameLen - len(ext)
	if maxBase < 1 {
		maxBase = 1
	}
	if len(base) > maxBase {
		base = base[:maxBase]
	}
	return base + ext
}
