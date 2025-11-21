package example

import (
	"sync"
	"workflow/taskpool"
)

// implement taskpool.TaskStorer interface methods
type myStore struct {
	store map[string]taskpool.Tasker
	mu    sync.RWMutex
}

func NewMyStore() *myStore {
	return &myStore{
		store: make(map[string]taskpool.Tasker),
	}
}

func (ms *myStore) GetStoreTask(id string) (taskpool.Tasker, bool) {
	ms.mu.RLock()
	defer ms.mu.RUnlock()
	t, ok := ms.store[id]
	return t, ok
}

func (ms *myStore) SetStoreTask(t taskpool.Tasker) {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	ms.store[t.GetID()] = t
}

func (ms *myStore) DeleteStoreTask(id string) {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	delete(ms.store, id)
}
