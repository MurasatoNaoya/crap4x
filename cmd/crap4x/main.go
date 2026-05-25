// Command crap4x analyzes the CRAP (Change Risk Anti-Patterns) score for
// functions across a Go, Python, or Rust project.
//
// Usage:
//
//	crap4x [path] --coverage <file.lcov> [flags]
//
// Flags:
//
//	--lang go|python|rust   override language detection (repeatable or comma-separated)
//	--coverage <file>       path to an lcov coverage file (required)
//	--threshold <float>     exit 1 if any function CRAP score exceeds this value
//	--top <int>             limit output to the top N functions (default 0 = all)
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/MurasatoNaoya/crap4x/internal/app"
	"github.com/MurasatoNaoya/crap4x/internal/crap"
	"github.com/MurasatoNaoya/crap4x/internal/detect"
)

const version = "0.1.0-dev"

// ExitCoverageRequired is the exit code used when --coverage is not supplied.
const ExitCoverageRequired = 2

// Config holds the parsed CLI configuration passed to Run.
type Config struct {
	// Path is the project root to analyze (default ".").
	Path string
	// Langs overrides language detection when non-empty.
	Langs []detect.Lang
	// CoverageFile is the path to an lcov file. It is required; when empty
	// Run prints per-language generation commands and returns ExitCoverageRequired.
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

// detectLangsForPath detects languages in root, returning detected langs (may
// be empty). Errors from detect.Detect are swallowed; an empty slice is
// returned so the caller can still print generic hints.
func detectLangsForPath(root string) []detect.Lang {
	langs, _ := detect.Detect(root)
	return langs
}

// coverageMissingMessage builds the output printed when --coverage is absent.
// It detects the project language(s) from root and prints the exact lcov
// generation commands from the README for each detected language.
func coverageMissingMessage(root string, forced []detect.Lang) string {
	langs := forced
	if len(langs) == 0 {
		langs = detectLangsForPath(root)
	}

	var sb strings.Builder
	sb.WriteString("--coverage is required. Generate an lcov report and pass it via --coverage <file>.\n\n")

	if len(langs) == 0 {
		sb.WriteString("No language detected; commands for all supported languages:\n\n")
		sb.WriteString("  Go:\n")
		sb.WriteString("    go test ./... -coverprofile=cover.out\n")
		sb.WriteString("    gcov2lcov -infile=cover.out -outfile=cover.lcov\n\n")
		sb.WriteString("  Python:\n")
		sb.WriteString("    coverage run -m pytest && coverage lcov -o cover.lcov\n\n")
		sb.WriteString("  Rust:\n")
		sb.WriteString("    cargo llvm-cov --lcov --output-path cover.lcov\n")
	} else {
		sb.WriteString("Detected language(s); run the appropriate command:\n\n")
		for _, l := range langs {
			cmd := app.LcovCommand(l)
			if cmd == "" {
				continue
			}
			sb.WriteString(fmt.Sprintf("  %s:\n", l.String()))
			for _, line := range strings.Split(cmd, "\n") {
				sb.WriteString(fmt.Sprintf("    %s\n", strings.TrimSpace(line)))
			}
			sb.WriteString("\n")
		}
	}

	sb.WriteString("Then re-run: crap4x --coverage cover.lcov\n")
	return sb.String()
}

// Run is the testable orchestration core. It writes output to out and returns
// the exit code (0, 1, or ExitCoverageRequired). Separating this from main()
// allows integration tests to exercise the full pipeline without spawning a
// subprocess.
//
// Exit codes:
//
//	0  success
//	1  analysis error or threshold exceeded
//	2  (ExitCoverageRequired) --coverage flag was not supplied
func Run(cfg Config, out *strings.Builder) int {
	// 1. Resolve the project path.
	root := cfg.Path
	if root == "" {
		root = "."
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		out.WriteString(fmt.Sprintf("crap4x: cannot resolve path %q: %v\n", root, err))
		return 1
	}

	// 2. Guard: --coverage is required.
	if cfg.CoverageFile == "" {
		out.WriteString(coverageMissingMessage(absRoot, cfg.Langs))
		return ExitCoverageRequired
	}

	// 3. Run the analysis via app.Analyze.
	results, err := app.Analyze(app.Options{
		Path:         absRoot,
		Langs:        cfg.Langs,
		CoverageLcov: cfg.CoverageFile,
		Threshold:    cfg.Threshold,
		Top:          cfg.Top,
	})
	if err != nil {
		out.WriteString(fmt.Sprintf("crap4x: %v\n", err))
		return 1
	}

	// 6. Print report.
	out.WriteString(crap.Report(results, cfg.Top))

	// 7. Threshold check.
	if cfg.Threshold > 0 {
		flagged := crap.AboveThreshold(results, cfg.Threshold)
		if len(flagged) > 0 {
			out.WriteString(fmt.Sprintf(
				"\n%d function(s) exceed CRAP threshold %.1f\n",
				len(flagged), cfg.Threshold,
			))
			return 1
		}
	}

	return 0
}

func main() {
	var langFlag langsFlag
	flag.Var(&langFlag, "lang", "language override (go|python|rust); repeatable or comma-separated")

	coverageFile := flag.String("coverage", "", "path to lcov coverage file (required; see README for generation commands)")
	threshold := flag.Float64("threshold", 0, "exit 1 when any CRAP score exceeds this value (0 = off)")
	top := flag.Int("top", 0, "limit output to top N functions (0 = all)")
	versionFlag := flag.Bool("version", false, "print version and exit")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: crap4x [path] --coverage <file.lcov> [flags]\n\n")
		fmt.Fprintf(os.Stderr, "Compute CRAP scores for functions in a Go, Python, or Rust project.\n")
		fmt.Fprintf(os.Stderr, "Version: %s\n\n", version)
		fmt.Fprintf(os.Stderr, "--coverage is required. Generate an lcov file first:\n\n")
		fmt.Fprintf(os.Stderr, "  Go:\n")
		fmt.Fprintf(os.Stderr, "    go test ./... -coverprofile=cover.out\n")
		fmt.Fprintf(os.Stderr, "    gcov2lcov -infile=cover.out -outfile=cover.lcov\n\n")
		fmt.Fprintf(os.Stderr, "  Python:\n")
		fmt.Fprintf(os.Stderr, "    coverage run -m pytest && coverage lcov -o cover.lcov\n\n")
		fmt.Fprintf(os.Stderr, "  Rust:\n")
		fmt.Fprintf(os.Stderr, "    cargo llvm-cov --lcov --output-path cover.lcov\n\n")
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
	code := Run(cfg, &sb)
	fmt.Print(sb.String())
	os.Exit(code)
}
