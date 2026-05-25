package lang

import (
	"sort"
	"testing"
)

// byName returns functions indexed by name for easy assertion.
func byName(fns []Function) map[string]Function {
	m := make(map[string]Function, len(fns))
	for _, f := range fns {
		m[f.Name] = f
	}
	return m
}

func analyzeGo(t *testing.T, src string) []Function {
	t.Helper()
	fns := Analyze([]byte(src), "test.go", Go())
	sort.Slice(fns, func(i, j int) bool { return fns[i].Start < fns[j].Start })
	return fns
}

func TestNoBranches(t *testing.T) {
	src := `package p

func plain() int {
	return 1
}
`
	fns := analyzeGo(t, src)
	if len(fns) != 1 {
		t.Fatalf("want 1 function, got %d: %+v", len(fns), fns)
	}
	f := fns[0]
	if f.Name != "plain" {
		t.Errorf("name: want plain, got %q", f.Name)
	}
	if f.Complexity != 1 {
		t.Errorf("complexity: want 1, got %d", f.Complexity)
	}
	if f.Start != 3 || f.End != 5 {
		t.Errorf("range: want 3-5, got %d-%d", f.Start, f.End)
	}
	if f.File != "test.go" {
		t.Errorf("file: want test.go, got %q", f.File)
	}
}

func TestIfAndFor(t *testing.T) {
	// if (+1) + for (+1) + base (1) = 3
	src := `package p

func loop(a bool) {
	if a {
	}
	for i := 0; i < 10; i++ {
	}
}
`
	f := byName(analyzeGo(t, src))["loop"]
	if f.Complexity != 3 {
		t.Errorf("complexity: want 3, got %d", f.Complexity)
	}
}

func TestSwitchCases(t *testing.T) {
	// base 1 + 3 expression_case; default_case does NOT count.
	src := `package p

func sw(a int) int {
	switch a {
	case 1:
		return 1
	case 2:
		return 2
	case 3:
		return 3
	default:
		return 0
	}
}
`
	f := byName(analyzeGo(t, src))["sw"]
	if f.Complexity != 4 {
		t.Errorf("complexity: want 4 (1 + 3 cases, default excluded), got %d", f.Complexity)
	}
}

func TestTypeSwitchCases(t *testing.T) {
	// base 1 + 2 type_case; default excluded.
	src := `package p

func ts(x interface{}) {
	switch x.(type) {
	case int:
	case string:
	default:
	}
}
`
	f := byName(analyzeGo(t, src))["ts"]
	if f.Complexity != 3 {
		t.Errorf("complexity: want 3, got %d", f.Complexity)
	}
}

func TestSelectCases(t *testing.T) {
	// base 1 + 2 communication_case; default excluded.
	src := `package p

func sel(a, b chan int) {
	select {
	case <-a:
	case <-b:
	default:
	}
}
`
	f := byName(analyzeGo(t, src))["sel"]
	if f.Complexity != 3 {
		t.Errorf("complexity: want 3, got %d", f.Complexity)
	}
}

func TestBooleanOperators(t *testing.T) {
	// base 1 + && (+1) + || (+1) = 3
	src := `package p

func boolexpr(a, b, c bool) bool {
	return a && b || c
}
`
	f := byName(analyzeGo(t, src))["boolexpr"]
	if f.Complexity != 3 {
		t.Errorf("complexity: want 3, got %d", f.Complexity)
	}
}

func TestClosureCountedSeparately(t *testing.T) {
	// Outer has an if (+1) -> 2. Closure has its own if (+1) -> 2.
	// The closure's decisions must NOT be double-counted into the outer.
	src := `package p

func outer(a bool) {
	if a {
	}
	f := func(b bool) {
		if b {
		}
	}
	_ = f
}
`
	fns := analyzeGo(t, src)
	if len(fns) != 2 {
		t.Fatalf("want 2 functions (outer + closure), got %d: %+v", len(fns), fns)
	}
	m := byName(fns)
	outer, ok := m["outer"]
	if !ok {
		t.Fatalf("missing outer; got %+v", fns)
	}
	if outer.Complexity != 2 {
		t.Errorf("outer complexity: want 2 (its if only, closure not double-counted), got %d", outer.Complexity)
	}
	// The closure is labelled func@<line>; it starts on line 6.
	var closure *Function
	for i := range fns {
		if fns[i].Name != "outer" {
			closure = &fns[i]
		}
	}
	if closure == nil {
		t.Fatalf("no closure function found")
	}
	if closure.Complexity != 2 {
		t.Errorf("closure complexity: want 2, got %d", closure.Complexity)
	}
	if closure.Name != "func@6" {
		t.Errorf("closure name: want func@6, got %q", closure.Name)
	}
}

func TestMethodName(t *testing.T) {
	src := `package p

type T struct{}

func (t T) Do(a bool) {
	if a {
	}
}

func (t *T) DoPtr() {
}
`
	m := byName(analyzeGo(t, src))
	if _, ok := m["T.Do"]; !ok {
		t.Errorf("want method named T.Do, got %v", keys(m))
	}
	if _, ok := m["T.DoPtr"]; !ok {
		t.Errorf("want method named T.DoPtr, got %v", keys(m))
	}
	if m["T.Do"].Complexity != 2 {
		t.Errorf("T.Do complexity: want 2, got %d", m["T.Do"].Complexity)
	}
}

func keys(m map[string]Function) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	return ks
}
