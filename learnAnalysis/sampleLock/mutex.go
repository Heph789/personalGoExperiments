package sampleLock

import "fmt"

type ProtectResource struct {
	resource string
	mutex    *Mutex
}

func (r *ProtectResource) GetResource() string {
	defer r.mutex.RUnlock()
	r.mutex.RLock()
	return r.resource
}

type Mutex struct {
	isLocked bool
}

func (m *Mutex) RLock() bool {
	fmt.Println("Locking")
	var old bool
	m.isLocked, old = true, m.isLocked
	return old // returns true if we are now deadlocked.
}

func (m *Mutex) RUnlock() {
	fmt.Println("Unlocking")
	m.isLocked = false
}

func NewMutex() *Mutex {
	mutex := Mutex{isLocked: false}
	return &mutex
}
