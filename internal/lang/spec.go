// Package lang holds language specifications for tree-sitter parsing.
// LangSpec, Function, and the grammar registry are fully implemented in later tasks.
package lang

import (
	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/golang"
	"github.com/smacker/go-tree-sitter/python"
	"github.com/smacker/go-tree-sitter/rust"
)

// Function represents a single callable unit found in source code.
type Function struct {
	Name       string
	File       string
	Start      int // 1-based line
	End        int // 1-based line
	Complexity int // cyclomatic complexity
}

// grammars wires in the three tree-sitter grammars so go mod tidy retains them.
// Each entry is a *sitter.Language; the full LangSpec registry is built in Task 3.
var grammars = []*sitter.Language{
	golang.GetLanguage(),
	python.GetLanguage(),
	rust.GetLanguage(),
}
