package media

import (
	"fmt"
	"os"
)

const defaultQuotaBytes = 10 * 1024 * 1024 * 1024 // 10 GB

// CheckQuota verifies that adding fileSize bytes would not exceed quota.
func CheckQuota(dirs []string, fileSize int64) error {
	used, err := calculateUsage(dirs)
	if err != nil {
		return fmt.Errorf("failed to calculate usage: %w", err)
	}

	if used+fileSize > defaultQuotaBytes {
		return fmt.Errorf(
			"storage quota exceeded: used %d bytes, file %d bytes, limit %d bytes",
			used, fileSize, defaultQuotaBytes,
		)
	}
	return nil
}

func calculateUsage(dirs []string) (int64, error) {
	var total int64
	for _, dir := range dirs {
		size, err := dirTopLevelSize(dir)
		if err != nil {
			return 0, err
		}
		total += size
	}
	return total, nil
}

func dirTopLevelSize(dir string) (int64, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}

	var total int64
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		info, err := e.Info()
		if err != nil {
			return 0, err
		}
		total += info.Size()
	}
	return total, nil
}
