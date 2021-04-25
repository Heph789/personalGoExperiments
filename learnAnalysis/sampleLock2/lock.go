package sampleLock2

import "github.com/Heph789/personalGoExperiments/learnAnalysis/sampleLock2/iTypes"

var resource *ProtectResource = &ProtectResource{resource: "protected"}
var a *iTypes.AwesomeProtectedResource
var nested *NestedResource = &NestedResource{n: &NotProtected{resource: "hello"}}

func DoSomething() {
	resource.RLock()
	a.SetResource("protected")
	resource.GetResource() // should be giving warning but analyzer is looking at NotProtected.GetResource instead
	nested.GetResource()
	nested.n.GetResource()
	resource.RUnlock()
}

func AnotherWayToDoSomething(r *ProtectResource) {
	r.RLock()
	r.GetResource()
	r.RUnlock()
}
