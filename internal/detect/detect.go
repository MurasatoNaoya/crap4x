// Package detect identifies which programming languages are present in a
// project directory by inspecting well-known marker files, and bridges those
// languages to the internal/lang LangSpec registry.
package detect

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/MurasatoNaoya/crap4x/internal/lang"
)

// Lang identifies a supported programming language.
type Lang int

const (
	Go     Lang = iota
	Python Lang = iota
	Rust   Lang = iota
)

// String returns the canonical lower-case name of the language.
func (l Lang) String() string {
	switch l {
	case Go:
		return "go"
	case Python:
		return "python"
	case Rust:
		return "rust"
	default:
		return "unknown"
	}
}

// Ext returns the file extension (including the leading dot) for source files
// of this language, used by the CLI when globbing for source files to analyze.
func (l Lang) Ext() string {
	switch l {
	case Go:
		return ".go"
	case Python:
		return ".py"
	case Rust:
		return ".rs"
	default:
		return ""
	}
}

// Spec bridges this language to the tree-sitter LangSpec stored in
// internal/lang, giving callers a single entry-point from a detected Lang to
// the full analysis configuration without importing internal/lang directly.
func (l Lang) Spec() lang.LangSpec {
	switch l {
	case Go:
		return lang.Go()
	case Python:
		return lang.Python()
	case Rust:
		return lang.Rust()
	default:
		return lang.LangSpec{}
	}
}

// ParseLang converts a language name string (case-insensitive) to the
// corresponding Lang constant. It returns (lang, true) on success and
// (0, false) if the name is not recognised. This is used to implement
// the --lang CLI flag override.
func ParseLang(s string) (Lang, bool) {
	switch strings.ToLower(s) {
	case "go":
		return Go, true
	case "python":
		return Python, true
	case "rust":
		return Rust, true
	default:
		return 0, false
	}
}

// pythonMarkers is the set of file names whose presence indicates a Python
// project. Any one of these is sufficient.
var pythonMarkers = []string{
	"pyproject.toml",
	"setup.py",
	"setup.cfg",
	"requirements.txt",
}

// Detect inspects dir for well-known project marker files and returns the
// languages present. Multiple languages may be returned (in unspecified order)
// when a directory contains markers for more than one language; e.g. a Go
// package vendored inside a Python monorepo.
//
// Markers used:
//
//	go.mod                                      → Go
//	Cargo.toml                                  → Rust
//	pyproject.toml | setup.py | setup.cfg |
//	  requirements.txt                          → Python
//
// If no markers are found, a nil/empty slice is returned with a nil error.
// An error is returned only for filesystem failures (e.g. dir does not exist).
func Detect(dir string) ([]Lang, error) {
	// Verify that dir is accessible.
	if _, err := os.Stat(dir); err != nil {
		return nil, err
	}

	var langs []Lang

	// Go
	if exists(filepath.Join(dir, "go.mod")) {
		langs = append(langs, Go)
	}

	// Rust
	if exists(filepath.Join(dir, "Cargo.toml")) {
		langs = append(langs, Rust)
	}

	// Python: any one marker is sufficient; avoid duplicates.
	for _, m := range pythonMarkers {
		if exists(filepath.Join(dir, m)) {
			langs = append(langs, Python)
			break
		}
	}

	return langs, nil
}

// exists reports whether the file at path is present (and accessible).
func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
