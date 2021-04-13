package sampleLock

var mutex *Mutex = NewMutex()

//var resource *ProtectResource = &ProtectResource{mutex: mutex, resource: "protected"}

func DoSomething() {
	mutex.RLock()
	DoSomethingElse()
	mutex.RUnlock()
}

func DoSomethingElse() {
	YetAnotherThing()
}

func YetAnotherThing() {
	mutex.RLock()
}
