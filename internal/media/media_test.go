package media

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- Sanitizer Tests ---

func TestSanitizeNormalFilename(t *testing.T) {
	got := Sanitize("report.pdf")
	if got != "report.pdf" {
		t.Errorf("got %q, want report.pdf", got)
	}
}

func TestSanitizePathTraversal(t *testing.T) {
	got := Sanitize("../../etc/passwd")
	if strings.Contains(got, "..") {
		t.Errorf("path traversal not stripped: %q", got)
	}
	if strings.Contains(got, "/") {
		t.Errorf("slashes not removed: %q", got)
	}
}

func TestSanitizeDotfile(t *testing.T) {
	got := Sanitize(".bashrc")
	if strings.HasPrefix(got, ".") {
		t.Errorf("dotfile prefix not replaced: %q", got)
	}
	if !strings.Contains(got, "bashrc") {
		t.Errorf("filename content lost: %q", got)
	}
}

func TestSanitizeLongFilename(t *testing.T) {
	name := strings.Repeat("a", 300) + ".pdf"
	got := Sanitize(name)
	if len(got) > 255 {
		t.Errorf("length = %d, want <= 255", len(got))
	}
	if !strings.HasSuffix(got, ".pdf") {
		t.Errorf("extension lost: %q", got)
	}
}

func TestSanitizeEmptyAfterCleaning(t *testing.T) {
	got := Sanitize("../..")
	if got == "" || got == "." {
		t.Errorf("empty result not handled: %q", got)
	}
	if !strings.HasPrefix(got, "file_") {
		t.Errorf("should generate name starting with file_, got: %q", got)
	}
}

func TestSanitizeSpecialChars(t *testing.T) {
	got := Sanitize("report (final) [v2].pdf")
	if got != "report (final) [v2].pdf" {
		t.Errorf("got %q, want report (final) [v2].pdf", got)
	}
}

func TestSanitizeWindowsSeparators(t *testing.T) {
	got := Sanitize("folder\\subfolder\\file.txt")
	if strings.Contains(got, "\\") {
		t.Errorf("backslashes not replaced: %q", got)
	}
	if !strings.Contains(got, "file.txt") {
		t.Errorf("filename lost: %q", got)
	}
}

// --- Collision Tests ---

func TestCollisionNoConflict(t *testing.T) {
	dir := t.TempDir()
	got := ResolveCollision(dir, "report.pdf")
	if got != "report.pdf" {
		t.Errorf("got %q, want report.pdf", got)
	}
}

func TestCollisionSingle(t *testing.T) {
	dir := t.TempDir()
	createFile(t, dir, "report.pdf")

	got := ResolveCollision(dir, "report.pdf")
	if got != "report_2.pdf" {
		t.Errorf("got %q, want report_2.pdf", got)
	}
}

func TestCollisionMultiple(t *testing.T) {
	dir := t.TempDir()
	createFile(t, dir, "report.pdf")
	createFile(t, dir, "report_2.pdf")
	createFile(t, dir, "report_3.pdf")

	got := ResolveCollision(dir, "report.pdf")
	if got != "report_4.pdf" {
		t.Errorf("got %q, want report_4.pdf", got)
	}
}

func TestCollisionNoExtension(t *testing.T) {
	dir := t.TempDir()
	createFile(t, dir, "README")

	got := ResolveCollision(dir, "README")
	if got != "README_2" {
		t.Errorf("got %q, want README_2", got)
	}
}

// --- Quota Tests ---

func TestQuotaWithin(t *testing.T) {
	dir := t.TempDir()
	writeFileSize(t, filepath.Join(dir, "a.bin"), 1024)

	err := CheckQuota([]string{dir}, 100*1024*1024)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestQuotaExceeds(t *testing.T) {
	dir := t.TempDir()
	// Write a file that uses almost all quota.
	writeFileSize(t, filepath.Join(dir, "big.bin"), defaultQuotaBytes-50)

	err := CheckQuota([]string{dir}, 100)
	if err == nil {
		t.Fatal("expected quota exceeded error")
	}
	if !strings.Contains(err.Error(), "quota exceeded") {
		t.Errorf("error should mention quota, got: %v", err)
	}
}

func TestQuotaEmptyDirs(t *testing.T) {
	dir := t.TempDir()
	err := CheckQuota([]string{dir}, 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestQuotaTopLevelOnly(t *testing.T) {
	dir := t.TempDir()
	writeFileSize(t, filepath.Join(dir, "top.bin"), 1000)
	subdir := filepath.Join(dir, "sub")
	if err := os.MkdirAll(subdir, 0755); err != nil {
		t.Fatal(err)
	}
	writeFileSize(t, filepath.Join(subdir, "nested.bin"), 999999)

	// Only top-level files should count (1000 bytes used + 100 MB fits).
	err := CheckQuota([]string{dir}, 100*1024*1024)
	if err != nil {
		t.Fatalf("unexpected error (subdirs should be ignored): %v", err)
	}
}

// --- Helpers ---

func createFile(t *testing.T, dir, name string) {
	t.Helper()
	f, err := os.Create(filepath.Join(dir, name))
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
}

func writeFileSize(t *testing.T, path string, size int64) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := f.Truncate(size); err != nil {
		t.Fatal(err)
	}
}
