package sampleLock

var mutex *Mutex = NewMutex()
var mutex2 *Mutex = mutex

//var resource *ProtectResource = &ProtectResource{mutex: mutex, resource: "protected"}

func DoSomething() {
	mutex.RLock()
	mutex2.RLock()
	mutex2.RUnlock()
	mutex.RUnlock()
}
