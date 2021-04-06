package sa

import (
	"errors"
	"fmt"
	"go/ast"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

// Analyzer runs static analysis.
var Analyzer = &analysis.Analyzer{
	Name:     "experiment",
	Doc:      "Checks to see how AST is structured",
	Requires: []*analysis.Analyzer{inspect.Analyzer},
	Run:      run,
}

func run(pass *analysis.Pass) (interface{}, error) {
	inspect, ok := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)
	if !ok {
		return nil, errors.New("analyzer is not type *inspector.Inspector")
	}

	nodeFilter := []ast.Node{
		(*ast.CallExpr)(nil),
	}

	inspect.Preorder(nodeFilter, func(node ast.Node) {
		switch stmt := node.(type) {
		case *ast.CallExpr:
			// Check if any of disallowed functions have been used.
			ast.Inspect(stmt, func(node2 ast.Node) bool {
				stmt2, ok := node2.(*ast.CallExpr)
				if ok {
					if sel, ok2 := stmt2.Fun.(*ast.SelectorExpr); ok2 {
						fmt.Printf("%v: %v\n", node2.Pos(), sel.Sel.Name)
					}
				}
				return true
			})
		}
	})

	return nil, nil
}
