package app

import (
	"path/filepath"
	"strings"

	"github.com/MurasatoNaoya/crap4x/internal/detect"
)

// isTestFile reports whether the given path is a test file for the given language.
//
// Rules:
//   - Go: filename ends with "_test.go".
//   - Python: basename matches "test_*.py" or "*_test.py" or equals "conftest.py",
//     OR any path component (directory) is exactly "tests" or "test".
//   - Rust: any path component (directory) is exactly "tests".
//     (Inline #[cfg(test)] modules are not detectable by path; documented limitation.)
func isTestFile(path string, l detect.Lang) bool {
	base := filepath.Base(path)

	switch l {
	case detect.Go:
		return strings.HasSuffix(base, "_test.go")

	case detect.Python:
		if strings.HasPrefix(base, "test_") && strings.HasSuffix(base, ".py") {
			return true
		}
		if strings.HasSuffix(base, "_test.py") {
			return true
		}
		if base == "conftest.py" {
			return true
		}
		for _, component := range splitPathComponents(path) {
			if component == "tests" || component == "test" {
				return true
			}
		}
		return false

	case detect.Rust:
		for _, component := range splitPathComponents(path) {
			if component == "tests" {
				return true
			}
		}
		return false

	default:
		return false
	}
}

// splitPathComponents returns all path components (directories and file name)
// from a slash- or OS-separated path, skipping empty strings.
func splitPathComponents(path string) []string {
	// Normalise to forward slashes for consistent splitting.
	normalized := filepath.ToSlash(path)
	parts := strings.Split(normalized, "/")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
