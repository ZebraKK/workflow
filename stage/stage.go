package stage

import (
	"workflow/record"
)

type steper interface {
	IsAsync() bool
	Run(ctx string, rcder *record.Record) error
	AsyncHandler(resp string, runningID string, ids []int, stageIndex int, rcder *record.Record)
	StepsCount() int
}

type Stage struct {
	Name    string
	ID      string
	Ctx     string   // TODO:
	Mode    string   // serial / parallel
	Steps   []steper // 数据结构TODO：优化
	isAsync bool
}

func NewStage(name, id, mode string, stp steper) *Stage {
	if stp == nil {
		return nil
	}

	stage := &Stage{
		Name:  name,
		Mode:  mode,
		Steps: make([]steper, 0),
	}
	if id != "" {
		stage.ID = id
	} else {
		stage.ID = name // todo: 生成唯一id
	}

	stage.AddStep(stp)

	return stage
}

// 需要预设好。不支持动态添加。如果开始运行了，就不支持添加了
func (s *Stage) AddStep(stp steper) {
	s.Steps = append(s.Steps, stp)
	if stp.IsAsync() {
		s.isAsync = true
	}
}

func (s *Stage) IsAsync() bool {
	return s.isAsync
}

func (s *Stage) StepsCount() int {
	return len(s.Steps)
}

func (s *Stage) GetName() string {
	return s.Name
}

func (s *Stage) GetID() string {
	return s.ID
}

func (s *Stage) Run(ctx string, rcder *record.Record) error {
	switch s.Mode {
	case "serial":
		return s.serialRun(ctx, rcder)
	case "parallel":
		return s.parallelRun(ctx, rcder)
	default:
		return nil
	}
}

func (s *Stage) AsyncHandler(resp string, runningID string, ids []int, stageIndex int, rcder *record.Record) {
	switch s.Mode {
	case "serial":
		s.serialAsyncHandler(resp, runningID, ids, stageIndex, rcder)
	case "parallel":
		s.parallelAsyncHandler(resp, runningID, ids, stageIndex, rcder)
	}
}

/*

type TaskState int

const (
    tCreated TaskState = iota
    tProcessing
    tSuccess
    tFailed
    tRetry
)

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
