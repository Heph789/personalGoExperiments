package main

import (
	"errors"
	"go/ast"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

func run9(pass *analysis.Pass) (interface{}, error) {
	inspect, ok := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)
	if !ok {
		return nil, errors.New("analyzer is not type *inspector.Inspector")
	}

	// filters out other pieces of source code except for function/method calls
	nodeFilter := []ast.Node{
		(*ast.CallExpr)(nil),
		(*ast.DeferStmt)(nil),
		(*ast.FuncDecl)(nil),
		(*ast.FuncLit)(nil),
		(*ast.File)(nil),
		(*ast.ReturnStmt)(nil),
	}

	debug := &debugHelper{
		pass: pass,
	}
	inspect.Preorder(nodeFilter, func(node ast.Node) {
		switch stmt := node.(type) {
		case *ast.FuncDecl:
			if stmt.Recv != nil {
				recv := stmt.Recv.List[0].Names[0]
				selNode := &selIdentNode{
					this:   recv,
					typObj: pass.TypesInfo.ObjectOf(recv),
				}
				debug.log(stmt, 10, 10, "/Users/chase/Documents/dev/personalGoExperiments/learnAnalysis/sampleLock2/iTypes/types.go", "%v", selNode.typObj.Type())
			}
		}
	})
	return nil, nil
}
