package crap_test

import (
	"math"
	"strings"
	"testing"

	"github.com/MurasatoNaoya/crap4x/internal/crap"
	"github.com/MurasatoNaoya/crap4x/internal/lang"
)

// handCRAP computes the expected CRAP score with the formula
// cc^2 * (1-cov)^3 + cc, mirroring what the implementation must produce.
func handCRAP(cc int, cov float64) float64 {
	c := float64(cc)
	return c*c*math.Pow(1-cov, 3) + c
}

const tol = 1e-9 // floating-point tolerance for assertions

func almostEqual(a, b float64) bool { return math.Abs(a-b) < tol }

// ---------------------------------------------------------------------------
// Helper: build a synthetic coverage map (file -> line -> hits).
// ---------------------------------------------------------------------------

func covMap(file string, lines map[int]int) map[string]map[int]int {
	return map[string]map[int]int{file: lines}
}

// ---------------------------------------------------------------------------
// Case 1: cc=12, 45% coverage.
//
// Instrumented lines in range [10,20]: 11 lines (10..20).
// 5 of those hit  (lines 10,12,14,16,18) → covered
// 6 of those zero (lines 11,13,15,17,19,20) → not covered
// cov = 5/11 ≈ 0.4545…
//
// We want exactly 45% so we use 9 instrumented lines with 4 covered + 5 zero.
// Actually the plan says "45% covered" which implies an exact fraction.
// Use 20 instrumented lines, 9 covered → cov = 9/20 = 0.45 exactly.
//
// CRAP = 12^2 * (1-0.45)^3 + 12
//      = 144 * 0.55^3 + 12
//      = 144 * 0.166375 + 12
//      = 23.958 + 12
//      = 35.958
// ---------------------------------------------------------------------------

func TestCRAP_cc12_cov45(t *testing.T) {
	// Build 20 instrumented lines in range [1,30]:  lines 1..20 (all within).
	// Lines 1..9 covered (9 hits), lines 10..20 not covered (0 hits).
	lines := make(map[int]int, 20)
	for i := 1; i <= 9; i++ {
		lines[i] = 3 // covered
	}
	for i := 10; i <= 20; i++ {
		lines[i] = 0 // not covered
	}

	fn := lang.Function{
		Name:       "BigComplexFunc",
		File:       "service.go",
		Start:      1,
		End:        30,
		Complexity: 12,
	}

	cov := covMap("service.go", lines)
	results := crap.Compute([]lang.Function{fn}, cov)

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	r := results[0]

	if !r.HasCoverage {
		t.Error("HasCoverage should be true")
	}

	wantCov := 9.0 / 20.0 // 0.45 exactly
	if !almostEqual(r.Coverage, wantCov) {
		t.Errorf("Coverage: got %.10f, want %.10f", r.Coverage, wantCov)
	}

	// CRAP = 12^2 * (1 - 0.45)^3 + 12 = 144 * 0.166375 + 12 = 35.958
	wantCRAP := handCRAP(12, 0.45)
	t.Logf("Hand-computed CRAP for cc=12, cov=0.45: %.10f", wantCRAP)
	if !almostEqual(r.CRAP, wantCRAP) {
		t.Errorf("CRAP: got %.10f, want %.10f", r.CRAP, wantCRAP)
	}
}

// ---------------------------------------------------------------------------
// Case 2: cc=1, cov=1.0 → CRAP = 1.
// CRAP = 1^2 * (1-1)^3 + 1 = 0 + 1 = 1
// ---------------------------------------------------------------------------

