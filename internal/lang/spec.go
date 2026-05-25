// Package lang holds per-language tree-sitter specifications and the analyzer
// that enumerates functions with their line ranges and cyclomatic complexity.
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

// LangSpec describes how to analyze one language's tree-sitter parse tree.
type LangSpec struct {
	// Lang is the tree-sitter grammar.
	Lang *sitter.Language
	// FuncNodes is the set of node types that introduce a new function scope.
	FuncNodes map[string]bool
	// DecisionNodes is the set of node types that each add 1 to cyclomatic
	// complexity by virtue of their type alone.
	DecisionNodes map[string]bool
	// BinaryOperators is the set of binary-expression operator tokens (e.g.
	// "&&", "||") that each add 1 to complexity. Detected via the node's
	// "operator" field. Empty means binary operators are not counted.
	BinaryOperators map[string]bool
	// NameOf extracts the display name for a function node. It is given the
	// function node and the full source. If it returns "", the analyzer falls
	// back to a positional label (e.g. func@<line>).
	NameOf func(node *sitter.Node, src []byte) string
}

func set(items ...string) map[string]bool {
	m := make(map[string]bool, len(items))
	for _, it := range items {
		m[it] = true
	}
	return m
}

// Go returns the LangSpec for the Go language.
//
// Node-type names below were verified empirically against the golang grammar
// (smacker/go-tree-sitter @ dd81d9e). Cyclomatic complexity is 1 + the count of
// decision points, where a decision point is:
//   - if_statement              (each if)
//   - for_statement             (each for / range loop)
//   - expression_case           (each non-default case of a value switch)
//   - type_case                 (each non-default case of a type switch)
//   - communication_case        (each non-default case of a select)
//   - a binary_expression whose operator is "&&" or "||"
//
// default_case is intentionally NOT counted: a switch with N cases plus a
// default contributes N, matching the convention that the fall-through default
// adds no independent branch.
func Go() LangSpec {
	return LangSpec{
		Lang:      golang.GetLanguage(),
		FuncNodes: set("function_declaration", "method_declaration", "func_literal"),
		DecisionNodes: set(
			"if_statement",
			"for_statement",
			"expression_case",
			"type_case",
			"communication_case",
		),
		BinaryOperators: set("&&", "||"),
		NameOf:          goName,
	}
}

// goName extracts a name for a Go function node. For methods it prefixes the
// receiver type (e.g. "T.Do"); for plain functions it returns the identifier;
// for func literals it returns "" so the analyzer applies a positional label.
func goName(node *sitter.Node, src []byte) string {
	name := node.ChildByFieldName("name")
	if name == nil {
		return "" // func_literal: anonymous
	}
	fn := name.Content(src)
	if recv := node.ChildByFieldName("receiver"); recv != nil {
		if rt := firstTypeIdentifier(recv, src); rt != "" {
			return rt + "." + fn
		}
	}
	return fn
}

// firstTypeIdentifier returns the content of the first type_identifier node in
// the subtree rooted at n, used to pull the receiver type out of a parameter
// list like "(r *T)" or "(T)".
func firstTypeIdentifier(n *sitter.Node, src []byte) string {
	if n.Type() == "type_identifier" {
		return n.Content(src)
	}
	for i := 0; i < int(n.NamedChildCount()); i++ {
		if r := firstTypeIdentifier(n.NamedChild(i), src); r != "" {
			return r
		}
	}
	return ""
}

// Python returns the LangSpec for the Python language.
//
// Node-type names were verified empirically against the python grammar
// (smacker/go-tree-sitter @ dd81d9e). Cyclomatic complexity is 1 + the count of
// decision points, where a decision point is:
//   - if_statement              (each if)
//   - elif_clause               (each elif branch)
//   - for_statement             (each for loop)
//   - while_statement           (each while loop)
//   - except_clause             (each except handler)
//   - boolean_operator          (each 'and' / 'or'; parser produces one
//                                boolean_operator node per operator occurrence)
//   - conditional_expression    (ternary: x if c else y)
//   - case_clause               (each arm of a match statement, including '_')
//
// Notes:
//   - else_clause / else branches are NOT counted (no extra branch).
//   - if_clause inside comprehensions ([x for x in xs if x>0]) is NOT counted;
//     those belong to the comprehension grammar and are not if_statement nodes.
//   - The wildcard case_clause (`case _:`) counts like any other arm.
func Python() LangSpec {
	return LangSpec{
		Lang:      python.GetLanguage(),
		FuncNodes: set("function_definition"),
		DecisionNodes: set(
			"if_statement",
			"elif_clause",
			"for_statement",
			"while_statement",
			"except_clause",
			"boolean_operator",
			"conditional_expression",
			"case_clause",
		),
		// Python uses boolean_operator node type (not binary_expression + operator
		// field), so BinaryOperators is empty; detection is via DecisionNodes.
		BinaryOperators: nil,
		NameOf:          pythonName,
	}
}

// pythonName extracts the function name from a function_definition node.
// Python functions always have a "name" field (an identifier).
func pythonName(node *sitter.Node, src []byte) string {
	name := node.ChildByFieldName("name")
	if name == nil {
		return ""
	}
	return name.Content(src)
}

// Rust returns the LangSpec for the Rust language.
//
// Node-type names were verified empirically against the rust grammar
// (smacker/go-tree-sitter @ dd81d9e). Cyclomatic complexity is 1 + the count of
// decision points, where a decision point is:
//   - if_expression             (each if / if let)
//   - while_expression          (each while / while let)
//   - for_expression            (each for..in loop)
//   - match_arm                 (each arm of a match, including wildcard '_')
//   - try_expression            (each ? operator usage)
//   - a binary_expression whose operator is "&&" or "||"
//
// Notes:
//   - else_clause is NOT counted separately (the else branch of an if is covered
//     by the if_expression's own +1).
//   - loop_expression is NOT counted: a bare "loop {}" is an unconditional
//     infinite loop with no branching condition, so by McCabe semantics it
//     introduces no independent decision. (while/for keep their +1 because they
//     branch on a condition.)
//   - Wildcard match_arm (`_ => ...`) counts like any other arm.
//   - closure_expression is a function scope: its decisions are counted separately
//     and not attributed to the enclosing function_item.
//   - && / || detection mirrors the Go spec: binary_expression + operator field.
func Rust() LangSpec {
	return LangSpec{
		Lang:      rust.GetLanguage(),
		FuncNodes: set("function_item", "closure_expression"),
		DecisionNodes: set(
			"if_expression",
			"while_expression",
			"for_expression",
			"match_arm",
			"try_expression",
		),
		BinaryOperators: set("&&", "||"),
		NameOf:          rustName,
	}
}

// rustName extracts a name for a Rust function node. For function_item nodes
// it returns the "name" field (an identifier). For closure_expression nodes
// it returns "" so the analyzer applies a positional label (func@<line>).
func rustName(node *sitter.Node, src []byte) string {
	if node.Type() == "closure_expression" {
		return "" // anonymous: fall back to func@<line>
	}
	name := node.ChildByFieldName("name")
	if name == nil {
		return ""
	}
	return name.Content(src)
}
