package taskpool

import (
	"debug/elf"
	"sync"
)

type Tasker interface {
	Run() error
	GetID() string
	GetStatus() string
	AsyncHandler(resp string)
	UpdateAsyncResp(resp string) // 把resp 存到task里
}

type TaskStorer interface {
	GetStoreTask(id string) (Tasker, bool)
	SetStoreTask(t Tasker)
	DeleteStoreTask(id string)
}

type TaskPool struct {
	/*---------------
	  存储可以替换 为db/redis/mq等
	---------------*/
	TaskStore map[string]Tasker
	mu        sync.RWMutex
	/*----------------
	  ---------------*/

	TaskCh  chan Tasker
	AsyncCh chan Tasker
}

func NewTaskPool(store TaskStorer, size int) *TaskPool {
	tp := &TaskPool{
		TaskStore: make(map[string]Tasker),
		TaskCh:    make(chan Tasker, 100),
		AsyncCh:   make(chan Tasker, 100),
	}

	return tp
}

func (tp *TaskPool) PushTask(t Tasker) error {
	select {
	case tp.TaskCh <- t:
		tp.SetStoreTask(t)
		return nil
	default:
		return &elf.FormatError{}
	}
}
func (tp *TaskPool) PickTask() Tasker {
	select {
	case t := <-tp.TaskCh:
		return t
	default:
		return nil
	}
}

/*
store 的三件套: get,set,delete
*/
func (tp *TaskPool) GetStoreTask(id string) (Tasker, bool) {
	tp.mu.RLock()
	defer tp.mu.RUnlock()

	t, ok := tp.TaskStore[id]

	return t, ok
}
func (tp *TaskPool) SetStoreTask(t Tasker) {
	tp.mu.Lock()
	defer tp.mu.Unlock()

	id := t.GetID()
	tp.TaskStore[id] = t
}

func (tp *TaskPool) DeleteStoreTask(id string) {
	tp.mu.Lock()
	defer tp.mu.Unlock()

	delete(tp.TaskStore, id)
}

/*----------------------------------*/

func (tp *TaskPool) PickAsyncCallback() Tasker {
	select {
	case t := <-tp.AsyncCh:
		return t
	default:
		return nil
	}
}

func (tp *TaskPool) PushAsyncCallback(t Tasker) error {
	select {
	case tp.AsyncCh <- t:
		return nil
	default:
		return &elf.FormatError{}
	}
}
