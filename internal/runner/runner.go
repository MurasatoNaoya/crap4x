// Package runner provides the default shell commands for generating lcov
// coverage files and a thin wrapper for executing them. Auto-run is an
// optional convenience; the primary coverage-ingestion path in crap4x is
// --coverage <file.lcov>, which accepts a pre-generated lcov file directly.
package runner

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/MurasatoNaoya/crap4x/internal/detect"
)

// DefaultCoverageCommand returns the shell command (suitable for passing to
// "sh -c") that will produce an lcov file at outPath for language l.
//
// Go note: `go test` natively produces a Go coverprofile (not lcov). The
// command returned here writes a Go coverprofile to outPath, documenting the
// expected conversion step. For true lcov output, users should run the
// returned `go test` command and then convert with a tool such as `gcov2lcov`
// (github.com/jandelgado/gcov2lcov) or `gocover-cobertura`, then pass the
// resulting lcov file via --coverage. This separation is intentional: v1 of
// crap4x treats --coverage <lcov> as the first-class ingest path; the Go
// auto-run command is a documented starting point, not a full pipeline.
//
// Python: requires the `coverage` package (pip install coverage pytest).
// Rust:   requires `cargo-llvm-cov` (cargo install cargo-llvm-cov).
func DefaultCoverageCommand(l detect.Lang, outPath string) string {
	switch l {
	case detect.Go:
		// Produces a Go coverprofile at outPath.
		// Convert to lcov with: gcov2lcov -infile <outPath> -outfile <outPath>.lcov
		// Then pass the .lcov file via --coverage.
		return fmt.Sprintf("go test ./... -coverprofile=%s", outPath)
	case detect.Python:
		return fmt.Sprintf("coverage run -m pytest && coverage lcov -o %s", outPath)
	case detect.Rust:
		return fmt.Sprintf("cargo llvm-cov --lcov --output-path %s", outPath)
	default:
		return ""
	}
}

// Run executes the given command string in dir using the system shell
// (sh -c on Unix). Stdout and stderr are forwarded to the parent process so
// the user can observe progress. Run is called only when the user opts into
// auto-run via --test-command or the equivalent flag; it is never invoked on
// the --coverage (file ingest) path.
func Run(dir, command string) error {
	cmd := exec.Command("sh", "-c", command)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
