package app_test

import (
	"math"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/MurasatoNaoya/crap4x/internal/app"
	"github.com/MurasatoNaoya/crap4x/internal/detect"
)

// testdataDir resolves the path to cmd/crap4x/testdata relative to this file.
func testdataDir(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	// thisFile = …/internal/app/app_test.go
	// testdata  = …/cmd/crap4x/testdata
	root := filepath.Join(filepath.Dir(thisFile), "..", "..", "cmd", "crap4x", "testdata")
	abs, err := filepath.Abs(root)
	if err != nil {
		t.Fatalf("resolving testdata path: %v", err)
	}
	return abs
}

// approxEqual returns true if |a-b| < epsilon.
func approxEqual(a, b, epsilon float64) bool {
	return math.Abs(a-b) < epsilon
}

// TestAnalyze_Integration is the end-to-end integration test for the Analyze
// orchestration function. It exercises:
//   - File walking (finds sample.go inside the testdata directory)
//   - lcov ingestion (reads sample.lcov)
//   - CC computation via tree-sitter (Go spec)
//   - CRAP computation and ranking
//
// Expected values (hand-computed):
//
//	simple:  CC=1, cov=1.0  → CRAP = 1²*(1-1)³  + 1 = 1.0
//	branchy: CC=4, cov=0.25 → CRAP = 4²*(0.75)³ + 4 = 16*0.421875 + 4 = 10.75
//
// The testdata lcov (sample.lcov) covers line 4 of simple (1/1 → 100%) and
// lines 8,9,10,14 of branchy with only line 8 hit (1/4 → 25%).
func TestAnalyze_Integration(t *testing.T) {
	td := testdataDir(t)
	lcov := filepath.Join(td, "sample.lcov")

	results, err := app.Analyze(app.Options{
		Path:         td,
		Langs:        []detect.Lang{detect.Go},
		CoverageLcov: lcov,
	})
	if err != nil {
		t.Fatalf("Analyze returned unexpected error: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	// Results are sorted by CRAP descending; branchy must rank first.
	first := results[0]
	second := results[1]

	// --- branchy: rank 1, CC=4, cov=25%, CRAP=10.75 ---
	if first.Func.Name != "branchy" {
		t.Errorf("expected branchy to rank first (highest CRAP), got %q", first.Func.Name)
	}
	if first.Func.Complexity != 4 {
		t.Errorf("branchy: expected CC=4, got %d", first.Func.Complexity)
	}
	if !approxEqual(first.Coverage, 0.25, 1e-9) {
		t.Errorf("branchy: expected coverage=0.25, got %f", first.Coverage)
	}
	wantBranchyCRAP := 10.75
	if !approxEqual(first.CRAP, wantBranchyCRAP, 1e-9) {
		t.Errorf("branchy: expected CRAP=%.4f, got %.4f", wantBranchyCRAP, first.CRAP)
	}

	// --- simple: rank 2, CC=1, cov=100%, CRAP=1.0 ---
	if second.Func.Name != "simple" {
		t.Errorf("expected simple to rank second, got %q", second.Func.Name)
	}
	if second.Func.Complexity != 1 {
		t.Errorf("simple: expected CC=1, got %d", second.Func.Complexity)
	}
	if !approxEqual(second.Coverage, 1.0, 1e-9) {
		t.Errorf("simple: expected coverage=1.0, got %f", second.Coverage)
	}
	wantSimpleCRAP := 1.0
	if !approxEqual(second.CRAP, wantSimpleCRAP, 1e-9) {
		t.Errorf("simple: expected CRAP=%.4f, got %.4f", wantSimpleCRAP, second.CRAP)
	}
}

// TestAnalyze_NoCoverage verifies that Analyze returns an informative error
// (not a panic) when CoverageLcov is not supplied.
func TestAnalyze_NoCoverage(t *testing.T) {
	td := testdataDir(t)
	_, err := app.Analyze(app.Options{
		Path:  td,
		Langs: []detect.Lang{detect.Go},
	})
	if err == nil {
		t.Fatal("expected an error when CoverageLcov is empty, got nil")
	}
	errStr := err.Error()
	if len(errStr) < 10 {
		t.Errorf("error message looks too short: %q", errStr)
	}
}

// TestAnalyze_ExcludesTestFiles verifies that by default test files are not
// scored, and that IncludeTests=true restores the previous behaviour.
func TestAnalyze_ExcludesTestFiles(t *testing.T) {
	// Build a temp dir with a production file, a test file, and an lcov.
	dir := t.TempDir()

	const prodSource = `package mypkg

func Prod(x int) int {
	return x + 1
}
`
	const testSource = `package mypkg

import "testing"

func TestProd(t *testing.T) {
	if Prod(1) != 2 {
		t.Fatal("bad")
	}
}
`
	// lcov covers both functions (prod on line 4, TestProd on line 6).
	const lcovSrc = `SF:prod.go
DA:4,1
end_of_record
SF:prod_test.go
DA:6,1
end_of_record
`
	writeFile := func(name, content string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	writeFile("prod.go", prodSource)
	writeFile("prod_test.go", testSource)
	lcovPath := filepath.Join(dir, "cov.lcov")
	writeFile("cov.lcov", lcovSrc)

	// Default: IncludeTests = false. TestProd must not appear.
	results, err := app.Analyze(app.Options{
		Path:         dir,
		Langs:        []detect.Lang{detect.Go},
		CoverageLcov: lcovPath,
		// IncludeTests defaults to false.
	})
	if err != nil {
		t.Fatalf("Analyze (exclude tests) error: %v", err)
	}
	for _, r := range results {
		if r.Func.Name == "TestProd" {
			t.Errorf("default mode: TestProd must not appear in results, got: %v", r)
		}
	}
	foundProd := false
	for _, r := range results {
		if r.Func.Name == "Prod" {
			foundProd = true
		}
	}
	if !foundProd {
		t.Error("default mode: Prod (production function) must appear in results")
	}

	// IncludeTests = true: TestProd must appear.
	resultsWithTests, err := app.Analyze(app.Options{
		Path:         dir,
		Langs:        []detect.Lang{detect.Go},
		CoverageLcov: lcovPath,
		IncludeTests: true,
	})
	if err != nil {
		t.Fatalf("Analyze (include tests) error: %v", err)
	}
	foundTest := false
	for _, r := range resultsWithTests {
		if r.Func.Name == "TestProd" {
			foundTest = true
		}
	}
	if !foundTest {
		t.Error("IncludeTests=true: TestProd must appear in results")
	}
}

// TestAnalyze_AutoDetect verifies that language auto-detection (Langs empty)
// works when a go.mod is present. We use the repo root itself as the path.
func TestAnalyze_AutoDetect(t *testing.T) {
	td := testdataDir(t)
	lcov := filepath.Join(td, "sample.lcov")

	// Auto-detect from testdata dir. testdata has no go.mod so detection yields
	// nothing; the test verifies the error path is informative (not a panic).
	_, err := app.Analyze(app.Options{
		Path:         td,
		CoverageLcov: lcov,
		// Langs intentionally omitted: triggers auto-detection.
	})
	// testdata has no go.mod/Cargo.toml/etc., so we expect "no supported languages" error.
	if err == nil {
		t.Fatal("expected error when no language markers are present in testdata, got nil")
	}
}
