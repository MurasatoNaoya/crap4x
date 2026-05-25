package lang

import (
	"sort"
	"testing"
)

func analyzeRust(t *testing.T, src string) []Function {
	t.Helper()
	fns := Analyze([]byte(src), "test.rs", Rust())
	sort.Slice(fns, func(i, j int) bool { return fns[i].Start < fns[j].Start })
	return fns
}

// TestRustNoBranches: base CC = 1.
func TestRustNoBranches(t *testing.T) {
	src := `fn simple() -> i32 {
    1
}
`
	fns := analyzeRust(t, src)
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
	if f.Start != 1 || f.End != 3 {
		t.Errorf("range: want 1-3, got %d-%d", f.Start, f.End)
	}
	if f.File != "test.rs" {
		t.Errorf("file: want test.rs, got %q", f.File)
	}
}

// TestRustIfAndFor: if (+1) + for (+1) + base = 3.
func TestRustIfAndFor(t *testing.T) {
	src := `fn loop_fn(a: bool, xs: &[i32]) {
    if a {
        let _ = 1;
    }
    for x in xs {
        let _ = x;
    }
}
`
	f := byName(analyzeRust(t, src))["loop_fn"]
	if f.Complexity != 3 {
		t.Errorf("complexity: want 3, got %d", f.Complexity)
	}
}

// TestRustWhileAndLoop: while (+1) + base = 2. A bare "loop {}" is an
// unconditional infinite loop with no branching condition, so it does NOT add a
// decision point (loop_expression is excluded from DecisionNodes).
func TestRustWhileAndLoop(t *testing.T) {
	src := `fn spins(mut n: i32) {
    while n > 0 {
        n -= 1;
    }
    loop {
        break;
    }
}
`
	f := byName(analyzeRust(t, src))["spins"]
	if f.Complexity != 2 {
		t.Errorf("complexity: want 2 (base + while; bare loop excluded), got %d", f.Complexity)
	}
}

// TestRustMatchArms: match with 3 match_arm (2 value arms + 1 wildcard) + base = 4.
// The wildcard arm _ is still a match_arm node and counts.
func TestRustMatchArms(t *testing.T) {
	src := `fn categorise(x: i32) -> &'static str {
    match x {
        1 => "one",
        2 => "two",
        _ => "other",
    }
}
`
	f := byName(analyzeRust(t, src))["categorise"]
	// base 1 + 3 match_arm = 4
	if f.Complexity != 4 {
		t.Errorf("complexity: want 4 (base + 3 match_arms incl. wildcard), got %d", f.Complexity)
	}
}

// TestRustBooleanOperators: binary_expression with && (+1) and || (+1) + base = 3.
// "a && b || c" parses as binary_expression(binary_expression(a,&&,b), ||, c):
// two binary_expression nodes each with a counted operator.
func TestRustBooleanOperators(t *testing.T) {
	src := `fn boolexpr(a: bool, b: bool, c: bool) -> bool {
    a && b || c
}
`
	f := byName(analyzeRust(t, src))["boolexpr"]
	if f.Complexity != 3 {
		t.Errorf("complexity: want 3 (base + && + ||), got %d", f.Complexity)
	}
}

// TestRustTryExpression: ? operator becomes a try_expression (+1) + base = 2.
func TestRustTryExpression(t *testing.T) {
	src := `fn parse_num(s: &str) -> Result<i32, std::num::ParseIntError> {
    let n = s.parse::<i32>()?;
    Ok(n)
}
`
	f := byName(analyzeRust(t, src))["parse_num"]
	if f.Complexity != 2 {
		t.Errorf("complexity: want 2 (base + try ?), got %d", f.Complexity)
	}
}

// TestRustClosureCountedSeparately: outer has its own if (+1) = 2.
// Closure has its own if (+1) = 2 and is counted separately.
// Closure's decisions must NOT be double-counted into outer.
func TestRustClosureCountedSeparately(t *testing.T) {
	src := `fn outer(a: bool) {
    if a {
        let _ = 1;
    }
    let f = |x: i32| {
        if x > 0 {
            x
        } else {
            0
        }
    };
    let _ = f(1);
}
`
	fns := analyzeRust(t, src)
	if len(fns) != 2 {
		t.Fatalf("want 2 functions (outer + closure), got %d: %+v", len(fns), fns)
	}
	m := byName(fns)
	outer, ok := m["outer"]
	if !ok {
		t.Fatalf("missing outer; got %+v", fns)
	}
	if outer.Complexity != 2 {
		t.Errorf("outer complexity: want 2 (its if only), got %d", outer.Complexity)
	}
	// closure is labelled func@<line>; it starts at line 5
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
	if closure.Name != "func@5" {
		t.Errorf("closure name: want func@5, got %q", closure.Name)
	}
}

// TestRustLineRange: verify Start/End for a function not at line 1.
func TestRustLineRange(t *testing.T) {
	src := `fn first() {
    let _ = 1;
}

fn second() {
    let _ = 2;
}
`
	fns := analyzeRust(t, src)
	if len(fns) != 2 {
		t.Fatalf("want 2 functions, got %d", len(fns))
	}
	second := fns[1]
	if second.Name != "second" {
		t.Errorf("name: want second, got %q", second.Name)
	}
	if second.Start != 5 {
		t.Errorf("Start: want 5, got %d", second.Start)
	}
}
