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

// 支持编排多个 Step 的任务
// 串行，并行
// 同步，异步
// 任务structure的模型
// 一个任务由多个stage组成，每个stage包含多个step。 一个stage内的step并行执行，stage之间串行执行
// 如果step 是异步完成，那么需要设计等待机制，等待step完成后再进入下一个stage
type Task struct {
	Name  string
	ID    string
	Ctx   string      // TODO:
	Steps []step.Step // 数据结构TODO：优化
	State string
}

func NewTask(name, id string) *Task {

	task := &Task{
		Name:  name,
		ID:    id, // generate
		Steps: make([]step.Step, 0),
		State: "created",
	}

	return task
}

func (t *Task) TaskRun() error {
	return nil
}

/*
func (s *Task) waitCompletion(wg *sync.WaitGroup) {
	if wg != nil {
		wg.Wait()
	}
}
*/

// 异步step调用后，需要在stage这边注册一下，才能让stage挂起
func (t *Task) PostHook() error {
	//t.SetStatus(tSuccess)
	// 同步任务，直接success or failed
	// 异步任务，success 即进入挂起状态，等待
	//         failed 则同同步任务failed
	return nil
}

/*
func (t *Task) decodeID(id string) (string, string) {
	return "stage-index", "step-index"
}

// id: task-stageindex-stepindex
func (t *Task) SetStepCompleted(id string) {
	gindex, sindex := t.decodeID(id)
	step, ok := t.stepsMap[gindex][sindex]
	if !ok {
		return
	}
	step.SetCompleted()
}
*/

func (t *Task) Retry() error {
	return nil
}
func (t *Task) GetStatus() string {
	return t.State
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
func (t *Task) GetName() string {
	return t.Name
}
