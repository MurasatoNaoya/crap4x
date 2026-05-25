package detect_test

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/MurasatoNaoya/crap4x/internal/detect"
)

// touch creates an empty file at path, creating intermediate directories.
func touch(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
}

func sortedStrings(langs []detect.Lang) []string {
	ss := make([]string, len(langs))
	for i, l := range langs {
		ss[i] = l.String()
	}
	sort.Strings(ss)
	return ss
}

// --- Detect ---

func TestDetect_Go(t *testing.T) {
	dir := t.TempDir()
	touch(t, filepath.Join(dir, "go.mod"))

	langs, err := detect.Detect(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := sortedStrings(langs)
	if len(got) != 1 || got[0] != "go" {
		t.Fatalf("got %v, want [go]", got)
	}
}

func TestDetect_Rust(t *testing.T) {
	dir := t.TempDir()
	touch(t, filepath.Join(dir, "Cargo.toml"))

	langs, err := detect.Detect(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := sortedStrings(langs)
	if len(got) != 1 || got[0] != "rust" {
		t.Fatalf("got %v, want [rust]", got)
	}
}

func TestDetect_Python_pyproject(t *testing.T) {
	dir := t.TempDir()
	touch(t, filepath.Join(dir, "pyproject.toml"))

	langs, err := detect.Detect(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := sortedStrings(langs)
	if len(got) != 1 || got[0] != "python" {
		t.Fatalf("got %v, want [python]", got)
	}
}

func TestDetect_Python_setup_py(t *testing.T) {
	dir := t.TempDir()
	touch(t, filepath.Join(dir, "setup.py"))

	langs, err := detect.Detect(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := sortedStrings(langs)
	if len(got) != 1 || got[0] != "python" {
		t.Fatalf("got %v, want [python]", got)
	}
}

func TestDetect_Python_setup_cfg(t *testing.T) {
	dir := t.TempDir()
	touch(t, filepath.Join(dir, "setup.cfg"))

	langs, err := detect.Detect(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := sortedStrings(langs)
	if len(got) != 1 || got[0] != "python" {
		t.Fatalf("got %v, want [python]", got)
	}
}

func TestDetect_Python_requirements(t *testing.T) {
	dir := t.TempDir()
	touch(t, filepath.Join(dir, "requirements.txt"))

	langs, err := detect.Detect(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := sortedStrings(langs)
	if len(got) != 1 || got[0] != "python" {
		t.Fatalf("got %v, want [python]", got)
	}
}

func TestDetect_GoAndPython(t *testing.T) {
	dir := t.TempDir()
	touch(t, filepath.Join(dir, "go.mod"))
	touch(t, filepath.Join(dir, "requirements.txt"))

	langs, err := detect.Detect(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := sortedStrings(langs)
	want := []string{"go", "python"}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestDetect_AllThree(t *testing.T) {
	dir := t.TempDir()
	touch(t, filepath.Join(dir, "go.mod"))
	touch(t, filepath.Join(dir, "Cargo.toml"))
	touch(t, filepath.Join(dir, "pyproject.toml"))

	langs, err := detect.Detect(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := sortedStrings(langs)
	want := []string{"go", "python", "rust"}
	if len(got) != 3 || got[0] != want[0] || got[1] != want[1] || got[2] != want[2] {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestDetect_Empty(t *testing.T) {
	dir := t.TempDir()

	langs, err := detect.Detect(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(langs) != 0 {
		t.Fatalf("got %v, want []", langs)
	}
}

func TestDetect_NoDuplicatePython(t *testing.T) {
	// Multiple Python markers should only produce one Python entry.
	dir := t.TempDir()
	touch(t, filepath.Join(dir, "pyproject.toml"))
	touch(t, filepath.Join(dir, "requirements.txt"))
	touch(t, filepath.Join(dir, "setup.py"))

	langs, err := detect.Detect(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := sortedStrings(langs)
	if len(got) != 1 || got[0] != "python" {
		t.Fatalf("got %v, want exactly [python]", got)
	}
}

// --- String ---

func TestLang_String(t *testing.T) {
	cases := []struct {
		lang detect.Lang
		want string
	}{
		{detect.Go, "go"},
		{detect.Python, "python"},
		{detect.Rust, "rust"},
	}
	for _, tc := range cases {
		if got := tc.lang.String(); got != tc.want {
			t.Errorf("Lang(%d).String() = %q, want %q", tc.lang, got, tc.want)
		}
	}
}

// --- ParseLang ---

func TestParseLang_RoundTrip(t *testing.T) {
	for _, name := range []string{"go", "python", "rust"} {
		l, ok := detect.ParseLang(name)
		if !ok {
			t.Fatalf("ParseLang(%q) returned ok=false", name)
		}
		if got := l.String(); got != name {
			t.Errorf("round-trip: ParseLang(%q).String() = %q", name, got)
		}
	}
}

func TestParseLang_Unknown(t *testing.T) {
	_, ok := detect.ParseLang("fortran")
	if ok {
		t.Fatal("expected ok=false for unknown language")
	}
}

func TestParseLang_CaseInsensitive(t *testing.T) {
	// ParseLang should accept "Go", "PYTHON", "Rust" etc.
	for _, tc := range []struct{ input, want string }{
		{"Go", "go"},
		{"PYTHON", "python"},
		{"Rust", "rust"},
	} {
		l, ok := detect.ParseLang(tc.input)
		if !ok {
			t.Fatalf("ParseLang(%q) returned ok=false", tc.input)
		}
		if l.String() != tc.want {
			t.Errorf("ParseLang(%q).String() = %q, want %q", tc.input, l.String(), tc.want)
		}
	}
}

// --- Ext ---

func TestLang_Ext(t *testing.T) {
	cases := []struct {
		lang detect.Lang
		want string
	}{
		{detect.Go, ".go"},
		{detect.Python, ".py"},
		{detect.Rust, ".rs"},
	}
	for _, tc := range cases {
		if got := tc.lang.Ext(); got != tc.want {
			t.Errorf("%s.Ext() = %q, want %q", tc.lang, got, tc.want)
		}
	}
}

// --- Spec ---

func TestLang_Spec(t *testing.T) {
	// Verify that Spec() returns a non-zero LangSpec (Lang grammar is non-nil).
	for _, l := range []detect.Lang{detect.Go, detect.Python, detect.Rust} {
		spec := l.Spec()
		if spec.Lang == nil {
			t.Errorf("%s.Spec().Lang is nil", l)
		}
		if len(spec.FuncNodes) == 0 {
			t.Errorf("%s.Spec().FuncNodes is empty", l)
		}
	}
}
