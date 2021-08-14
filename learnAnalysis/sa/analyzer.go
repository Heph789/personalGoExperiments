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

var alreadyPrinted bool = false

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
		(*ast.ReturnStmt)(nil),
	}

	foundRLock := 0
	deferredRLock := false
	funcEnd := token.NoPos
	retEnd := token.NoPos
	inspect.Preorder(nodeFilter, func(node ast.Node) {
		if retEnd.IsValid() && node.Pos() > retEnd { // if we are in a return statement
			// lineNumber := pass.Fset.File(node.Pos()).Line(node.Pos())
			// if lineNumber > 153 && lineNumber < 175 {
			// 	fmt.Printf("finding RLock in retEnd.isValid\n")
			// }
			foundRLock++
			retEnd = token.NoPos
			deferredRLock = true
		}
		if funcEnd.IsValid() && node.Pos() > funcEnd { // if we are past the function, then the deferred RUnlock has been called
			if deferredRLock {
				deferredRLock = false
				foundRLock--
			}
			funcEnd = token.NoPos
		}
		switch stmt := node.(type) {
		case *ast.CallExpr:
			call := getCallInfo(pass.TypesInfo, stmt)
			lineNumber := pass.Fset.File(node.Pos()).Line(node.Pos())
			if lineNumber == 553 && pass.Fset.File(node.Pos()).Name() == "/mnt/e/Development/Ethereum+Truffle/prysm/beacon-chain/p2p/peers/status.go" && !alreadyPrinted {
				// name := "N/A"
				// if call != nil {
				// 	name = call.id
				// }
				// fmt.Printf("Name: %v; foundRLock: %v; deferredRLock: %v; Position: %v; retEnd: %v; Number pos: %v\n", name, foundRLock, deferredRLock, pass.Fset.Position(stmt.Pos()), retEnd, stmt.Pos())
				ast.Print(pass.Fset, node)
				alreadyPrinted = true
			}
			// if 532 < lineNumber && lineNumber < 560 && pass.Fset.File(node.Pos()).Name() == "/mnt/e/Development/Ethereum+Truffle/prysm/beacon-chain/p2p/peers/status.go" && call != nil {
			// 	// call2 := typeutil.Callee(pass.TypesInfo, stmt)
			// 	// _, ok := call2.Type().(*types.Signature)
			// 	// // if ok {
			// 	// // 	_, ok2 := theVar.Type().(*types.Signature)
			// 	// // 	fmt.Println(ok2)
			// 	// // }
			// 	// callString := types.ObjectString(call2, nil)
			// 	// ast.Print(pass.Fset, stmt)
			// 	fmt.Println(call.String())
			// } else if call == nil {
			// 	fmt.Println("nil")
			// }
			// if call == nil {
			// 	break
			// }
			break
			name := call.id
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
				// lineNumber := pass.Fset.File(stmt.Pos()).Line(stmt.Pos())
				// if lineNumber > 153 && lineNumber < 175 {
				// 	fmt.Printf("finding RLock in callExpr\n")
				// }
				foundRLock++
			} else if name == "RUnlock" && !deferredRLock {
				foundRLock--
			} else if name != "RUnlock" && foundRLock > 0 {
				// fmt.Printf("METHOD Found '%v' of type '%v' at %v\n", name, t, pass.Fset.Position(node.Pos()))
				// ast.Print(pass.Fset, findCallDeclarationNode(getName(stmt.Fun), t, inspect))
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
		case *ast.DeferStmt:
			call := getCallInfo(pass.TypesInfo, stmt.Call)
			if call != nil && call.id == "RUnlock" {
				deferredRLock = true
			}
			// lineNumber := pass.Fset.File(stmt.Pos()).Line(stmt.Pos())
			// if lineNumber == 156 {
			// 	fmt.Printf("We are deferring. Here is the value: %v\n", deferredRLock)
			// }
		case *ast.FuncDecl:
			lineNumber := pass.Fset.File(stmt.Pos()).Line(stmt.Pos())
			if lineNumber > 532 && lineNumber < 559 {
				// fmt.Printf("funcEnd: %v; newFuncEnd: %v; currentPos:%v\n", funcEnd, stmt.End(), stmt.Pos())
				// fmt.Printf("Func name: %v\n", pass.TypesInfo.ObjectOf(stmt.Name).Id())
			}
			if funcEnd == token.NoPos {
				funcEnd = stmt.End()
			}
		case *ast.ReturnStmt:
			if deferredRLock { // only keep track of return end if RUnlock is deferred
				deferredRLock = false
				foundRLock--
				retEnd = stmt.End()
			}
			// lineNumber := pass.Fset.File(stmt.Pos()).Line(stmt.Pos())
			// if lineNumber == 159 || lineNumber == 161 {
			// 	fmt.Printf("deferred: %v; foundRL: %v; retEnd:%v; pos:%v\n", deferredRLock, foundRLock, retEnd, stmt.Pos())
			// }
		}
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

func (c *callInfo) String() string {
	if c.isMethod() {
		return fmt.Sprintf("%v: %v", c.id, c.typ.String())
	}
	return c.id
}

// returns a *callInfo struct with call info (ID and type)
func getCallInfo(tInfo *types.Info, call *ast.CallExpr) (c *callInfo) {
	c = &callInfo{}
	f := typeutil.Callee(tInfo, call)
	if f == nil {
		return nil
	}
	s, ok := f.Type().(*types.Signature)
	if _, isBuiltin := f.(*types.Builtin); isBuiltin || interfaceMethod(s) {
		return nil
	}
	c.id = f.Id()
	if r := s.Recv(); ok && r != nil {
		c.typ = r.Type()
	}
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
	node := findCallDeclarationNode(call, inspect, pass.TypesInfo)
	if node == nil {
		return ""
	}
	addition := fmt.Sprintf("\t%q at %v\n", call.id, f.Position(node.Pos()))
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
				// fmt.Printf("Type: %v\n", t)
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
