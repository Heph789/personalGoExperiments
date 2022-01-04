package sampleLock2

import (
	"sync"
)

type ProtectResource struct {
	*sync.RWMutex
	resource string
}

func (r *ProtectResource) GetResource() string {
	defer r.RUnlock()
	r.RLock()
	return r.resource
}

type NotProtected struct {
	resource string
	*sync.RWMutex
}

func (r *NotProtected) GetResource() string {
	return r.resource
}

func (r *NotProtected) DummyFunc() int {
	return 1 + 1
}

type NestedResource struct {
	ProtectResource
	n    *NotProtected
	nest *NestedResource
}

func (ne *NestedResource) GetNestedResource() string {
	if ne.nest == nil {
		return ""
	}
	return ne.nest.GetResource()
}
