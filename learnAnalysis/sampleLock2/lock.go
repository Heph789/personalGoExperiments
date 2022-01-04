package sampleLock2

import "github.com/Heph789/personalGoExperiments/learnAnalysis/sampleLock2/iTypes"

var resource *ProtectResource = &ProtectResource{resource: "protected"}
var a *iTypes.AwesomeProtectedResource
var nested *NestedResource = &NestedResource{
	n:    &NotProtected{resource: "hello"},
	nest: &NestedResource{n: &NotProtected{resource: "goodbye"}},
}

func DoSomething() {
	nested.nest.RLock()
	fun := func() {
		nested.GetNestedResource()
	}
	fun()
	nested.nest.RUnlock()
}

func AnotherWayToDoSomething(r *ProtectResource) {
	r.RLock()
	r.GetResource()
	r.RUnlock()
}
