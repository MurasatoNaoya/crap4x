// Package crap joins parsed coverage data with analyzed functions, computes the
// CRAP score for each function, and renders a sorted report.
//
// Formula: CRAP(f) = cc(f)^2 * (1 - cov(f))^3 + cc(f)
//
// where cc is the cyclomatic complexity and cov is the fraction of instrumented
// lines within the function's range that were executed at least once (0..1).
// When there are no instrumented lines within the range, cov is treated as 0
// and HasCoverage is set to false; the function still receives a CRAP score.
package crap

import (
	"fmt"
	"math"
	"path/filepath"
	"sort"
	"strings"

	"github.com/MurasatoNaoya/crap4x/internal/lang"
)

// Result holds the computed CRAP data for a single function.
type Result struct {
	Func        lang.Function
	Coverage    float64 // fraction 0..1; 0 when HasCoverage is false
	HasCoverage bool    // false when no instrumented lines exist within [Start,End]
	CRAP        float64
}

// Compute joins each function in funcs with the parsed coverage map
// (file -> line -> hit count), computes coverage fraction and CRAP score,
// and returns results sorted by CRAP descending. Ties are broken
// deterministically by function name ascending, then by file ascending.
//
// Path matching strategy (documented):
//  1. Exact match: Function.File == coverage key.
//  2. Suffix match: the coverage key ends with ("/" + Function.File) or
//     Function.File ends with ("/" + coverage key). This handles the common
//     case where lcov SF: paths are absolute while analyzed paths are relative
//     (or vice versa).
//  3. Basename match: filepath.Base(Function.File) == filepath.Base(coverage key).
//     Used as a last resort; if multiple coverage entries share the same
//     basename the first matching one is used (map iteration order is random,
//     but in practice lcov files rarely have basename collisions).
func Compute(funcs []lang.Function, cov map[string]map[int]int) []Result {
	results := make([]Result, 0, len(funcs))

	for _, fn := range funcs {
		lineCov := findCoverage(fn.File, cov)

		instrumented, covered := countLines(fn.Start, fn.End, lineCov)

		var covFrac float64
		hasCov := instrumented > 0
		if hasCov {
			covFrac = float64(covered) / float64(instrumented)
		}

		results = append(results, Result{
			Func:        fn,
			Coverage:    covFrac,
			HasCoverage: hasCov,
			CRAP:        crapScore(fn.Complexity, covFrac),
		})
	}

	sort.Slice(results, func(i, j int) bool {
		a, b := results[i], results[j]
		if a.CRAP != b.CRAP {
			return a.CRAP > b.CRAP // higher CRAP first
		}
		// Deterministic tie-break: name asc, then file asc.
		if a.Func.Name != b.Func.Name {
			return a.Func.Name < b.Func.Name
		}
		return a.Func.File < b.Func.File
	})

	return results
}

