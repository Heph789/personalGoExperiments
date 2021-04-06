package sampleLock

import "fmt"

type ProtectResource struct {
	resource string
	mutex    *Mutex
}

func (r *ProtectResource) GetResource() string {
	defer r.mutex.Unlock()
	r.mutex.Lock()
	return r.resource
}

type Mutex struct {
	isLocked bool
}

func (m *Mutex) Lock() bool {
	fmt.Println("Locking")
	var old bool
	m.isLocked, old = true, m.isLocked
	return old // returns true if we are now deadlocked.
}

func (m *Mutex) Unlock() {
	fmt.Println("Unlocking")
	m.isLocked = false
}

func NewMutex() *Mutex {
	mutex := Mutex{isLocked: false}
	return &mutex
}
