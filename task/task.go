package task

import (
	"workflow/record"
)

/*
type TaskState int

const (
    tCreated TaskState = iota
    tProcessing
    tSuccess
    tFailed
    tRetry
)
*/

type steper interface {
	IsAsync() bool
	Run(ctx string, rcder *record.Record) error
	AsyncHandler(resp string, runningID string, ids []int, stageIndex int, rcder *record.Record)
	StepsCount() int
}

type Task struct {
	Name    string
	ID      string
	Ctx     string   // TODO:
	Steps   []steper // 数据结构TODO：优化
	isAsync bool
}

func NewTask(name, id string, stp steper) *Task {
	if stp == nil {
		return nil
	}

	task := &Task{
		Name:  name,
		Steps: make([]steper, 0),
	}
	if id != "" {
		task.ID = id
	} else {
		task.ID = name // todo: 生成唯一id
	}

	task.AddStep(stp)

	return task
}

// 需要预设好。不支持动态添加。如果开始运行了，就不支持添加了
func (t *Task) AddStep(s steper) {
	t.Steps = append(t.Steps, s)
	if s.IsAsync() {
		t.isAsync = true
	}
}

func (t *Task) IsAsync() bool {
	return t.isAsync
}

func (t *Task) StepsCount() int {
	return len(t.Steps)
}

func (t *Task) GetName() string {
	return t.Name
}

func (t *Task) GetID() string {
	return t.ID
}

func (t *Task) UpdateAsyncResp(resp string) {
}

/*
   func (t *Task) SetStatus(newState TaskState) error {
       t.mu.Lock()
       defer t.mu.Unlock()

       switch t.State {
       case tCreated:
           if newState == tProcessing {
               t.State = tProcessing
               return nil
           }
       case tProcessing:
           switch newState {
           case tSuccess:
               t.State = tSuccess
               return nil
           case tFailed:
               t.State = tFailed
               return nil
           }
       case tFailed:
           if newState == tRetry {
               t.State = tRetry
               return nil
           }
       case tRetry:
           if newState == tProcessing {
               t.State = tProcessing
               return nil
           }
       }

       return nil
   }
*/