// Report renders results as a text table with columns:
//
//	Function | File | CC | Cov% | CRAP
//
// CRAP and Cov% are formatted to one decimal place. Cov% shows "n/a" when
// HasCoverage is false. If top > 0 only the first top rows are rendered
// (caller should pass results pre-sorted by Compute). top <= 0 means all rows.
func Report(results []Result, top int) string {
	rows := results
	if top > 0 && top < len(rows) {
		rows = rows[:top]
	}

	// Column widths — compute dynamically so the table is readable at any scale.
	const (
		hFunc = "Function"
		hFile = "File"
		hCC   = "CC"
		hCov  = "Cov%"
		hCRAP = "CRAP"
	)

	wFunc, wFile, wCC, wCov, wCRAP := len(hFunc), len(hFile), len(hCC), len(hCov), len(hCRAP)

	type row struct {
		fn, file, cc, cov, crapStr string
	}
	rendered := make([]row, len(rows))
	for i, r := range rows {
		fn := r.Func.Name
		file := r.Func.File
		cc := fmt.Sprintf("%d", r.Func.Complexity)

		var covStr string
		if r.HasCoverage {
			covStr = fmt.Sprintf("%.1f", r.Coverage*100)
		} else {
			covStr = "n/a"
		}
		crapStr := fmt.Sprintf("%.1f", r.CRAP)

		rendered[i] = row{fn, file, cc, covStr, crapStr}

		if len(fn) > wFunc {
			wFunc = len(fn)
		}
		if len(file) > wFile {
			wFile = len(file)
		}
		if len(cc) > wCC {
			wCC = len(cc)
		}
		if len(covStr) > wCov {
			wCov = len(covStr)
		}
		if len(crapStr) > wCRAP {
			wCRAP = len(crapStr)
		}
	}

	sep := fmt.Sprintf("| %s | %s | %s | %s | %s |",
		strings.Repeat("-", wFunc),
		strings.Repeat("-", wFile),
		strings.Repeat("-", wCC),
		strings.Repeat("-", wCov),
		strings.Repeat("-", wCRAP),
	)

	header := fmt.Sprintf("| %-*s | %-*s | %-*s | %-*s | %-*s |",
		wFunc, hFunc,
		wFile, hFile,
		wCC, hCC,
		wCov, hCov,
		wCRAP, hCRAP,
	)

	var sb strings.Builder
	sb.WriteString(header)
	sb.WriteByte('\n')
	sb.WriteString(sep)
	sb.WriteByte('\n')

	for _, r := range rendered {
		line := fmt.Sprintf("| %-*s | %-*s | %-*s | %-*s | %-*s |",
			wFunc, r.fn,
			wFile, r.file,
			wCC, r.cc,
			wCov, r.cov,
			wCRAP, r.crapStr,
		)
		sb.WriteString(line)
		sb.WriteByte('\n')
	}

	return sb.String()
}

// AboveThreshold returns the subset of results whose CRAP score strictly
// exceeds threshold. Callers may use this to drive a non-zero exit code.
func AboveThreshold(results []Result, threshold float64) []Result {
	var out []Result
	for _, r := range results {
		if r.CRAP > threshold {
			out = append(out, r)
		}
	}
	return out
}

// ---------------------------------------------------------------------------
// internal helpers
// ---------------------------------------------------------------------------

// crapScore applies the CRAP formula: cc^2 * (1-cov)^3 + cc.
func crapScore(cc int, cov float64) float64 {
	c := float64(cc)
	return c*c*math.Pow(1-cov, 3) + c
}

// countLines returns (instrumented, covered) counts for lines in [start, end]
// within lineCov. lineCov may be nil (file not found in coverage).
func countLines(start, end int, lineCov map[int]int) (instrumented, covered int) {
	for line, hits := range lineCov {
		if line >= start && line <= end {
			instrumented++
			if hits > 0 {
				covered++
			}
		}
	}
	return
}

// findCoverage finds the line-coverage map for the given file path using the
// three-step matching strategy documented on Compute.
func findCoverage(file string, cov map[string]map[int]int) map[int]int {
	// Step 1: exact match.
	if lc, ok := cov[file]; ok {
		return lc
	}

	fileBase := filepath.Base(file)

	// Step 2 & 3: scan for suffix or basename match.
	// We prefer suffix over basename, so collect suffix matches first.
	var suffixMatch map[int]int
	var baseMatch map[int]int

	for key, lc := range cov {
		// Suffix match in either direction.
		if hasSuffixPath(key, file) || hasSuffixPath(file, key) {
			suffixMatch = lc
			break // take the first suffix match (deterministic for typical lcov)
		}
		// Basename match as fallback.
		if baseMatch == nil && filepath.Base(key) == fileBase {
			baseMatch = lc
		}
	}

	if suffixMatch != nil {
		return suffixMatch
	}
	return baseMatch // nil if nothing matched
}

// hasSuffixPath reports whether longer ends with "/" + shorter, meaning shorter
// is a path suffix of longer (e.g. longer="/abs/pkg/foo.go", shorter="pkg/foo.go").
func hasSuffixPath(longer, shorter string) bool {
	if shorter == "" || longer == "" {
		return false
	}
	// Normalise separators.
	l := filepath.ToSlash(longer)
	s := filepath.ToSlash(shorter)
	return strings.HasSuffix(l, "/"+s)
}