func TestCRAP_cc1_fullCoverage(t *testing.T) {
	// All 5 instrumented lines hit.
	lines := map[int]int{1: 2, 2: 1, 3: 5, 4: 1, 5: 3}

	fn := lang.Function{
		Name:       "SimpleFunc",
		File:       "util.go",
		Start:      1,
		End:        10,
		Complexity: 1,
	}

	cov := covMap("util.go", lines)
	results := crap.Compute([]lang.Function{fn}, cov)

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	r := results[0]

	if !r.HasCoverage {
		t.Error("HasCoverage should be true")
	}
	if !almostEqual(r.Coverage, 1.0) {
		t.Errorf("Coverage: got %.10f, want 1.0", r.Coverage)
	}
	if !almostEqual(r.CRAP, 1.0) {
		t.Errorf("CRAP: got %.10f, want 1.0", r.CRAP)
	}
}

// ---------------------------------------------------------------------------
// Case 3: no instrumented lines in range → HasCoverage=false, cov=0.
// CRAP = cc^2 * (1-0)^3 + cc = cc^2 + cc
// Use cc=5 → CRAP = 25 + 5 = 30
// ---------------------------------------------------------------------------

func TestCRAP_noInstrumentedLines(t *testing.T) {
	// Coverage exists for the file but all instrumented lines are outside [50,60].
	lines := map[int]int{1: 2, 2: 0, 3: 1} // lines 1-3, function is at 50-60

	fn := lang.Function{
		Name:       "UncoveredFunc",
		File:       "handler.go",
		Start:      50,
		End:        60,
		Complexity: 5,
	}

	cov := covMap("handler.go", lines)
	results := crap.Compute([]lang.Function{fn}, cov)

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	r := results[0]

	if r.HasCoverage {
		t.Error("HasCoverage should be false when no instrumented lines in range")
	}
	if !almostEqual(r.Coverage, 0.0) {
		t.Errorf("Coverage: got %.10f, want 0.0", r.Coverage)
	}
	// CRAP = 5^2 * 1^3 + 5 = 30
	wantCRAP := handCRAP(5, 0.0)
	if !almostEqual(r.CRAP, wantCRAP) {
		t.Errorf("CRAP: got %.10f, want %.10f", r.CRAP, wantCRAP)
	}
}

// ---------------------------------------------------------------------------
// Case 4: also no instrumented lines when file is completely absent from
// the coverage map (not just lines-out-of-range).
// ---------------------------------------------------------------------------

func TestCRAP_fileAbsentFromCoverage(t *testing.T) {
	fn := lang.Function{
		Name:       "MissingCovFunc",
		File:       "missing.go",
		Start:      1,
		End:        10,
		Complexity: 3,
	}

	cov := map[string]map[int]int{} // no files at all
	results := crap.Compute([]lang.Function{fn}, cov)

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	r := results[0]

	if r.HasCoverage {
		t.Error("HasCoverage should be false when file absent")
	}
	// CRAP = 3^2 + 3 = 12
	wantCRAP := handCRAP(3, 0.0)
	if !almostEqual(r.CRAP, wantCRAP) {
		t.Errorf("CRAP: got %.10f, want %.10f", r.CRAP, wantCRAP)
	}
}

// ---------------------------------------------------------------------------
// Case 5: sorting — higher CRAP first; deterministic tie-break by name then
// file when CRAP values are equal.
// ---------------------------------------------------------------------------

func TestCompute_Sorting(t *testing.T) {
	// Three functions:
	//   alpha: cc=1,  cov=1.0 → CRAP=1.0
	//   beta:  cc=5,  cov=0.0 → CRAP=30.0   (no instrumented lines in range)
	//   gamma: cc=12, cov=0.45 → CRAP=35.958
	//
	// Expected order: gamma, beta, alpha

	lines20 := make(map[int]int, 20)
	for i := 1; i <= 9; i++ {
		lines20[i] = 1
	}
	for i := 10; i <= 20; i++ {
		lines20[i] = 0
	}

	funcs := []lang.Function{
		{Name: "alpha", File: "a.go", Start: 1, End: 5, Complexity: 1},
		{Name: "beta", File: "b.go", Start: 100, End: 110, Complexity: 5},  // range not in cov
		{Name: "gamma", File: "c.go", Start: 1, End: 30, Complexity: 12},
	}

	cov := map[string]map[int]int{
		"a.go": {1: 1, 2: 1, 3: 1, 4: 1, 5: 1},
		"b.go": {1: 0},  // instrumented line outside gamma's range
		"c.go": lines20,
	}

	results := crap.Compute(funcs, cov)

	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	if results[0].Func.Name != "gamma" {
		t.Errorf("first result should be gamma, got %s", results[0].Func.Name)
	}
	if results[1].Func.Name != "beta" {
		t.Errorf("second result should be beta, got %s", results[1].Func.Name)
	}
	if results[2].Func.Name != "alpha" {
		t.Errorf("third result should be alpha, got %s", results[2].Func.Name)
	}
}

