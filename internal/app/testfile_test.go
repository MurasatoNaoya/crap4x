package app

import (
	"testing"

	"github.com/MurasatoNaoya/crap4x/internal/detect"
)

func TestIsTestFile(t *testing.T) {
	tests := []struct {
		name string
		path string
		lang detect.Lang
		want bool
	}{
		// Go: _test.go suffix
		{name: "Go test file", path: "store_test.go", lang: detect.Go, want: true},
		{name: "Go test file nested", path: "internal/store/store_test.go", lang: detect.Go, want: true},
		{name: "Go source file", path: "store.go", lang: detect.Go, want: false},
		{name: "Go main", path: "main.go", lang: detect.Go, want: false},

		// Python: test_*.py, *_test.py, conftest.py, tests/ or test/ dir component
		{name: "Python test_ prefix", path: "test_foo.py", lang: detect.Python, want: true},
		{name: "Python _test suffix", path: "foo_test.py", lang: detect.Python, want: true},
		{name: "Python conftest", path: "conftest.py", lang: detect.Python, want: true},
		{name: "Python conftest nested", path: "mypackage/conftest.py", lang: detect.Python, want: true},
		{name: "Python under tests/ dir", path: "tests/integration.py", lang: detect.Python, want: true},
		{name: "Python under test/ dir", path: "test/unit.py", lang: detect.Python, want: true},
		{name: "Python nested under tests/", path: "pkg/tests/foo.py", lang: detect.Python, want: true},
		{name: "Python plain source", path: "app.py", lang: detect.Python, want: false},
		{name: "Python utils", path: "utils/helper.py", lang: detect.Python, want: false},

		// Rust: tests/ directory component (integration tests)
		{name: "Rust under tests/ dir", path: "tests/integration.rs", lang: detect.Rust, want: true},
		{name: "Rust under tests/ nested", path: "crate/tests/foo.rs", lang: detect.Rust, want: true},
		{name: "Rust src lib", path: "src/lib.rs", lang: detect.Rust, want: false},
		{name: "Rust main", path: "src/main.rs", lang: detect.Rust, want: false},
		{name: "Rust mod", path: "src/parser.rs", lang: detect.Rust, want: false},

		// Cross-lang: Go test file not matched for Python lang
		{name: "Go test.go not test for Python", path: "store_test.go", lang: detect.Python, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isTestFile(tt.path, tt.lang)
			if got != tt.want {
				t.Errorf("isTestFile(%q, %v) = %v, want %v", tt.path, tt.lang, got, tt.want)
			}
		})
	}
}
