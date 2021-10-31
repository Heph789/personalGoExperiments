package sa

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
	Doc:      "Checks for recursive or nested RLock calls",
	Requires: []*analysis.Analyzer{inspect.Analyzer},
	Run:      run,
}

var errNestedRLock = errors.New("found recursive read lock call")

var once bool = true

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

		switch stmt := node.(type) {
		case *ast.CallExpr:
			call := getCallInfo(pass.TypesInfo, stmt)
			if call == nil {
				break
			}
			name := call.id
			if name == "RLock" { // if the method found is an RLock method
				if keepTrackOf.foundRLock > 0 { // if we have already seen an RLock method without seeing a corresponding RUnlock
					pass.Reportf(
						node.Pos(),
						fmt.Sprintf(
							"%v",
							errNestedRLock,
						),
					)
				}
				keepTrackOf.incFRU()
			} else if name == "RUnlock" && !keepTrackOf.deferredRUnlock {
				keepTrackOf.deincFRU()
			} else if name != "RUnlock" && keepTrackOf.foundRLock > 0 {
				if stack := hasNestedRLock(call, inspect, pass, make(map[string]bool)); stack != "" {
					pass.Reportf(
						node.Pos(),
						fmt.Sprintf(
							"%v\n%v",
							errNestedRLock,
							stack,
						),
					)
				}
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

type tracker struct {
	funcEnd         token.Pos
	retEnd          token.Pos
	deferEnd        token.Pos
	funcLitEnd      token.Pos
	deferredRUnlock bool
	foundRLock      int
}

func (t tracker) toString() string {
	return fmt.Sprintf("funcEnd:%v\nretEnd:%v\ndeferEnd:%v\ndeferredRU:%v\nfoundRLock:%v\n", t.funcEnd, t.retEnd, t.deferEnd, t.deferredRUnlock, t.foundRLock)
}

func (t *tracker) deincFRU() {
	if t.foundRLock > 0 {
		t.foundRLock -= 1
	}
}
func (t *tracker) incFRU() {
	t.foundRLock += 1
}

// debug functions and helpers
type debugHelper struct {
	pass *analysis.Pass
}

func (d *debugHelper) getPosition(node ast.Node) token.Position {
	return d.pass.Fset.File(node.Pos()).Position(node.Pos())
}

func (d *debugHelper) log(node ast.Node, a int, b int, fileName string, format string, c ...interface{}) {
	lineNumber := d.pass.Fset.File(node.Pos()).Line(node.Pos())
	fName := d.pass.Fset.File(node.Pos()).Name()
	if a <= lineNumber && lineNumber <= b && fName == fileName {
		fmt.Printf(format, c...)
	}
}

type callInfo struct {
	call *ast.CallExpr
	id   string     // type ID [either the name (if the function is exported) or the package/name if otherwise] of the function/method
	typ  types.Type // type of the method receiver (nil if a function)
}

// returns true if callInfo represents a method, false if it is a function
func (c *callInfo) isMethod() bool {
	return c.typ != nil
}

func (c *callInfo) String() string {
	if c.isMethod() {
		return fmt.Sprintf("%v: %v", c.id, c.typ.String())
	}
	return c.id
}

// returns a *callInfo struct with call info (ID and type)
func getCallInfo(tInfo *types.Info, call *ast.CallExpr) (c *callInfo) {
	c = &callInfo{}
	c.call = call
	f := typeutil.Callee(tInfo, call)
	if f == nil {
		return nil
	}
	if _, isBuiltin := f.(*types.Builtin); isBuiltin {
		return nil
	}
	s, ok := f.Type().(*types.Signature)
	if ok {
		if interfaceMethod(s) {
			return nil
		}
		if r := s.Recv(); r != nil {
			c.typ = r.Type()
		}
	}
	c.id = f.Id()
	return c
}

func interfaceMethod(s *types.Signature) bool {
	recv := s.Recv()
	return recv != nil && types.IsInterface(recv.Type())
}

// hasNestedRLock takes a call expression represented by callInfo as input and returns a stack trace of the nested or recursive RLock within
// that call expression. If the call expression does not contain a nested or recursive RLock, hasNestedRLock returns an empty string.
// hasNestedRLock finds a nested or recursive RLock by recursively calling itself on any functions called by the function/method represented
// by callInfo.
func hasNestedRLock(call *callInfo, inspect *inspector.Inspector, pass *analysis.Pass, hist map[string]bool) (retStack string) {
	f := pass.Fset
	tInfo := pass.TypesInfo
	cH := callHelper{
		call: call.call,
		fset: pass.Fset,
	}
	var node ast.Node = cH.identifyFuncLitBlock(call.call.Fun)
	if node == (*ast.BlockStmt)(nil) {
		node = findCallDeclarationNode(call, inspect, pass.TypesInfo)
	}
	if node == (*ast.FuncDecl)(nil) {
		return ""
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

			if name == "RLock" { // if the method found is an RLock method
				retStack += addition + fmt.Sprintf("\t%q at %v\n", name, f.Position(iNode.Pos()))
			} else if name != "RUnlock" { // name should not equal the previousName to prevent infinite recursive loop
				nt := c.String()
				if !hist[nt] { // make sure we are not in an infinite recursive loop
					hist[nt] = true
					stack := hasNestedRLock(c, inspect, pass, hist)
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
func findCallDeclarationNode(c *callInfo, inspect *inspector.Inspector, tInfo *types.Info) *ast.FuncDecl {
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

type callHelper struct {
	call *ast.CallExpr
	fset *token.FileSet
}

func (c callHelper) identifyFuncLitBlock(expr ast.Expr) *ast.BlockStmt {
	switch stmt := expr.(type) {
	case *ast.FuncLit:
		return stmt.Body
	case *ast.CallExpr:
		return nil
	case *ast.Ident:
		if stmt.Obj != nil {
			switch objDecl := stmt.Obj.Decl.(type) {
			case *ast.ValueSpec:
				identIndex := findIdentIndex(stmt, objDecl.Names)
				if identIndex != -1 {
					value := objDecl.Values[identIndex]
					return c.identifyFuncLitBlock(value)
				}
			case *ast.AssignStmt:
				exprIndex := findExprIndex(c.call.Fun, objDecl.Lhs)
				if exprIndex != -1 && len(objDecl.Lhs) == len(objDecl.Rhs) { // only deals with simple func lit assignments
					value := objDecl.Rhs[exprIndex]
					return c.identifyFuncLitBlock(value)
				}
				// if exprIndex >= len(objDecl.Rhs) && once {
				// 	ast.Print(c.fset, stmt)
				// 	once = false
				// }
			}
		}
	}
	return nil
}

func findIdentIndex(id *ast.Ident, exprs []*ast.Ident) int {
	for i, v := range exprs {
		if v.Name == id.Name {
			return i
		}
	}
	return -1
}

func findExprIndex(expr ast.Expr, exprs []ast.Expr) int {
	switch stmt := expr.(type) {
	case *ast.Ident:
		return findIdentIndexFromExpr(stmt, exprs)
	case *ast.SelectorExpr:
		return findSelectorIndex(stmt, exprs)
	}
	return -1
}

func findIdentIndexFromExpr(id *ast.Ident, exprs []ast.Expr) int {
	for i, v := range exprs {
		if val, ok := v.(*ast.Ident); ok && val.Name == id.Name {
			return i
		}
	}
	return -1
}

func findSelectorIndex(expr ast.Expr, exprs []ast.Expr) int {
	return 0 // a place holder for later
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

// Plan for fixing multiple return statements bug:
/*
- Keep track of the end position of a return statement
- Decrement foundRLock until the end position of return statement is reached
- Reincrement foundRLock
*/
