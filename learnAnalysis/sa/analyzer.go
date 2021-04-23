package sa

// IMPORTANT NOTE: DEFERRED FUNCTION IS EXECUTED RIGHT AFTER RETURN STATEMENT.
// NOT AFTER FUNCTION CALL IN RETURN STATEMENT

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

var funcToTest = "saveStateByRoot"
var currentFunc = ""

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
		(*ast.File)(nil),
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
				t := ""
				if fun, ok := stmt.Fun.(*ast.SelectorExpr); ok {
					t = getXType(fun)
				}
				fmt.Printf("METHOD Found '%v' of type '%v' at %v\n", name, t, pass.Fset.Position(node.Pos()))
				ast.Print(pass.Fset, findCallDeclarationNode(getName(stmt.Fun), t, inspect))
				currentFunc = name
				// funcType := getType(stmt.Fun)
				// if stack := hasNestedRLock(name, funcType, inspect, pass.Fset); stack != "" {
				// 	pass.Reportf(
				// 		node.Pos(),
				// 		fmt.Sprintf(
				// 			"%v",
				// 			errNestedRLock,
				// 			// stack,
				// 		),
				// 	)
				// }
			}
		case *ast.DeferStmt:
			name := getName(stmt.Call.Fun)
			if name == "RUnlock" {
				deferredRLock = true
			}
		case *ast.FuncDecl:
			// if stmt.Name.Name == "GetResource" {
			// 	ast.Print(pass.Fset, stmt)
			// }
			endPos = stmt.End()
		case *ast.File:
			//fmt.Printf("PACKAGE Found '%v' at %v\n", stmt.Name.Name, pass.Fset.Position(stmt.Name.NamePos))
		}
	})

	return nil, nil
}

func getNameAndType(fun ast.Expr) (string, string) {
	switch expr := fun.(type) {
	case *ast.SelectorExpr:
		return expr.Sel.Name, getXType(expr)
	case *ast.Ident:
		return expr.Name, ""
	}
	return "", ""
}

// for a selector expression like "X.Sel", get the type of X. If not a slector expression, return with empty string
func getXType(expr *ast.SelectorExpr) string {
	var i *ast.Ident
	switch e := expr.X.(type) {
	case *ast.SelectorExpr:
		i = e.Sel
	case *ast.Ident:
		i = e
	}
	if i != nil {
		switch dec := i.Obj.Decl.(type) {
		case *ast.ValueSpec:
			return getName(dec.Type)
		case *ast.Field:
			return getName(dec.Type)
		}
	}
	return ""
}

func getName(e ast.Expr) string {
	var name string
	ast.Inspect(e, func(n ast.Node) bool {
		if i, ok := n.(*ast.Ident); ok {
			name = i.Name
			return false
		}
		return true
	})
	return name
}

func hasNestedRLock(funcName string, ofType string, inspect *inspector.Inspector, f *token.FileSet) (retStack string) {
	node := findCallDeclarationNode(funcName, ofType, inspect)
	if node == nil {
		return ""
	}
	ast.Inspect(node, func(iNode ast.Node) bool {
		switch stmt := iNode.(type) {
		case *ast.CallExpr:
			name := getName(stmt.Fun)
			addition := fmt.Sprintf("\tat %v\n", f.Position(iNode.Pos()))
			if funcToTest == currentFunc {
				fmt.Println(addition)
			}
			if name == "RLock" { // if the method found is an RLock method
				retStack += addition
			} else if name != "RUnlock" && name != funcName { // name should not equal the previousName to prevent infinite recursive loop
				stack := hasNestedRLock(name, ofType, inspect, f)
				if stack != "" {
					retStack += addition + stack
				}
			}
		}
		return true
	})
	return retStack
}

func findCallDeclarationNode(targetName string, targetType string, inspect *inspector.Inspector) ast.Node {
	var retNode ast.Node = nil
	nodeFilter := []ast.Node{
		(*ast.FuncDecl)(nil),
	}
	inspect.Preorder(nodeFilter, func(node ast.Node) {
		funcDec, _ := node.(*ast.FuncDecl)
		name := funcDec.Name.Name
		if targetType != "" { // are we looking for a method of a specific type?
			if funcDec.Recv == nil { // if this particular call declaration isn't even a method, we can move on
				return
			}
			if targetType != getName(funcDec.Recv.List[0].Type) { // if the found type does not equal the target type, we can move on
				return
			}
		}
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

// FOR REFERENCE: THIS IS BREAKING IN beacon-chain/state/stategen at setter.go at SaveState() function declaration
/*
Alright so what is happening is two-fold:
1. My program cannot differentiate between two different methods of the same name but belonging to diff types
2. My program loops through recursion of nested levels (calling foo() calls bar() which calls bang() which calls foo() again)

Solution for 1:
- Check types by looking at the Object field of the Selector Expression identifier
- Keep this type in storage and match it when using the "findCallDeclarationNode" helper function
*/
