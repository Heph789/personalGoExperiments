package sa

import (
	"errors"
	"fmt"

	// "container/list"

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

var errNestedRLock = errors.New("found recursive read lock mutex")

func run(pass *analysis.Pass) (interface{}, error) {
	inspect, ok := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)
	if !ok {
		return nil, errors.New("analyzer is not type *inspector.Inspector")
	}

	// filters out other pieces of source code except for function/method calls
	nodeFilter := []ast.Node{
		(*ast.CallExpr)(nil),
	}

	// potentialBadRLocks := list.New()
	foundRLock := make(map[string]bool) // using a mapping ensures that we do not confuse two RLocks from different mutex declarations as the same

	inspect.Preorder(nodeFilter, func(node ast.Node) {
		switch stmt := node.(type) {
		case *ast.CallExpr:
			if fun, ok := stmt.Fun.(*ast.SelectorExpr); ok {
				//fmt.Println("if 1")
				if x, ok := fun.X.(*ast.Ident); ok {
					//fmt.Println("if 2")
					if x.Obj != nil {
						fmt.Println("The ast:")
						ast.Print(pass.Fset, x.Obj.Decl)
					}
					if fun.Sel.Name == "RLock" { // if the method found is an RLock method
						if foundRLock[x.Name] { // if we have already seen an RLock method without seeing a corresponding RUnlock
							fmt.Println("Found RLock")
							pass.Reportf(
								node.Pos(),
								fmt.Sprintf(
									"%v",
									errNestedRLock,
								),
							)
							return
						}
						foundRLock[x.Name] = true
					} else if fun.Sel.Name == "RUnlock" {
						foundRLock[x.Name] = false
					}
				}
			}
		}
	})

	return nil, nil
}

func findRootName(iden *ast.Ident) string {

	return ""
}
