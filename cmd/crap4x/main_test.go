// Integration tests for the crap4x CLI orchestration layer.
// These tests call app.Analyze directly (no subprocess) and verify the
// end-to-end pipeline: detect -> walk -> analyze -> coverage join -> CRAP report.
package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MurasatoNaoya/crap4x/internal/app"
	"github.com/MurasatoNaoya/crap4x/internal/crap"
	"github.com/MurasatoNaoya/crap4x/internal/detect"
)

// goSource contains two functions with known complexity and coverage:
//
//   - simple: CC=1, covered 100% -> CRAP = 1^2*(1-1)^3 + 1 = 1.0
//   - complex: CC=5, covered 0%  -> CRAP = 5^2*(1-0)^3 + 5 = 30.0
const goSource = `package fixture

func simple(x int) int {
	return x + 1
}

func complex(x int) int {
	if x < 0 {
		return -1
	}
	if x == 0 {
		return 0
	}
	if x < 10 {
		return 1
	}
	if x < 100 {
		return 2
	}
	return 3
}
`

// lcovFixture covers "simple" (lines 3-5) fully; "complex" (lines 7-19) is uncovered.
// The SF path matches the relative path "fixture.go" produced by the walk.
const lcovFixture = `SF:fixture.go
DA:3,1
DA:4,1
DA:5,1
DA:7,0
DA:8,0
DA:9,0
DA:10,0
DA:11,0
DA:12,0
DA:13,0
DA:14,0
DA:15,0
DA:16,0
DA:17,0
DA:18,0
DA:19,0
end_of_record
`

// setupProject creates a temp dir with go.mod, a Go source file, and an lcov file.
func setupProject(t *testing.T) (dir string, lcovPath string) {
	t.Helper()
	dir = t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module fixture\n\ngo 1.21\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "fixture.go"), []byte(goSource), 0o644); err != nil {
		t.Fatal(err)
	}
	lcovPath = filepath.Join(dir, "cov.lcov")
	if err := os.WriteFile(lcovPath, []byte(lcovFixture), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir, lcovPath
}

func TestIntegration_RankOrder(t *testing.T) {
	dir, lcovPath := setupProject(t)
	results, err := app.Analyze(app.Options{
		Path:         dir,
		CoverageLcov: lcovPath,
	})
	if err != nil {
		t.Fatalf("Analyze returned error: %v", err)
	}

	report := crap.Report(results, 0)

	// Both functions must appear.
	if !strings.Contains(report, "complex") {
		t.Errorf("expected 'complex' in report; got:\n%s", report)
	}
	if !strings.Contains(report, "simple") {
		t.Errorf("expected 'simple' in report; got:\n%s", report)
	}

	// "complex" must appear before "simple" (sorted by CRAP descending).
	idxComplex := strings.Index(report, "complex")
	idxSimple := strings.Index(report, "simple")
	if idxComplex >= idxSimple {
		t.Errorf("expected 'complex' to rank before 'simple'; got:\n%s", report)
	}
}

func TestIntegration_ThresholdExceeded(t *testing.T) {
	dir, lcovPath := setupProject(t)
	results, err := app.Analyze(app.Options{
		Path:         dir,
		CoverageLcov: lcovPath,
	})
	if err != nil {
		t.Fatalf("Analyze returned error: %v", err)
	}
	// complex has CRAP=30; threshold 20 should flag it.
	flagged := crap.AboveThreshold(results, 20)
	if len(flagged) == 0 {
		t.Error("expected at least one function to exceed threshold 20")
	}
	found := false
	for _, r := range flagged {
		if r.Func.Name == "complex" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected 'complex' in flagged functions; got %v", flagged)
	}
}

func TestIntegration_ThresholdNotExceeded(t *testing.T) {
	dir, lcovPath := setupProject(t)
	results, err := app.Analyze(app.Options{
		Path:         dir,
		CoverageLcov: lcovPath,
	})
	if err != nil {
		t.Fatalf("Analyze returned error: %v", err)
	}
	// threshold 100 is above all CRAP scores.
	flagged := crap.AboveThreshold(results, 100)
	if len(flagged) != 0 {
		t.Errorf("expected no functions above threshold 100, got %d", len(flagged))
	}
}

func TestIntegration_TopLimit(t *testing.T) {
	dir, lcovPath := setupProject(t)
	results, err := app.Analyze(app.Options{
		Path:         dir,
		CoverageLcov: lcovPath,
	})
	if err != nil {
		t.Fatalf("Analyze returned error: %v", err)
	}
	report := crap.Report(results, 1) // top=1
	if !strings.Contains(report, "complex") {
		t.Errorf("expected 'complex' in top-1 report; got:\n%s", report)
	}
	if strings.Contains(report, "simple") {
		t.Errorf("unexpected 'simple' in top-1 report; got:\n%s", report)
	}
}