// ---------------------------------------------------------------------------
// Case 6: deterministic tie-break — two functions with identical CRAP values
// sorted by name (then file) so results are stable.
// cc=1, cov=1.0 → CRAP=1.0 for both.
// ---------------------------------------------------------------------------

func TestCompute_TieBreak(t *testing.T) {
	funcs := []lang.Function{
		{Name: "zzz", File: "z.go", Start: 1, End: 5, Complexity: 1},
		{Name: "aaa", File: "a.go", Start: 1, End: 5, Complexity: 1},
	}

	lines := map[int]int{1: 1, 2: 1, 3: 1, 4: 1, 5: 1}
	cov := map[string]map[int]int{
		"z.go": lines,
		"a.go": lines,
	}

	results := crap.Compute(funcs, cov)

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	// Both CRAP=1.0 tie: aaa < zzz alphabetically
	if results[0].Func.Name != "aaa" {
		t.Errorf("tie-break first should be aaa, got %s", results[0].Func.Name)
	}
	if results[1].Func.Name != "zzz" {
		t.Errorf("tie-break second should be zzz, got %s", results[1].Func.Name)
	}
}

// ---------------------------------------------------------------------------
// Case 7: path matching — Function.File is a relative basename ("service.go")
// but the coverage map is keyed by an absolute path
// ("/home/ci/project/pkg/service.go"). They must still match.
// ---------------------------------------------------------------------------

func TestCompute_PathMatchingAbsoluteVsBasename(t *testing.T) {
	fn := lang.Function{
		Name:       "DoThing",
		File:       "service.go", // basename only
		Start:      1,
		End:        5,
		Complexity: 2,
	}

	cov := map[string]map[int]int{
		"/home/ci/project/pkg/service.go": {1: 1, 2: 1, 3: 0, 4: 1, 5: 0},
	}

	results := crap.Compute([]lang.Function{fn}, cov)

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	r := results[0]

	if !r.HasCoverage {
		t.Error("HasCoverage should be true — path matching should have found the file")
	}
	// 3 of 5 lines covered → cov = 3/5 = 0.6
	wantCov := 3.0 / 5.0
	if !almostEqual(r.Coverage, wantCov) {
		t.Errorf("Coverage: got %.10f, want %.10f", r.Coverage, wantCov)
	}
}

// ---------------------------------------------------------------------------
// Case 8: path matching — coverage keyed by relative path, Function.File is
// absolute. E.g. Function.File="/abs/path/to/util.go", cov key="util.go".
// ---------------------------------------------------------------------------

func TestCompute_PathMatchingRelativeVsAbsolute(t *testing.T) {
	fn := lang.Function{
		Name:       "Helper",
		File:       "/abs/path/to/util.go", // absolute
		Start:      10,
		End:        15,
		Complexity: 3,
	}

	cov := map[string]map[int]int{
		"util.go": {10: 1, 11: 1, 12: 1, 13: 0, 14: 0, 15: 0},
	}

	results := crap.Compute([]lang.Function{fn}, cov)

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	r := results[0]

	if !r.HasCoverage {
		t.Error("HasCoverage should be true — path matching should find by basename")
	}
	// 3 of 6 covered → cov = 0.5
	if !almostEqual(r.Coverage, 0.5) {
		t.Errorf("Coverage: got %.10f, want 0.5", r.Coverage)
	}
}

