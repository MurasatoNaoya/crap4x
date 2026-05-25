// Package app contains the orchestration logic for the crap4x analysis
// pipeline. It is intentionally separate from the main package so that the
// end-to-end flow can be exercised directly in tests without spawning a
// subprocess.
package app

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/MurasatoNaoya/crap4x/internal/coverage"
	"github.com/MurasatoNaoya/crap4x/internal/crap"
	"github.com/MurasatoNaoya/crap4x/internal/detect"
	"github.com/MurasatoNaoya/crap4x/internal/lang"
)

// Options controls a single crap4x analysis run.
type Options struct {
	// Path is the root directory to walk for source files.
	// Defaults to "." when empty.
	Path string

	// Langs overrides language auto-detection. When empty, Detect is called on
	// Path to determine the languages present.
	Langs []detect.Lang

	// CoverageLcov is the path to a pre-generated lcov file. It is the primary
	// coverage-ingest path in v1. When empty, Analyze returns an error with a
	// hint about how to generate one.
	CoverageLcov string

	// Threshold, when > 0, is used by the caller to gate CI: any result with
	// CRAP > Threshold indicates a quality violation. Analyze itself does not
	// enforce the threshold; it is surfaced via crap.AboveThreshold.
	Threshold float64

	// Top limits the rows rendered by crap.Report. 0 means all rows.
	Top int

	// IncludeTests, when true, includes test files in the analysis. By default
	// (false) test files are skipped so they do not pollute the CRAP report.
	IncludeTests bool
}

// skipDirs is the set of directory names that the file walker never descends into.
var skipDirs = map[string]bool{
	"vendor":       true,
	".git":         true,
	"target":       true,
	"node_modules": true,
}

// Analyze is the top-level orchestration function. It:
//  1. Auto-detects languages from Options.Path (or uses Options.Langs if set).
//  2. Walks the directory tree, skipping vendored/hidden dirs, collecting source
//     files whose extension matches a detected/specified language.
//  3. Parses each source file with lang.Analyze.
//  4. Parses the lcov file at Options.CoverageLcov with coverage.Parse.
//  5. Joins functions with coverage via crap.Compute and returns the results
//     sorted by CRAP descending.
func Analyze(opts Options) ([]crap.Result, error) {
	root := opts.Path
	if root == "" {
		root = "."
	}

	// Resolve languages.
	langs := opts.Langs
	if len(langs) == 0 {
		detected, err := detect.Detect(root)
		if err != nil {
			return nil, fmt.Errorf("detecting languages in %q: %w", root, err)
		}
		langs = detected
	}
	if len(langs) == 0 {
		return nil, fmt.Errorf("no supported languages detected in %q; use --lang to specify one", root)
	}

	// Build extension→spec map.
	type langEntry struct {
		ext  string
		lang detect.Lang
		spec lang.LangSpec
	}
	entries := make([]langEntry, 0, len(langs))
	for _, l := range langs {
		entries = append(entries, langEntry{ext: l.Ext(), lang: l, spec: l.Spec()})
	}

	// Walk source files.
	var funcs []lang.Function
	walkErr := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := d.Name()
		if d.IsDir() {
			// Skip vendored/hidden directories.
			if skipDirs[name] || strings.HasPrefix(name, ".") {
				return filepath.SkipDir
			}
			return nil
		}
		ext := filepath.Ext(name)
		for _, entry := range entries {
			if ext == entry.ext {
				// Use a path relative to root so it can match lcov SF: paths.
				rel, relErr := filepath.Rel(root, path)
				if relErr != nil {
					rel = path
				}
				// Skip test files unless the caller opted in.
				if !opts.IncludeTests && isTestFile(rel, entry.lang) {
					break
				}
				src, readErr := os.ReadFile(path)
				if readErr != nil {
					// Log and continue; a single unreadable file should not abort the run.
					fmt.Fprintf(os.Stderr, "warning: skipping %s: %v\n", path, readErr)
					continue
				}
				fileFuncs := lang.Analyze(src, rel, entry.spec)
				funcs = append(funcs, fileFuncs...)
				break
			}
		}
		return nil
	})
	if walkErr != nil {
		return nil, fmt.Errorf("walking %q: %w", root, walkErr)
	}

	// Load coverage.
	if opts.CoverageLcov == "" {
		// Build a hint listing the per-language generation commands.
		return nil, noCoverageError(langs, root)
	}
	f, err := os.Open(opts.CoverageLcov)
	if err != nil {
		return nil, fmt.Errorf("opening coverage file %q: %w", opts.CoverageLcov, err)
	}
	defer f.Close()

	cov, err := coverage.Parse(f)
	if err != nil {
		return nil, fmt.Errorf("parsing coverage file %q: %w", opts.CoverageLcov, err)
	}

	return crap.Compute(funcs, cov), nil
}

// LcovCommand returns the exact lcov-generation command(s) for the given
// language, as documented in the README "Producing an lcov report" table.
// For Go the two steps are returned joined by a newline. For languages that
// are not recognised an empty string is returned.
func LcovCommand(l detect.Lang) string {
	switch l {
	case detect.Go:
		return "go test ./... -coverprofile=cover.out\n  gcov2lcov -infile=cover.out -outfile=cover.lcov"
	case detect.Python:
		return "coverage run -m pytest && coverage lcov -o cover.lcov"
	case detect.Rust:
		return "cargo llvm-cov --lcov --output-path cover.lcov"
	default:
		return ""
	}
}

// noCoverageError constructs a helpful error when --coverage is not supplied.
func noCoverageError(langs []detect.Lang, root string) error {
	var sb strings.Builder
	sb.WriteString("--coverage <lcov> is required; generate one with:\n")
	if len(langs) == 0 {
		sb.WriteString("  go test ./... -coverprofile=cover.out\n")
		sb.WriteString("    gcov2lcov -infile=cover.out -outfile=cover.lcov\n")
		sb.WriteString("  coverage run -m pytest && coverage lcov -o cover.lcov\n")
		sb.WriteString("  cargo llvm-cov --lcov --output-path cover.lcov\n")
	} else {
		for _, l := range langs {
			cmd := LcovCommand(l)
			if cmd != "" {
				sb.WriteString(fmt.Sprintf("  %s\n", cmd))
			}
		}
	}
	sb.WriteString("\nthen pass the resulting lcov file via --coverage")
	_ = root
	return fmt.Errorf("%s", sb.String())
}
