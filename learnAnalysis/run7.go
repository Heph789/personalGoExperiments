package main

import (
	"errors"
	"go/ast"
	"go/token"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

func run7(pass *analysis.Pass) (interface{}, error) {
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
	var keepTrackOf tracker
	inspect.Preorder(nodeFilter, func(node ast.Node) {
		if keepTrackOf.funcLitEnd.IsValid() && node.Pos() <= keepTrackOf.funcLitEnd {
			return
		} else {
			keepTrackOf.funcLitEnd = token.NoPos
		}
		if keepTrackOf.deferEnd.IsValid() && node.Pos() > keepTrackOf.deferEnd {
			keepTrackOf.deferEnd = token.NoPos
		} else if keepTrackOf.deferEnd.IsValid() {
			return
		}
		if keepTrackOf.retEnd.IsValid() && node.Pos() > keepTrackOf.retEnd {
			keepTrackOf.retEnd = token.NoPos
			keepTrackOf.incFRU()
		}
		switch stmt := node.(type) {
		case *ast.CallExpr:
			call := getCallInfo(pass.TypesInfo, stmt)
			if call == nil {
				break
			}
			name := call.id
			selMap := mapSelTypes(stmt, pass)
			if keepTrackOf.foundRLock > 0 && keepTrackOf.rLockSelector.isEqual(selMap, 0) {
				debug.log(stmt, 10, 19, "/Users/chase/Documents/dev/personalGoExperiments/learnAnalysis/sampleLock2/lock.go", "%v\n", "found RLock")
			}
			if name == "RLock" && keepTrackOf.foundRLock == 0 {
				keepTrackOf.rLockSelector = selMap
				keepTrackOf.incFRU()
			}
			if name == "RUnlock" && keepTrackOf.rLockSelector.isEqual(selMap, 1) {
				keepTrackOf.deincFRU()
				debug.log(stmt, 10, 19, "/Users/chase/Documents/dev/personalGoExperiments/learnAnalysis/sampleLock2/lock.go", "%v\n", "deincr")
			}
		case *ast.File:
			keepTrackOf = tracker{}
		case *ast.FuncDecl:
			keepTrackOf = tracker{}
			keepTrackOf.funcEnd = stmt.End()
		case *ast.FuncLit:
			if keepTrackOf.funcLitEnd == token.NoPos {
				keepTrackOf.funcLitEnd = stmt.End()
			}
		case *ast.DeferStmt:
			call := getCallInfo(pass.TypesInfo, stmt.Call)
			if keepTrackOf.deferEnd == token.NoPos {
				keepTrackOf.deferEnd = stmt.End()
			}
			if call != nil && call.id == "RUnlock" {
				keepTrackOf.deferredRUnlock = true
			}
		case *ast.ReturnStmt:
			if keepTrackOf.deferredRUnlock && keepTrackOf.retEnd == token.NoPos {
				keepTrackOf.deincFRU()
				keepTrackOf.retEnd = stmt.End()
			}
		}
	})
	return nil, nil
}
