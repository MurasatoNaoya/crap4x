package lang

import (
	"sort"
	"testing"
)

func analyzePy(t *testing.T, src string) []Function {
	t.Helper()
	fns := Analyze([]byte(src), "test.py", Python())
	sort.Slice(fns, func(i, j int) bool { return fns[i].Start < fns[j].Start })
	return fns
}

// TestPyNoBranches: base CC = 1.
func TestPyNoBranches(t *testing.T) {
	src := `def simple():
    return 1
`
	fns := analyzePy(t, src)
	if len(fns) != 1 {
		t.Fatalf("want 1 function, got %d: %+v", len(fns), fns)
	}
	f := fns[0]
	if f.Name != "simple" {
		t.Errorf("name: want simple, got %q", f.Name)
	}
	if f.Complexity != 1 {
		t.Errorf("complexity: want 1, got %d", f.Complexity)
	}
	if f.Start != 1 || f.End != 2 {
		t.Errorf("range: want 1-2, got %d-%d", f.Start, f.End)
	}
	if f.File != "test.py" {
		t.Errorf("file: want test.py, got %q", f.File)
	}
}

// TestPyIfAndFor: if (+1) + for (+1) + base = 3.
func TestPyIfAndFor(t *testing.T) {
	src := `def loop(a, xs):
    if a:
        pass
    for x in xs:
        pass
`
	f := byName(analyzePy(t, src))["loop"]
	if f.Complexity != 3 {
		t.Errorf("complexity: want 3, got %d", f.Complexity)
	}
}

// TestPyElifAndWhile: if (+1) + elif (+1) + while (+1) + base = 4.
func TestPyElifAndWhile(t *testing.T) {
	src := `def mixed(a, b, n):
    if a:
        pass
    elif b:
        pass
    while n > 0:
        n -= 1
`
	f := byName(analyzePy(t, src))["mixed"]
	if f.Complexity != 4 {
		t.Errorf("complexity: want 4 (if+elif+while+base), got %d", f.Complexity)
	}
}

// TestPyExcept: try with 2 except_clause (+2) + base = 3.
func TestPyExcept(t *testing.T) {
	src := `def risky():
    try:
        pass
    except ValueError:
        pass
    except TypeError:
        pass
`
	f := byName(analyzePy(t, src))["risky"]
	if f.Complexity != 3 {
		t.Errorf("complexity: want 3 (base + 2 except), got %d", f.Complexity)
	}
}

// TestPyMatchCases: match with 3 case_clause (including wildcard _) + base = 4.
// The wildcard arm is still a case_clause and counts.
func TestPyMatchCases(t *testing.T) {
	src := `def categorise(x):
    match x:
        case 1:
            return "one"
        case 2:
            return "two"
        case _:
            return "other"
`
	f := byName(analyzePy(t, src))["categorise"]
	// base 1 + 3 case_clause = 4
	if f.Complexity != 4 {
		t.Errorf("complexity: want 4 (base + 3 case_clauses incl. wildcard), got %d", f.Complexity)
	}
}

// TestPyBooleanOperators: boolean_operator nodes for 'and' and 'or'.
// "a and b or c" parses as boolean_operator(boolean_operator(a,and,b), or, c):
// two boolean_operator nodes, so +2; base = 1 gives total 3.
func TestPyBooleanOperators(t *testing.T) {
	src := `def boolexpr(a, b, c):
    return a and b or c
`
	f := byName(analyzePy(t, src))["boolexpr"]
	if f.Complexity != 3 {
		t.Errorf("complexity: want 3 (base + and + or), got %d", f.Complexity)
	}
}

// TestPyTernary: conditional_expression (+1) + base = 2.
func TestPyTernary(t *testing.T) {
	src := `def choose(a):
    return 1 if a else 2
`
	f := byName(analyzePy(t, src))["choose"]
	if f.Complexity != 2 {
		t.Errorf("complexity: want 2 (base + ternary), got %d", f.Complexity)
	}
}

// TestPyNestedFunctionSeparate: outer has its own if (+1) = 2; inner has its own if (+1) = 2.
// Inner's decisions must NOT be double-counted into outer.
func TestPyNestedFunctionSeparate(t *testing.T) {
	src := `def outer():
    def inner():
        if True:
            pass
    if True:
        pass
`
	fns := analyzePy(t, src)
	if len(fns) != 2 {
		t.Fatalf("want 2 functions (outer + inner), got %d: %+v", len(fns), fns)
	}
	m := byName(fns)
	outer, ok := m["outer"]
	if !ok {
		t.Fatalf("missing outer; got %+v", fns)
	}
	if outer.Complexity != 2 {
		t.Errorf("outer complexity: want 2 (its if only), got %d", outer.Complexity)
	}
	inner, ok := m["inner"]
	if !ok {
		t.Fatalf("missing inner; got %+v", fns)
	}
	if inner.Complexity != 2 {
		t.Errorf("inner complexity: want 2, got %d", inner.Complexity)
	}
}

// TestPyLineRange: verify Start/End for a function not at line 1.
func TestPyLineRange(t *testing.T) {
	src := `def first():
    pass

def second():
    pass
`
	fns := analyzePy(t, src)
	if len(fns) != 2 {
		t.Fatalf("want 2 functions, got %d", len(fns))
	}
	// second starts at line 4
	second := fns[1]
	if second.Name != "second" {
		t.Errorf("name: want second, got %q", second.Name)
	}
	if second.Start != 4 {
		t.Errorf("Start: want 4, got %d", second.Start)
	}
}