func TestIntegration_LangOverride(t *testing.T) {
	// No go.mod marker, but --lang go forces Go analysis.
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "fixture.go"), []byte(goSource), 0o644); err != nil {
		t.Fatal(err)
	}
	lcovPath := filepath.Join(dir, "cov.lcov")
	if err := os.WriteFile(lcovPath, []byte(lcovFixture), 0o644); err != nil {
		t.Fatal(err)
	}
	results, err := app.Analyze(app.Options{
		Path:         dir,
		Langs:        []detect.Lang{detect.Go},
		CoverageLcov: lcovPath,
	})
	if err != nil {
		t.Fatalf("Analyze returned error: %v", err)
	}
	report := crap.Report(results, 0)
	if !strings.Contains(report, "complex") {
		t.Errorf("expected 'complex' in report; got:\n%s", report)
	}
}

func TestIntegration_SkipsVendor(t *testing.T) {
	dir, lcovPath := setupProject(t)
	// Add a Go file under vendor/ that must be skipped.
	vendorDir := filepath.Join(dir, "vendor")
	if err := os.MkdirAll(vendorDir, 0o755); err != nil {
		t.Fatal(err)
	}
	const vendorSource = `package vendored

func vendored(x int) int { return x }
`
	if err := os.WriteFile(filepath.Join(vendorDir, "v.go"), []byte(vendorSource), 0o644); err != nil {
		t.Fatal(err)
	}
	results, err := app.Analyze(app.Options{
		Path:         dir,
		CoverageLcov: lcovPath,
	})
	if err != nil {
		t.Fatalf("Analyze returned error: %v", err)
	}
	for _, r := range results {
		if r.Func.Name == "vendored" {
			t.Errorf("vendor/ file should be skipped; found function 'vendored' in results")
		}
	}
}

// TestRun_NoCoverage_GoProject verifies that Run returns ExitCoverageRequired (2)
// and prints the Go lcov generation commands when --coverage is absent on a Go
// project (go.mod present).
func TestRun_NoCoverage_GoProject(t *testing.T) {
	dir := t.TempDir()
	// Create a minimal go.mod so language detection returns Go.
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module fixture\n\ngo 1.21\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "fixture.go"), []byte(goSource), 0o644); err != nil {
		t.Fatal(err)
	}

	var sb strings.Builder
	code := Run(Config{
		Path:         dir,
		CoverageFile: "", // not provided
	}, &sb)

	if code != ExitCoverageRequired {
		t.Errorf("expected exit code %d (ExitCoverageRequired), got %d", ExitCoverageRequired, code)
	}

	out := sb.String()
	// Must contain the Go-specific lcov commands from the README table.
	wantSubstrings := []string{
		"go test ./... -coverprofile=cover.out",
		"gcov2lcov -infile=cover.out -outfile=cover.lcov",
	}
	for _, want := range wantSubstrings {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q; got:\n%s", want, out)
		}
	}
}

// TestParseFlags_PathBeforeFlags verifies that the flag parser correctly extracts
// the positional path argument even when it appears BEFORE the named flags.
// This is the real user invocation pattern: crap4x /some/path --coverage file.lcov --top 3
// The standard flag.Parse() stops at the first non-flag argument, so a path-first
// invocation leaves --coverage, --top, etc. unparsed. The fix uses iterative parsing.
func TestParseFlags_PathBeforeFlags(t *testing.T) {
	args := []string{"/some/project", "--coverage", "cover.lcov", "--top", "3", "--lang", "rust"}
	cfg, err := parseFlags(args)
	if err != nil {
		t.Fatalf("parseFlags returned error: %v", err)
	}
	if cfg.Path != "/some/project" {
		t.Errorf("Path: got %q, want %q", cfg.Path, "/some/project")
	}
	if cfg.CoverageFile != "cover.lcov" {
		t.Errorf("CoverageFile: got %q, want %q", cfg.CoverageFile, "cover.lcov")
	}
	if cfg.Top != 3 {
		t.Errorf("Top: got %d, want 3", cfg.Top)
	}
	if len(cfg.Langs) != 1 || cfg.Langs[0].String() != "rust" {
		t.Errorf("Langs: got %v, want [rust]", cfg.Langs)
	}
}

