package main

// IMPORTANT NOTE: DEFERRED FUNCTION IS EXECUTED RIGHT AFTER RETURN STATEMENT.
// NOT AFTER FUNCTION CALL IN RETURN STATEMENT

import (
	"errors"
	"fmt"

	// "container/list"

	"go/ast"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
	"golang.org/x/tools/go/types/typeutil"
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
	return run3(pass)
}

func run1(pass *analysis.Pass) (interface{}, error) {
	inspect, ok := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)
	if !ok {
		return nil, errors.New("analyzer is not type *inspector.Inspector")
	}

	// filters out other pieces of source code except for function/method calls
	nodeFilter := []ast.Node{
		(*ast.Ident)(nil),
	}
	inspect.Preorder(nodeFilter, func(node ast.Node) {
		i, _ := node.(*ast.Ident)
		if fmt.Sprint(pass.Fset.Position(i.NamePos)) == "/mnt/e/Development/Ethereum+Truffle/prysm/beacon-chain/state/stategen/setter.go:57:15" {
			ast.Print(pass.Fset, i)
		}
	})
	return nil, nil
}

func run2(pass *analysis.Pass) (interface{}, error) {
	inspect, ok := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)
	if !ok {
		return nil, errors.New("analyzer is not type *inspector.Inspector")
	}

	// filters out other pieces of source code except for function/method calls
	nodeFilter := []ast.Node{
		(*ast.CallExpr)(nil),
	}
	inspect.Preorder(nodeFilter, func(node ast.Node) {
		i, _ := node.(*ast.CallExpr)
		fmt.Println(typeutil.StaticCallee(pass.TypesInfo, i).FullName())
		// s, ok := typeutil.StaticCallee(pass.TypesInfo, i).Type().(*types.Signature)
		// if r := s.Recv(); ok && r != nil {
		// 	fmt.Println(r.Type())
		// }

	})
	return nil, nil
}

func run3(pass *analysis.Pass) (interface{}, error) {
	inspect, ok := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)
	if !ok {
		return nil, errors.New("analyzer is not type *inspector.Inspector")
	}

	// filters out other pieces of source code except for function/method calls
	nodeFilter := []ast.Node{
		(*ast.FuncDecl)(nil),
	}
	inspect.Preorder(nodeFilter, func(node ast.Node) {
		i, _ := node.(*ast.FuncDecl)
		fmt.Println(pass.TypesInfo.ObjectOf(i.Name).Id())
		// if i.Recv != nil {
		// 	fmt.Println(pass.TypesInfo.TypeOf(i.Recv.List[0].Type))
		// }

	})
	return nil, nil
}

func run4(pass *analysis.Pass) (interface{}, error) {
	inspect, ok := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)
	if !ok {
		return nil, errors.New("analyzer is not type *inspector.Inspector")
	}

	// filters out other pieces of source code except for function/method calls
	nodeFilter := []ast.Node{
		(*ast.CallExpr)(nil),
	}
	inspect.Preorder(nodeFilter, func(node ast.Node) {
		i, _ := node.(*ast.CallExpr)
		c1 := getCallInfo(pass.TypesInfo, i)
		c2 := getCallInfo(pass.TypesInfo, i)
		if c1 != nil && c2 != nil {

			fmt.Println(c1 == c2)
		}
		// s, ok := typeutil.StaticCallee(pass.TypesInfo, i).Type().(*types.Signature)
		// if r := s.Recv(); ok && r != nil {
		// 	fmt.Println(r.Type())
		// }

	})
	return nil, nil
}

type callInfo struct {
	id  string     // type ID [either the name (if the function is exported) or the package/name if otherwise] of the function/method
	typ types.Type // type of the method receiver (nil if a function)
}

// returns true if callInfo represents a method, false if it is a function
func (c *callInfo) isMethod() bool {
	return c.typ != nil
}

// returns a *callInfo struct with call info (ID and type)
func getCallInfo(tInfo *types.Info, call *ast.CallExpr) *callInfo {
	var c *callInfo = &callInfo{}
	f := typeutil.StaticCallee(tInfo, call)
	if f == nil {
		return nil
	}
	fmt.Println(f)
	c.id = "hello"
	s, ok := f.Type().(*types.Signature)
	if r := s.Recv(); ok && r != nil {
		c.typ = r.Type()
	}
	return c
}

// func getNameAndType(fun ast.Expr) (string, string) {
// 	switch expr := fun.(type) {
// 	case *ast.SelectorExpr:
// 		return expr.Sel.Name, getXType(expr)
// 	case *ast.Ident:
// 		return expr.Name, ""
// 	}
// 	return "", ""
// }
var doOnce = true

// for a selector expression like "X.Sel", get the type of X. If not a slector expression, return with empty string
func getXType(expr *ast.SelectorExpr, f *token.FileSet) string {
	var i *ast.Ident
	switch e := expr.X.(type) {
	case *ast.SelectorExpr:
		i = e.Sel
	case *ast.Ident:
		i = e
	}
	if i != nil && i.Obj != nil {
		switch dec := i.Obj.Decl.(type) {
		case *ast.ValueSpec:
			return getName(dec.Type)
		case *ast.Field:
			return getName(dec.Type)
		default:
			if fmt.Sprint(f.Position(i.NamePos)) == "/mnt/e/Development/Ethereum+Truffle/prysm/beacon-chain/state/stategen/setter.go:57:15" && doOnce {
				doOnce = false
				ast.Print(f, i)
			}
		}
	}
	return ""
}

func getName(e ast.Expr) string {
	if e == nil {
		return ""
	}
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
