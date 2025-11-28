package step

import (
	"errors"
	"time"
	"workflow/record"
)

type Actioner interface {
	StepActor() error
	AsyncHandler(resp string) error
}

// 最小执行单元
type Step struct {
	description string
	name        string
	isAsync     bool
	execute     Actioner
}

func NewStep(name, description string, actor Actioner) *Step {
	if actor == nil {
		return nil
	}
	return &Step{
		name:        name,
		description: description,
		execute:     actor,
	}
}

func (s *Step) IsAsync() bool {
	return s.isAsync
}

func (s *Step) Run(ctx string, rcder *record.Record) error {
	if rcder == nil {
		return errors.New("record is nil")
	}

	var err error
	rcder.StartAt = time.Now().UnixMilli()
	defer func() {
		rcder.EndAt = time.Now().UnixMilli()

		if err != nil {
			rcder.Status = "failed"
		} else {
			if s.isAsync {
				rcder.Status = "async_waiting"
			} else {
				rcder.Status = "done"
			}
		}
	}()

	// 超时处理 TODO
	err = s.execute.StepActor()

	return err
}

// step 处理异步响应 (设置step给的自定义的函数)
func (s *Step) AsyncHandler(resp string, runningID string, ids []int, stageIndex int, rcder *record.Record) {
	if rcder == nil {
		return
	}

	// 递归终止条件, step 是最终的执行单元,可以不用考虑递归继续嵌套
	if stageIndex >= len(ids) {
		return
	}

	if rcder.AsyncRecord == nil {
		rcder.AsyncRecord = record.NewRecord(rcder.ID, "-async", 0)
	}
	rcder.AsyncRecord.StartAt = time.Now().UnixMilli()
	defer func() {
		rcder.AsyncRecord.EndAt = time.Now().UnixMilli()
	}()

	var err error
	defer func() {
		// update record status
		if err != nil {
			rcder.AsyncRecord.Status = "failed"
		} else {
			rcder.AsyncRecord.Status = "done"
		}
	}()

	err = s.execute.AsyncHandler(resp)
}

func (s *Step) StepsCount() int {
	return 0
}
