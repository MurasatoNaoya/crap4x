package lang

import (
	"context"
	"fmt"

	sitter "github.com/smacker/go-tree-sitter"
)

// Analyze parses src and returns one Function per node whose type is in
// spec.FuncNodes. Complexity is 1 plus the number of decision points strictly
// inside that function but NOT inside any nested function, so closures and
// nested funcs are counted as their own Function and never double-counted into
// the enclosing one. Start/End are 1-based line numbers.
func Analyze(src []byte, file string, spec LangSpec) []Function {
	parser := sitter.NewParser()
	parser.SetLanguage(spec.Lang)
	tree, err := parser.ParseCtx(context.Background(), nil, src)
	if err != nil || tree == nil {
		return nil
	}
	defer tree.Close()

	var out []Function
	var walk func(n *sitter.Node)
	walk = func(n *sitter.Node) {
		if spec.FuncNodes[n.Type()] {
			out = append(out, buildFunction(n, src, file, spec))
		}
		for i := 0; i < int(n.NamedChildCount()); i++ {
			walk(n.NamedChild(i))
		}
	}
	walk(tree.RootNode())
	return out
}

// buildFunction constructs the Function for a function node, counting decision
// points within its own scope only.
func buildFunction(node *sitter.Node, src []byte, file string, spec LangSpec) Function {
	start := int(node.StartPoint().Row) + 1
	end := int(node.EndPoint().Row) + 1

	name := ""
	if spec.NameOf != nil {
		name = spec.NameOf(node, src)
	}
	if name == "" {
		name = fmt.Sprintf("func@%d", start)
	}

	cc := 1 + countDecisions(node, src, spec, true)
	return Function{
		Name:       name,
		File:       file,
		Start:      start,
		End:        end,
		Complexity: cc,
	}
}

// countDecisions counts decision points in the subtree rooted at n, descending
// into children but stopping at the boundary of any nested function (so those
// belong to their own Function). When isRoot is true, n is itself a function
// node and we always descend into it.
func countDecisions(n *sitter.Node, src []byte, spec LangSpec, isRoot bool) int {
	if !isRoot && spec.FuncNodes[n.Type()] {
		return 0 // nested function: its decisions belong to it, not us
	}
	count := 0
	if !isRoot {
		if spec.DecisionNodes[n.Type()] {
			count++
		}
		if len(spec.BinaryOperators) > 0 && n.Type() == "binary_expression" {
			if op := n.ChildByFieldName("operator"); op != nil {
				if spec.BinaryOperators[op.Content(src)] {
					count++
				}
			}
		}
	}
	for i := 0; i < int(n.NamedChildCount()); i++ {
		count += countDecisions(n.NamedChild(i), src, spec, false)
	}
	return count
}
