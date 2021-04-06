package sampleLock

var mutex *Mutex = NewMutex()
var resource *ProtectResource = &ProtectResource{mutex: mutex, resource: "protected"}

func DoSomething() {
	mutex.Lock()
	resource.GetResource()
	mutex.Unlock()
}
