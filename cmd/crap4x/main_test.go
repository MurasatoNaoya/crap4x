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
