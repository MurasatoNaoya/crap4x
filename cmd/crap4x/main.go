// Command crap4x analyzes the CRAP (Change Risk Anti-Patterns) score for
// functions across a Go, Python, or Rust project.
//
// Usage:
//
//	crap4x [path] [flags]
//
// Flags:
//
//	--lang go|python|rust   override language detection (repeatable or comma-separated)
//	--coverage <file>       path to an lcov coverage file
//	--threshold <float>     exit 1 if any function CRAP score exceeds this value
//	--top <int>             limit output to the top N functions (default 0 = all)
package main

import (
	"flag"
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

const version = "0.1.0-dev"

// Config holds the parsed CLI configuration passed to Run.
type Config struct {
	// Path is the project root to analyze (default ".").
	Path string
	// Langs overrides language detection when non-empty.
	Langs []detect.Lang
	// CoverageFile is the path to an lcov file (may be "").
	// When empty, functions are computed with zero coverage (CRAP = CC²+CC).
	CoverageFile string
	// Threshold causes a non-zero exit when any CRAP score exceeds this value.
	// 0 means the threshold check is disabled.
	Threshold float64
	// Top limits the report to the top N functions (0 means all).
	Top int
}

// langsFlag implements flag.Value for a repeatable/comma-separated --lang flag.
type langsFlag []detect.Lang

func (lf *langsFlag) String() string {
	if lf == nil || len(*lf) == 0 {
		return ""
	}
	names := make([]string, len(*lf))
	for i, l := range *lf {
		names[i] = l.String()
	}
	return strings.Join(names, ",")
}

func (lf *langsFlag) Set(val string) error {
	for _, part := range strings.Split(val, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		l, ok := detect.ParseLang(part)
		if !ok {
			return fmt.Errorf("unknown language %q (supported: go, python, rust)", part)
		}
		*lf = append(*lf, l)
	}
	return nil
}

// skipDirs is the set of directory names that the file walker never descends into.
var skipDirs = map[string]bool{
	"vendor":       true,
	"target":       true,
	"node_modules": true,
	".git":         true,
}

// Run is the testable orchestration core. It writes output to out and returns
// the exit code (0 or 1). Separating this from main() allows integration tests
// to exercise the full pipeline without spawning a subprocess.
func Run(cfg Config, out *strings.Builder) (int, error) {
	// 1. Resolve the project path.
	root := cfg.Path
	if root == "" {
		root = "."
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return 1, fmt.Errorf("cannot resolve path %q: %w", root, err)
	}

	// 2. Determine languages.
	langs := cfg.Langs
	if len(langs) == 0 {
		detected, err := detect.Detect(absRoot)
		if err != nil {
			return 1, fmt.Errorf("language detection failed: %w", err)
		}
		langs = detected
	}
	if len(langs) == 0 {
		out.WriteString("no supported languages detected; use --lang to specify one\n")
		return 0, nil
	}

	// 3. Walk the tree and analyze source files.
	var funcs []lang.Function

	for _, l := range langs {
		ext := l.Ext()
		spec := l.Spec()

		walkErr := filepath.WalkDir(absRoot, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				if skipDirs[d.Name()] || strings.HasPrefix(d.Name(), ".") {
					return filepath.SkipDir
				}
				return nil
			}
			if filepath.Ext(path) != ext {
				return nil
			}

			src, err := os.ReadFile(path)
			if err != nil {
				return fmt.Errorf("reading %s: %w", path, err)
			}

			// Use a relative path for display and lcov matching.
			rel, relErr := filepath.Rel(absRoot, path)
			if relErr != nil {
				rel = path
			}

			found := lang.Analyze(src, rel, spec)
			funcs = append(funcs, found...)
			return nil
		})
		if walkErr != nil {
			return 1, fmt.Errorf("walking %s: %w", absRoot, walkErr)
		}
	}

	// 4. Parse coverage (optional — nil map means zero coverage).
	var cov map[string]map[int]int
	if cfg.CoverageFile != "" {
		f, err := os.Open(cfg.CoverageFile)
		if err != nil {
			return 1, fmt.Errorf("opening coverage file %q: %w", cfg.CoverageFile, err)
		}
		defer f.Close()
		cov, err = coverage.Parse(f)
		if err != nil {
			return 1, fmt.Errorf("parsing coverage: %w", err)
		}
	}

	// 5. Compute CRAP scores and print report.
	results := crap.Compute(funcs, cov)
	out.WriteString(crap.Report(results, cfg.Top))

	// 6. Threshold check.
	if cfg.Threshold > 0 {
		flagged := crap.AboveThreshold(results, cfg.Threshold)
		if len(flagged) > 0 {
			out.WriteString(fmt.Sprintf(
				"\n%d function(s) exceed CRAP threshold %.1f\n",
				len(flagged), cfg.Threshold,
			))
			return 1, nil
		}
	}

	return 0, nil
}

func main() {
	var langFlag langsFlag
	flag.Var(&langFlag, "lang", "language override (go|python|rust); repeatable or comma-separated")

	coverageFile := flag.String("coverage", "", "path to lcov coverage file (generate per-language commands with --help)")
	threshold := flag.Float64("threshold", 0, "exit 1 when any CRAP score exceeds this value (0 = off)")
	top := flag.Int("top", 0, "limit output to top N functions (0 = all)")
	versionFlag := flag.Bool("version", false, "print version and exit")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: crap4x [path] [flags]\n\n")
		fmt.Fprintf(os.Stderr, "Compute CRAP scores for functions in a Go, Python, or Rust project.\n")
		fmt.Fprintf(os.Stderr, "Version: %s\n\n", version)
		fmt.Fprintf(os.Stderr, "Generate coverage (then pass via --coverage):\n")
		fmt.Fprintf(os.Stderr, "  Go:     go test ./... -coverprofile=cover.out  (convert with gcov2lcov)\n")
		fmt.Fprintf(os.Stderr, "  Python: coverage run -m pytest && coverage lcov -o coverage.lcov\n")
		fmt.Fprintf(os.Stderr, "  Rust:   cargo llvm-cov --lcov --output-path coverage.lcov\n\n")
		fmt.Fprintf(os.Stderr, "Flags:\n")
		flag.PrintDefaults()
	}

	flag.Parse()

	if *versionFlag {
		fmt.Printf("crap4x %s\n", version)
		os.Exit(0)
	}

	path := "."
	if flag.NArg() > 0 {
		path = flag.Arg(0)
	}

	cfg := Config{
		Path:         path,
		Langs:        langFlag,
		CoverageFile: *coverageFile,
		Threshold:    *threshold,
		Top:          *top,
	}

	var sb strings.Builder
	code, err := Run(cfg, &sb)
	if err != nil {
		fmt.Fprintf(os.Stderr, "crap4x: %v\n", err)
		os.Exit(1)
	}
	fmt.Print(sb.String())
	os.Exit(code)
}
