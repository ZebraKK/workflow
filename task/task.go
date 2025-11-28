package task

import (
	"workflow/step"
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

type Task struct {
	Name  string
	ID    string
	Ctx   string       // TODO:
	Steps []*step.Step // 数据结构TODO：优化
}

func NewTask(name, id string, stp *step.Step) *Task {
	if stp == nil {
		return nil
	}

	task := &Task{
		Name:  name,
		Steps: make([]*step.Step, 0),
	}
	if id != "" {
		task.ID = id
	} else {
		task.ID = name // todo: 生成唯一id
	}

	task.AddStep(stp)

	return task
}

func (t *Task) AddStep(s *step.Step) {
	//indexStr := strconv.Itoa(len(t.Steps)) // 生成 step index 字符串
	// 串行, 并行的ID 生成规则不一样 todo

	//s.SetID(t.ID + "-" + indexStr)

	t.Steps = append(t.Steps, s)
}

func (t *Task) GetStatus() string {
	return ""
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