// TestParseFlags_FlagsBeforePath verifies that the flag parser also handles the
// conventional order (flags before positional path).
func TestParseFlags_FlagsBeforePath(t *testing.T) {
	args := []string{"--coverage", "cover.lcov", "--top", "5", "/some/project"}
	cfg, err := parseFlags(args)
	if err != nil {
		t.Fatalf("parseFlags returned error: %v", err)
	}
	if cfg.Path != "/some/project" {
		t.Errorf("Path: got %q, want %q", cfg.Path, "/some/project")
	}
	if cfg.CoverageFile != "cover.lcov" {
		t.Errorf("CoverageFile: got %q, want %q", cfg.CoverageFile, "cover.lcov")
	}
	if cfg.Top != 5 {
		t.Errorf("Top: got %d, want 5", cfg.Top)
	}
}

// TestRun_TopLimitViaCfg verifies that Run respects cfg.Top and limits printed rows.
func TestRun_TopLimitViaCfg(t *testing.T) {
	dir, lcovPath := setupProject(t)
	var sb strings.Builder
	code := Run(Config{
		Path:         dir,
		CoverageFile: lcovPath,
		Top:          1, // only the top function should be printed
	}, &sb)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d; output:\n%s", code, sb.String())
	}
	out := sb.String()
	// The highest-CRAP function ("complex") must appear; "simple" must not.
	if !strings.Contains(out, "complex") {
		t.Errorf("expected 'complex' in top-1 output; got:\n%s", out)
	}
	if strings.Contains(out, "simple") {
		t.Errorf("'simple' must NOT appear in top-1 output; got:\n%s", out)
	}
}

// TestParseFlags_IncludeTests verifies that --include-tests is parsed correctly.
func TestParseFlags_IncludeTests(t *testing.T) {
	// Default: flag absent, IncludeTests must be false.
	cfgDefault, err := parseFlags([]string{"--coverage", "cover.lcov"})
	if err != nil {
		t.Fatalf("parseFlags (default) error: %v", err)
	}
	if cfgDefault.IncludeTests {
		t.Error("IncludeTests should be false when --include-tests is not supplied")
	}

	// With flag: IncludeTests must be true.
	cfgWith, err := parseFlags([]string{"--coverage", "cover.lcov", "--include-tests"})
	if err != nil {
		t.Fatalf("parseFlags (--include-tests) error: %v", err)
	}
	if !cfgWith.IncludeTests {
		t.Error("IncludeTests should be true when --include-tests is supplied")
	}
}

// TestIntegration_ExcludesTestFiles verifies that Run excludes test files by
// default and includes them when IncludeTests is set.
func TestIntegration_ExcludesTestFiles(t *testing.T) {
	dir, lcovPath := setupProject(t)

	// Add a _test.go file with one extra function.
	const testSource = `package fixture

import "testing"

func TestSimple(t *testing.T) {
	if simple(1) != 2 {
		t.Fatal("bad")
	}
}
`
	if err := os.WriteFile(filepath.Join(dir, "fixture_test.go"), []byte(testSource), 0o644); err != nil {
		t.Fatal(err)
	}

	// Default (exclude tests): TestSimple must not appear.
	var sbExclude strings.Builder
	codeExclude := Run(Config{
		Path:         dir,
		CoverageFile: lcovPath,
		// IncludeTests defaults to false.
	}, &sbExclude)
	if codeExclude != 0 {
		t.Fatalf("Run (exclude tests) returned code %d; output:\n%s", codeExclude, sbExclude.String())
	}
	if strings.Contains(sbExclude.String(), "TestSimple") {
		t.Errorf("default mode: TestSimple must not appear in output; got:\n%s", sbExclude.String())
	}

	// IncludeTests = true: TestSimple must appear.
	var sbInclude strings.Builder
	codeInclude := Run(Config{
		Path:         dir,
		CoverageFile: lcovPath,
		IncludeTests: true,
	}, &sbInclude)
	if codeInclude != 0 {
		t.Fatalf("Run (include tests) returned code %d; output:\n%s", codeInclude, sbInclude.String())
	}
	if !strings.Contains(sbInclude.String(), "TestSimple") {
		t.Errorf("IncludeTests=true: TestSimple must appear in output; got:\n%s", sbInclude.String())
	}
}

// TestRun_CoverageFileMissing verifies that Run returns exit code 1 and a clear
// error message when --coverage points to a non-existent file.
func TestRun_CoverageFileMissing(t *testing.T) {
	dir, _ := setupProject(t)

	var sb strings.Builder
	code := Run(Config{
		Path:         dir,
		CoverageFile: filepath.Join(dir, "does-not-exist.lcov"),
	}, &sb)

	if code == 0 {
		t.Errorf("expected non-zero exit code when coverage file does not exist, got 0")
	}
	out := sb.String()
	if !strings.Contains(out, "does-not-exist.lcov") {
		t.Errorf("expected error to mention the missing file; got:\n%s", out)
	}
}
