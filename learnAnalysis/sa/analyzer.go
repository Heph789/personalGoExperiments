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
	"golang.org/x/tools/go/ast/astutil"
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
			if keepTrackOf.foundRLock > 0 && keepTrackOf.rLockSelector.isEqual(selMap, 0) {
				pass.Reportf(
					node.Pos(),
					fmt.Sprintf(
						"%v",
						errNestedRLock,
					),
				)
			} else if keepTrackOf.foundRLock > 0 {
				if stack := hasNestedRLock(keepTrackOf.rLockSelector, selMap, call, inspect, pass, make(map[string]bool)); stack != "" {
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

type tracker struct {
	funcEnd         token.Pos
	retEnd          token.Pos
	deferEnd        token.Pos
	funcLitEnd      token.Pos
	deferredRUnlock bool
	foundRLock      int
	rLockSelector   *selIdentList
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

type selIdentNode struct {
	next   *selIdentNode
	this   *ast.Ident
	typObj types.Object
}

type selIdentList struct {
	start        *selIdentNode
	length       int
	current      *selIdentNode
	currentIndex int
}

func (s *selIdentList) next() (n *selIdentNode) {
	n = s.current.next
	if n != nil {
		s.current = n
		s.currentIndex++
	}
	return n
}

func (s *selIdentList) reset() {
	s.current = s.start
	s.currentIndex = 0
}

func (s *selIdentList) isEqual(s2 *selIdentList, offset int) bool {
	if s2 == nil || (s.length != s2.length) {
		return false
	}
	s.reset()
	s2.reset()
	for i := true; i; {
		if !s.current.isEqual(s2.current) {
			return false
		}
		if s.currentIndex < s.length-offset-1 && s.next() != nil {
			s2.next()
		} else {
			i = false
		}
	}
	return true
}

func (s *selIdentList) getSub(s2 *selIdentList) *selIdentList {
	if s2 == nil || s2.length > s.length {
		return nil
	}
	s.reset()
	s2.reset()
	for i := true; i; {
		if !s.current.isEqual(s2.current) {
			return nil
		}
		if s2.currentIndex != s2.length-2 { // might want to add a selNode.prev() func
			s.next()
			s2.next()
		} else {
			i = false
		}
	}
	return &selIdentList{
		start:        s.current,
		length:       s.length - s.currentIndex,
		current:      s.current,
		currentIndex: 0,
	}
}

func (s *selIdentList) changeRoot(r *ast.Ident, t types.Object) {
	selNode := &selIdentNode{
		this:   r,
		next:   s.start.next,
		typObj: t,
	}
	if s.start == s.current {
		s.start = selNode
		s.current = selNode
	} else {
		s.start = selNode
	}
}

func (s selIdentList) String() (str string) {
	var temp *selIdentNode = s.start
	str = fmt.Sprintf("length: %v\n[\n", s.length)
	for i := 0; temp != nil; i++ {
		if i == s.currentIndex {
			str += "*"
		}
		str += fmt.Sprintf("%v: %v\n", i, temp)
		temp = temp.next
	}
	str += "]"
	return str
}

func (s *selIdentNode) isEqual(s2 *selIdentNode) bool {
	return (s.this.Name == s2.this.Name) && (s.typObj == s2.typObj)
}

func (s selIdentNode) String() string {
	return fmt.Sprintf("{ ident: '%v', type: '%v' }", s.this, s.typObj)
}

func mapSelTypes(c *ast.CallExpr, pass *analysis.Pass) *selIdentList {
	list := &selIdentList{}
	valid := list.recurMapSelTypes(c.Fun, nil, pass.TypesInfo)
	if !valid {
		return nil
	}
	return list
}

func (l *selIdentList) recurMapSelTypes(e ast.Expr, next *selIdentNode, t *types.Info) bool {
	expr := astutil.Unparen(e)
	l.length++
	s := &selIdentNode{next: next}
	switch stmt := expr.(type) {
	case *ast.Ident:
		s.this = stmt
		s.typObj = t.ObjectOf(stmt)
	case *ast.SelectorExpr:
		s.this = stmt.Sel
		if sel, ok := t.Selections[stmt]; ok {
			s.typObj = sel.Obj() // method or field
		} else {
			s.typObj = t.Uses[stmt.Sel] // qualified identifier?
		}
		return l.recurMapSelTypes(stmt.X, s, t)
	default:
		return false
	}
	l.current = s
	l.start = s
	return true
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
func hasNestedRLock(fullRLockSelector *selIdentList, compareMap *selIdentList, call *callInfo, inspect *inspector.Inspector, pass *analysis.Pass, hist map[string]bool) (retStack string) {
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
		node = findCallDeclarationNode(call, inspect, pass.TypesInfo)
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
					stack := hasNestedRLock(rLockSelector, selMap, c, inspect, pass, hist)
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
