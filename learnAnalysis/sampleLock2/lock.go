package sampleLock2

import "github.com/Heph789/personalGoExperiments/learnAnalysis/sampleLock2/iTypes"

var resource *ProtectResource = &ProtectResource{resource: "protected"}
var a *iTypes.AwesomeProtectedResource

func DoSomething() {
	resource.RLock()
	a.SetResource("protected")
	resource.GetResource() // should be giving warning but analyzer is looking at NotProtected.GetResource instead
	resource.RUnlock()
}

func AnotherWayToDoSomething(r *ProtectResource) {
	r.RLock()
	r.GetResource()
	r.RUnlock()
}
