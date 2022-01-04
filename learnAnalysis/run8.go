package main

import (
	"errors"
	"fmt"
	"go/ast"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

func run8(pass *analysis.Pass) (interface{}, error) {
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

	// debug := &debugHelper{
	// 	pass: pass,
	// }
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
			if selMap == nil {
				break
			}
			if keepTrackOf.foundRLock > 0 && keepTrackOf.rLockSelector.isEqualDebug(selMap, 0, node, pass) {
				// debug.log(stmt, 10, 19, "/Users/chase/Documents/dev/personalGoExperiments/learnAnalysis/sampleLock2/lock.go", "%v\n", "found RLock")
			} else if keepTrackOf.foundRLock > 0 {
				retStack := hasNestedRLockS(keepTrackOf.rLockSelector, selMap, call, inspect, pass, make(map[string]bool))
				fmt.Printf("%v", retStack)
			}
			if name == "RLock" && keepTrackOf.foundRLock == 0 {
				keepTrackOf.rLockSelector = selMap
				keepTrackOf.incFRU()
			}
			if name == "RUnlock" && keepTrackOf.rLockSelector.isEqual(selMap, 1) {
				keepTrackOf.deincFRU()
				//debug.log(stmt, 10, 19, "/Users/chase/Documents/dev/personalGoExperiments/learnAnalysis/sampleLock2/lock.go", "%v\n", "deincr")
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

// hasNestedRLock takes a call expression represented by callInfo as input and returns a stack trace of the nested or recursive RLock within
// that call expression. If the call expression does not contain a nested or recursive RLock, hasNestedRLock returns an empty string.
// hasNestedRLock finds a nested or recursive RLock by recursively calling itself on any functions called by the function/method represented
// by callInfo.
func hasNestedRLockS(fullRLockSelector *selIdentList, compareMap *selIdentList, call *callInfo, inspect *inspector.Inspector, pass *analysis.Pass, hist map[string]bool) (retStack string) {
	// debug := debugHelper{
	// 	pass: pass,
	// }
	var rLockSelector *selIdentList
	f := pass.Fset
	tInfo := pass.TypesInfo
	cH := callHelper{
		call: call.call,
		fset: pass.Fset,
	}
	var node ast.Node = cH.identifyFuncLitBlock(cH.call.Fun) // this seems a bit redundant
	var recv *ast.Ident
	if node == (*ast.BlockStmt)(nil) {
		subMap := fullRLockSelector.getSub(compareMap)
		if subMap != nil {
			rLockSelector = subMap
		} else {
			return "" // if this is not a local function literal call, and the selectors don't match up, then we can just return
		}
		node = findCallDeclarationNodeS(call, inspect, pass.TypesInfo)
		if node == (*ast.FuncDecl)(nil) {
			return ""
		} else if castedNode := node.(*ast.FuncDecl); castedNode.Recv != nil {
			recv = castedNode.Recv.List[0].Names[0]
			rLockSelector.changeRoot(recv, pass.TypesInfo.ObjectOf(recv))
		}
	} else {
		rLockSelector = fullRLockSelector // no need to find a submap, since this is a local function call
	}
	addition := fmt.Sprintf("\t%q at %v\n", call.id, f.Position(call.call.Pos()))
	ast.Inspect(node, func(iNode ast.Node) bool {
		switch stmt := iNode.(type) {
		case *ast.CallExpr:
			c := getCallInfo(tInfo, stmt)
			if c == nil {
				return false
			}
			name := c.id
			selMap := mapSelTypes(stmt, pass)
			if rLockSelector.isEqual(selMap, 0) { // if the method found is an RLock method
				retStack += addition + fmt.Sprintf("\t%q at %v\n", name, f.Position(iNode.Pos()))
			} else if name != "RUnlock" { // name should not equal the previousName to prevent infinite recursive loop
				nt := c.String()
				if !hist[nt] { // make sure we are not in an infinite recursive loop
					hist[nt] = true
					stack := hasNestedRLockS(rLockSelector, selMap, c, inspect, pass, hist)
					delete(hist, nt)
					if stack != "" {
						retStack += addition + stack
					}
				}
			}
		}
		return true
	})
	return retStack
}

// findCallDeclarationNode takes a callInfo struct and inspects the AST of the package
// to find a matching method or function declaration. It returns this declaration of type *ast.FuncDecl
func findCallDeclarationNodeS(c *callInfo, inspect *inspector.Inspector, tInfo *types.Info) *ast.FuncDecl {
	var retNode *ast.FuncDecl = nil
	nodeFilter := []ast.Node{
		(*ast.FuncDecl)(nil),
	}
	inspect.Preorder(nodeFilter, func(node ast.Node) {
		funcDec, _ := node.(*ast.FuncDecl)
		name := tInfo.ObjectOf(funcDec.Name).Id()
		if c.isMethod() { // are we looking for a method of a specific type?
			if funcDec.Recv == nil { // if this particular call declaration isn't even a method, we can move on
				return
			}
			if t := tInfo.TypeOf(funcDec.Recv.List[0].Type); !types.Identical(t, c.typ) { // if the found type does not equal the target type, we can move on
				// fmt.Printf("Found call declaration of type %v\n", t)
				return
			}
		} else if funcDec.Recv != nil { // if we are looking for a function, ignore methods
			return
		}
		if c.id == name {
			retNode = funcDec
		}
	})
	return retNode
}