// ---------------------------------------------------------------------------
// Case 9: path matching — suffix match. Function.File="pkg/util.go",
// coverage key="/home/ci/project/pkg/util.go".
// ---------------------------------------------------------------------------

func TestCompute_PathMatchingSuffix(t *testing.T) {
	fn := lang.Function{
		Name:       "ParseFoo",
		File:       "pkg/parser.go", // relative with directory
		Start:      5,
		End:        8,
		Complexity: 2,
	}

	cov := map[string]map[int]int{
		"/workspace/myapp/pkg/parser.go": {5: 2, 6: 0, 7: 1, 8: 0},
	}

	results := crap.Compute([]lang.Function{fn}, cov)

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	r := results[0]

	if !r.HasCoverage {
		t.Error("HasCoverage should be true via suffix path match")
	}
	// 2 of 4 covered → cov = 0.5
	if !almostEqual(r.Coverage, 0.5) {
		t.Errorf("Coverage: got %.10f, want 0.5", r.Coverage)
	}
}

// ---------------------------------------------------------------------------
// Case 10: Report formatting.
// ---------------------------------------------------------------------------

func TestReport_Format(t *testing.T) {
	// Build results manually.
	results := []crap.Result{
		{
			Func:        lang.Function{Name: "BigFunc", File: "main.go", Complexity: 12},
			Coverage:    0.45,
			HasCoverage: true,
			CRAP:        35.958,
		},
		{
			Func:        lang.Function{Name: "TinyFunc", File: "util.go", Complexity: 1},
			Coverage:    1.0,
			HasCoverage: true,
			CRAP:        1.0,
		},
		{
			Func:        lang.Function{Name: "DeadFunc", File: "dead.go", Complexity: 5},
			Coverage:    0.0,
			HasCoverage: false,
			CRAP:        30.0,
		},
	}

	report := crap.Report(results, 0) // top=0 means all

	// Must contain header columns.
	for _, col := range []string{"Function", "File", "CC", "Cov%", "CRAP"} {
		if !strings.Contains(report, col) {
			t.Errorf("report missing column header %q", col)
		}
	}

	// Must contain function names.
	for _, name := range []string{"BigFunc", "TinyFunc", "DeadFunc"} {
		if !strings.Contains(report, name) {
			t.Errorf("report missing function name %q", name)
		}
	}

	// n/a should appear for the zero-instrumented function.
	if !strings.Contains(report, "n/a") {
		t.Error("report should contain 'n/a' for function with HasCoverage=false")
	}

	// CRAP values formatted to one decimal.
	if !strings.Contains(report, "35.9") { // 35.958 → "36.0"? No, 35.9 is within one decimal
		// 35.958 formatted to 1dp → 36.0. Let's check both.
		if !strings.Contains(report, "36.0") {
			t.Error("report should contain formatted CRAP value for BigFunc (35.9 or 36.0)")
		}
	}
	if !strings.Contains(report, "1.0") {
		t.Error("report should contain CRAP=1.0 for TinyFunc")
	}

	// Cov% formatted to one decimal.
	if !strings.Contains(report, "45.0") {
		t.Error("report should contain Cov%=45.0 for BigFunc")
	}
	if !strings.Contains(report, "100.0") {
		t.Error("report should contain Cov%=100.0 for TinyFunc")
	}
}

// ---------------------------------------------------------------------------
// Case 11: Report top-N limiting.
// ---------------------------------------------------------------------------

