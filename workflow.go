package workflow

import (
	"sync"
)

type Tasker interface {
	AsyncHandler(resp string)
	TaskRun() error
}

type Workflow struct {
	Tasks map[string]Tasker

	AsyncTask map[string]struct{}

	mu sync.Mutex
}

func NewWorkflow() *Workflow {
	return &Workflow{
		Tasks: make(map[string]Tasker),
	}
}

// 管理回调, 解析任务, 调用任务的AsyncHandler
func (w *Workflow) CallbackHandler(id string) {
	// 解析id，找到对应的wg，调用wg.Done()
	// id格式： taskID-stageIndex-stepIndex
	taskID := "" // 从外部获取taskID
	resp := ""
	task, ok := w.Tasks[taskID]
	if !ok {
		return
	}
	task.AsyncHandler(resp)
}

func (w *Workflow) AsyncRegister(id string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.AsyncTask[id] = struct{}{}
}
