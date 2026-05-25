package runner_test

import (
	"strings"
	"testing"

	"github.com/MurasatoNaoya/crap4x/internal/detect"
	"github.com/MurasatoNaoya/crap4x/internal/runner"
)

func TestDefaultCoverageCommand_Go(t *testing.T) {
	cmd := runner.DefaultCoverageCommand(detect.Go, "/tmp/out.lcov")
	// Go path: go test produces a coverprofile; command must mention go test and
	// the output path so callers know what to pass to --coverage.
	if !strings.Contains(cmd, "go test") {
		t.Errorf("Go command %q does not contain 'go test'", cmd)
	}
	if !strings.Contains(cmd, "/tmp/out.lcov") {
		t.Errorf("Go command %q does not contain the output path", cmd)
	}
}

func TestDefaultCoverageCommand_Python(t *testing.T) {
	cmd := runner.DefaultCoverageCommand(detect.Python, "/tmp/out.lcov")
	if !strings.Contains(cmd, "coverage") {
		t.Errorf("Python command %q does not contain 'coverage'", cmd)
	}
	if !strings.Contains(cmd, "/tmp/out.lcov") {
		t.Errorf("Python command %q does not contain the output path", cmd)
	}
	if !strings.Contains(cmd, "lcov") {
		t.Errorf("Python command %q does not contain 'lcov'", cmd)
	}
}

func TestDefaultCoverageCommand_Rust(t *testing.T) {
	cmd := runner.DefaultCoverageCommand(detect.Rust, "/tmp/out.lcov")
	if !strings.Contains(cmd, "cargo") {
		t.Errorf("Rust command %q does not contain 'cargo'", cmd)
	}
	if !strings.Contains(cmd, "/tmp/out.lcov") {
		t.Errorf("Rust command %q does not contain the output path", cmd)
	}
	if !strings.Contains(cmd, "lcov") {
		t.Errorf("Rust command %q does not contain 'lcov'", cmd)
	}
}

func TestDefaultCoverageCommand_OutPathSubstitution(t *testing.T) {
	// Changing outPath changes the returned command.
	a := runner.DefaultCoverageCommand(detect.Python, "/a/cov.lcov")
	b := runner.DefaultCoverageCommand(detect.Python, "/b/cov.lcov")
	if a == b {
		t.Error("commands with different outPath are identical; outPath not substituted")
	}
	if !strings.Contains(a, "/a/cov.lcov") {
		t.Errorf("command %q missing outPath /a/cov.lcov", a)
	}
	if !strings.Contains(b, "/b/cov.lcov") {
		t.Errorf("command %q missing outPath /b/cov.lcov", b)
	}
}
