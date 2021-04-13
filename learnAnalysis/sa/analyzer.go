package sa

import (
	"errors"
	"fmt"

	// "container/list"

	"go/ast"
	"go/token"

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

var errNestedRLock = errors.New("found recursive read lock call")

func run(pass *analysis.Pass) (interface{}, error) {
	inspect, ok := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)
	if !ok {
		return nil, errors.New("analyzer is not type *inspector.Inspector")
	}

	// filters out other pieces of source code except for function/method calls
	nodeFilter := []ast.Node{
		(*ast.CallExpr)(nil),
		(*ast.DeferStmt)(nil),
		(*ast.FuncDecl)(nil),
	}

	foundRLock := 0
	deferredRLock := false
	endPos := token.NoPos
	inspect.Preorder(nodeFilter, func(node ast.Node) {
		if node.Pos() > endPos && deferredRLock {
			deferredRLock = false
			foundRLock--
		}
		switch stmt := node.(type) {
		case *ast.CallExpr:
			name := getName(stmt.Fun)
			if name == "RLock" { // if the method found is an RLock method
				if foundRLock > 0 { // if we have already seen an RLock method without seeing a corresponding RUnlock
					// fmt.Println("Found RLock")
					pass.Reportf(
						node.Pos(),
						fmt.Sprintf(
							"%v",
							errNestedRLock,
						),
					)
				}
				foundRLock++
			} else if name == "RUnlock" && !deferredRLock {
				foundRLock--
			} else if name != "RUnlock" && foundRLock > 0 {
				// fmt.Printf("Found '%v' at %v\n", name, pass.Fset.Position(node.Pos()))
				if stack := hasNestedRLock(name, inspect, pass.Fset); stack != "" {
					pass.Reportf(
						node.Pos(),
						fmt.Sprintf(
							"%v",
							errNestedRLock,
							// stack,
						),
					)
				}
			}
		case *ast.DeferStmt:
			name := getName(stmt.Call.Fun)
			if name == "RUnlock" {
				deferredRLock = true
			}
		case *ast.FuncDecl:
			endPos = stmt.Body.Rbrace
		}
	})

	return nil, nil
}

func getName(fun ast.Expr) string {
	switch expr := fun.(type) {
	case *ast.SelectorExpr:
		return expr.Sel.Name
	case *ast.Ident:
		return expr.Name
	}
	return ""
}

func hasNestedRLock(funcName string, inspect *inspector.Inspector, f *token.FileSet) (retStack string) {
	node := findCallDeclarationNode(funcName, inspect)
	if node == nil {
		return ""
	}
	ast.Inspect(node, func(iNode ast.Node) bool {
		switch stmt := iNode.(type) {
		case *ast.CallExpr:
			name := getName(stmt.Fun)
			addition := fmt.Sprintf("\tat %v\n", f.Position(iNode.Pos()))
			if name == "RLock" { // if the method found is an RLock method
				retStack += addition
			} else if name != "RUnlock" && name != funcName { // name should not equal the previousName to prevent infinite recursive loop
				stack := hasNestedRLock(name, inspect, f)
				if stack != "" {
					retStack += addition + stack
				}
			}
		}
		return true
	})
	return retStack
}

func findCallDeclarationNode(targetName string, inspect *inspector.Inspector) ast.Node {
	var retNode ast.Node = nil
	nodeFilter := []ast.Node{
		(*ast.FuncDecl)(nil),
	}
	inspect.Preorder(nodeFilter, func(node ast.Node) {
		funcDec, _ := node.(*ast.FuncDecl)
		name := funcDec.Name.Name
		if targetName == name {
			retNode = node
		}
	})
	return retNode
}

/*
    The process:
go through ast
if we find an RLock, call hasNestedRLock with the node as the parameter

	hasNestedRLock:
return false if node is nil
look through tree until we find
	an RLock(): return true
	a method call: return hasNestedRLock(findMethodDeclarationNode(methodName))
	an RUnlock(): return false

	findCallDeclarationNode(name):
inspect ast until we find a call declaration of name <name>
*/
