package coverage_test

import (
	"strings"
	"testing"

	"github.com/MurasatoNaoya/crap4x/internal/coverage"
)

func TestParse_TwoFiles(t *testing.T) {
	input := `TN:
SF:pkg/foo/foo.go
DA:5,1
DA:6,0
DA:7,3
end_of_record
SF:pkg/bar/bar.go
DA:10,0
DA:11,2
end_of_record
`
	got, err := coverage.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 files, got %d", len(got))
	}

	foo := got["pkg/foo/foo.go"]
	if foo == nil {
		t.Fatal("missing pkg/foo/foo.go")
	}
	if foo[5] != 1 {
		t.Errorf("foo line 5: want 1, got %d", foo[5])
	}
	if foo[6] != 0 {
		t.Errorf("foo line 6: want 0 (uncovered), got %d", foo[6])
	}
	if foo[7] != 3 {
		t.Errorf("foo line 7: want 3, got %d", foo[7])
	}

	bar := got["pkg/bar/bar.go"]
	if bar == nil {
		t.Fatal("missing pkg/bar/bar.go")
	}
	if bar[10] != 0 {
		t.Errorf("bar line 10: want 0 (uncovered), got %d", bar[10])
	}
	if bar[11] != 2 {
		t.Errorf("bar line 11: want 2, got %d", bar[11])
	}
}

func TestParse_CoveredVsUncoveredDistinct(t *testing.T) {
	input := `SF:main.go
DA:1,0
DA:2,0
DA:3,5
DA:4,0
DA:5,10
end_of_record
`
	got, err := coverage.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m := got["main.go"]
	if m == nil {
		t.Fatal("missing main.go")
	}
	// All five lines must be present, hits==0 and hits>0 tracked distinctly.
	for _, line := range []int{1, 2, 3, 4, 5} {
		if _, ok := m[line]; !ok {
			t.Errorf("line %d not present in map", line)
		}
	}
	if m[1] != 0 || m[2] != 0 || m[4] != 0 {
		t.Errorf("uncovered lines should have hit count 0")
	}
	if m[3] != 5 || m[5] != 10 {
		t.Errorf("covered lines should preserve hit counts")
	}
}

func TestParse_UnknownRecordTypesIgnored(t *testing.T) {
	input := `TN:mytest
SF:lib.go
FN:3,myFunc
FNDA:1,myFunc
FNF:1
FNH:1
BRDA:3,0,0,1
BRF:1
BRH:1
DA:3,7
LF:1
LH:1
end_of_record
`
	got, err := coverage.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m := got["lib.go"]
	if m == nil {
		t.Fatal("missing lib.go")
	}
	if len(m) != 1 {
		t.Errorf("expected 1 DA line, got %d entries", len(m))
	}
	if m[3] != 7 {
		t.Errorf("line 3: want 7, got %d", m[3])
	}
}

func TestParse_MissingFinalEndOfRecord(t *testing.T) {
	// File ends without end_of_record; data should still be returned.
	input := `SF:incomplete.go
DA:1,4
DA:2,0
`
	got, err := coverage.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m := got["incomplete.go"]
	if m == nil {
		t.Fatal("missing incomplete.go")
	}
	if m[1] != 4 {
		t.Errorf("line 1: want 4, got %d", m[1])
	}
	if m[2] != 0 {
		t.Errorf("line 2: want 0, got %d", m[2])
	}
}

func TestParse_EmptyInput(t *testing.T) {
	got, err := coverage.Parse(strings.NewReader(""))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty map, got %d entries", len(got))
	}
}

func TestParse_MalformedDALinesSkipped(t *testing.T) {
	input := `SF:malformed.go
DA:notanumber,1
DA:3
DA:,5
DA:4,2
end_of_record
`
	got, err := coverage.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m := got["malformed.go"]
	if m == nil {
		t.Fatal("missing malformed.go")
	}
	// Only line 4 is valid; malformed ones are skipped, not errored.
	if len(m) != 1 {
		t.Errorf("expected 1 valid DA entry, got %d: %v", len(m), m)
	}
	if m[4] != 2 {
		t.Errorf("line 4: want 2, got %d", m[4])
	}
}
