package outliers

// func FuncLitCalled() {
// 	x := 1
// 	defer func() {
// 		x += 1
// 	}()
// 	if x != 1 {
// 		return
// 	}
// }

// func VarStmtCalled() {
// 	var y = 1
// 	var x func() = func() {
// 		y += 1
// 	}
// 	x()
// }

// func AssignStmtCalled() {
// 	var y = 1
// 	x := func() {
// 		y += 1
// 	}
// 	x()
// }

// func StructMethodCalled() {
// 	x := 1
// 	s := &struct{ metho func() }{
// 		metho: func() {
// 			x += 1
// 		},
// 	}
// 	s.metho()
// }

// func StructMethodCalledUnary() {
// 	x := 1
// 	s := &struct{ metho func() }{
// 		metho: func() {
// 			x += 1
// 		},
// 	}
// 	(*s).metho()
// }

func StructMethodAssignedThenCalled() {
	x := 1
	var s *struct{ metho func() }
	s.metho = func() {
		x += 1
	}
	s.metho()
}