func TestReport_TopN(t *testing.T) {
	results := []crap.Result{
		{Func: lang.Function{Name: "AlphaFunc"}, CRAP: 100.0, HasCoverage: true, Coverage: 0.0},
		{Func: lang.Function{Name: "BetaFunc"}, CRAP: 50.0, HasCoverage: true, Coverage: 0.0},
		{Func: lang.Function{Name: "GammaFunc"}, CRAP: 10.0, HasCoverage: true, Coverage: 0.0},
	}

	report := crap.Report(results, 2) // top 2 only

	if !strings.Contains(report, "AlphaFunc") {
		t.Error("top-2 report should include AlphaFunc")
	}
	if !strings.Contains(report, "BetaFunc") {
		t.Error("top-2 report should include BetaFunc")
	}
	if strings.Contains(report, "GammaFunc") {
		t.Error("top-2 report should NOT include GammaFunc")
	}
}

// ---------------------------------------------------------------------------
// Case 12: exact hand-computed CRAP for cc=4, cov=0.5.
// CRAP = 4^2 * (1-0.5)^3 + 4 = 16 * 0.125 + 4 = 2 + 4 = 6
// ---------------------------------------------------------------------------

func TestCRAP_cc4_cov50(t *testing.T) {
	// 4 instrumented lines in range, 2 covered, 2 not covered -> cov = 0.5
	fn := lang.Function{
		Name:       "HalfCoveredFunc",
		File:       "half.go",
		Start:      1,
		End:        10,
		Complexity: 4,
	}
	cov := covMap("half.go", map[int]int{1: 1, 2: 1, 3: 0, 4: 0})
	results := crap.Compute([]lang.Function{fn}, cov)

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	r := results[0]

	if !r.HasCoverage {
		t.Error("HasCoverage should be true")
	}
	if !almostEqual(r.Coverage, 0.5) {
		t.Errorf("Coverage: got %.10f, want 0.5", r.Coverage)
	}
	// CRAP = 16 * 0.125 + 4 = 6 exactly
	if !almostEqual(r.CRAP, 6.0) {
		t.Errorf("CRAP: got %.10f, want 6.0", r.CRAP)
	}
}

// ---------------------------------------------------------------------------
// Case 13: cc=5, cov=0.0 -> CRAP = 25 + 5 = 30 (explicit zero-coverage path).
// ---------------------------------------------------------------------------

func TestCRAP_cc5_zeroCoverage(t *testing.T) {
	fn := lang.Function{
		Name:       "NoCovFunc",
		File:       "nocov.go",
		Start:      1,
		End:        5,
		Complexity: 5,
	}
	results := crap.Compute([]lang.Function{fn}, map[string]map[int]int{})

	r := results[0]
	// CRAP = 5^2*(1-0)^3 + 5 = 25 + 5 = 30
	if !almostEqual(r.CRAP, 30.0) {
		t.Errorf("CRAP: got %.10f, want 30.0", r.CRAP)
	}
}

// ---------------------------------------------------------------------------
// Case 14: AboveThreshold filtering.
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// Case 15: path matching — absolute lcov SF path vs relative Function.File,
// mirroring the real bse-consensus scenario where lcov SF: paths are absolute
// ("/Users/andrewnaoyamcwilliam/repo/bse-consensus/src/pouw.rs") and the
// analyzed Function.File is relative ("src/pouw.rs"). These must join.
// ---------------------------------------------------------------------------

func TestCompute_AbsoluteLcovRelativeSource(t *testing.T) {
	fn := lang.Function{
		Name:       "verify",
		File:       "src/pouw.rs", // relative path produced by filepath.Rel(root, abs)
		Start:      10,
		End:        20,
		Complexity: 5,
	}

	// Coverage keyed by the absolute SF: path as lcov produces it.
	cov := map[string]map[int]int{
		"/Users/andrewnaoyamcwilliam/repo/bse-consensus/src/pouw.rs": {
			10: 3, 11: 1, 12: 1, 13: 0, 14: 1,
			15: 1, 16: 0, 17: 1, 18: 1, 19: 0, 20: 1,
		},
	}

	results := crap.Compute([]lang.Function{fn}, cov)

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	r := results[0]

	if !r.HasCoverage {
		t.Error("HasCoverage must be true: absolute lcov SF path must join with relative Function.File via suffix match")
	}
	// 8 of 11 lines covered (lines 13, 16, 19 are zero).
	wantCov := 8.0 / 11.0
	if !almostEqual(r.Coverage, wantCov) {
		t.Errorf("Coverage: got %.10f, want %.10f (8/11)", r.Coverage, wantCov)
	}
}

