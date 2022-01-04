package main

import (
	"errors"
	"fmt"
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/astutil"
	"golang.org/x/tools/go/ast/inspector"
)

func run1(pass *analysis.Pass) (interface{}, error) {
	inspect, ok := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)
	if !ok {
		return nil, errors.New("analyzer is not type *inspector.Inspector")
	}
	debug := debugHelper{
		pass: pass,
	}
	// filters out other pieces of source code except for function/method calls
	nodeFilter := []ast.Node{
		(*ast.CallExpr)(nil),
	}
	var selList *selIdentList
	inspect.Preorder(nodeFilter, func(node ast.Node) {
		switch stmt := node.(type) {
		case *ast.CallExpr:
			if selList == nil {
				selList = mapSelTypes(stmt, pass)
			} else {
				var sL *selIdentList = mapSelTypes(stmt, pass)
				debug.log(stmt, 14, 15, "/Users/chase/Documents/dev/personalGoExperiments/learnAnalysis/sampleLock2/lock.go", "%v\n", selList.isEqual(sL, 1))
			}
		}
	})
	return nil, nil
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

func (s *selIdentList) isEqualDebug(s2 *selIdentList, offset int, node ast.Node, pass *analysis.Pass) bool {
	d := &debugHelper{pass: pass}
	if s == nil {
		fmt.Printf("%v\n%v\n%v\n", d.getPosition(node), s, s2)
	}
	return false
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