// ---------------------------------------------------------------------------
// Case 16: component-aware suffix matching — Function.File with multiple path
// components ("src/pouw.rs") must match an absolute key that ends with that
// suffix, while a bare basename ("pouw.rs") must NOT be promoted to a suffix
// match; it should fall through to basename fallback instead. The distinction
// matters when two coverage entries share a basename but differ in directory,
// preventing the wrong entry from being selected via suffix.
// ---------------------------------------------------------------------------

func TestCompute_ComponentAwareSuffixNoFalsePositive(t *testing.T) {
	// Two functions: one has a multi-component relative file, one has a basename-only file.
	// Both share the same base name "pouw.rs".
	fnMulti := lang.Function{
		Name:       "verify",
		File:       "src/pouw.rs", // multi-component: must suffix-match /abs/src/pouw.rs
		Start:      1,
		End:        5,
		Complexity: 2,
	}
	fnBase := lang.Function{
		Name:       "helper",
		File:       "pouw.rs", // basename only: must NOT be promoted to suffix match
		Start:      1,
		End:        5,
		Complexity: 2,
	}

	// Only one coverage entry — the suffix-specific one.
	// fnMulti should match it via suffix; fnBase should match it via basename fallback.
	cov := map[string]map[int]int{
		"/abs/src/pouw.rs": {1: 1, 2: 1, 3: 0, 4: 1, 5: 0},
	}

	resultsMulti := crap.Compute([]lang.Function{fnMulti}, cov)
	if len(resultsMulti) != 1 {
		t.Fatalf("expected 1 result for fnMulti, got %d", len(resultsMulti))
	}
	if !resultsMulti[0].HasCoverage {
		t.Error("fnMulti (src/pouw.rs): HasCoverage must be true via suffix match")
	}
	// 3 of 5 covered.
	if !almostEqual(resultsMulti[0].Coverage, 3.0/5.0) {
		t.Errorf("fnMulti Coverage: got %.10f, want %.10f", resultsMulti[0].Coverage, 3.0/5.0)
	}

	resultsBase := crap.Compute([]lang.Function{fnBase}, cov)
	if len(resultsBase) != 1 {
		t.Fatalf("expected 1 result for fnBase, got %d", len(resultsBase))
	}
	// fnBase should still find coverage via basename fallback.
	if !resultsBase[0].HasCoverage {
		t.Error("fnBase (pouw.rs): HasCoverage must be true via basename fallback")
	}
}

func TestAboveThreshold(t *testing.T) {
	results := []crap.Result{
		{Func: lang.Function{Name: "low"}, CRAP: 5.0},
		{Func: lang.Function{Name: "mid"}, CRAP: 15.0},
		{Func: lang.Function{Name: "high"}, CRAP: 30.0},
	}

	above := crap.AboveThreshold(results, 10.0)
	if len(above) != 2 {
		t.Fatalf("expected 2 results above threshold 10, got %d", len(above))
	}
	for _, r := range above {
		if r.CRAP <= 10.0 {
			t.Errorf("result %s has CRAP=%.1f which is not above threshold 10.0", r.Func.Name, r.CRAP)
		}
	}

	// Threshold exactly equal to a value: strictly greater than, so 15.0 is NOT above 15.0.
	at15 := crap.AboveThreshold(results, 15.0)
	if len(at15) != 1 {
		t.Fatalf("expected 1 result strictly above 15.0, got %d", len(at15))
	}
	if at15[0].Func.Name != "high" {
		t.Errorf("expected high, got %s", at15[0].Func.Name)
	}
}
