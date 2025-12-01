package step

import (
	"errors"
	"time"

	"workflow/logger"
	"workflow/record"
)

// Use logger.Logger from shared logger package
type Logger = logger.Logger

type Actioner interface {
	StepActor(ctx interface{}) error
	AsyncHandler(ctx interface{}, resp interface{}) error
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

func (s *Step) Run(ctx interface{}, rcder *record.Record, logger Logger) error {
	if rcder == nil {
		return errors.New("record is nil")
	}

	stepLogger := logger.With("step", s.name, "description", s.description, "isAsync", s.isAsync)
	stepLogger.Info("Starting step execution")

	var err error
	rcder.StartAt = time.Now().UnixMilli()
	rcder.Status = "processing"
	defer func() {
		rcder.EndAt = time.Now().UnixMilli()

		if err != nil {
			rcder.Status = "failed"
			stepLogger.Error("Step execution failed", "error", err)
		} else {
			if s.isAsync {
				rcder.Status = "async_waiting"
				stepLogger.Info("Step completed - waiting for async callback")
			} else {
				rcder.Status = "done"
				stepLogger.Info("Step completed successfully")
			}
		}
	}()

	// 超时处理 TODO
	stepLogger.Debug("Executing step actor")
	err = s.execute.StepActor(ctx)

	return err
}

// step 处理异步响应 (设置step给的自定义的函数)
func (s *Step) AsyncHandler(ctx interface{}, resp interface{}, runningID string, ids []int, stageIndex int, rcder *record.Record, logger Logger) {
	stepLogger := logger.With("step", s.name, "description", s.description, "runningID", runningID)
	stepLogger.Info("Handling async callback for step")

	if rcder == nil {
		stepLogger.Warn("AsyncHandler called with nil record")
		return
	}

	// 递归终止条件, step 是最终的执行单元,可以不用考虑递归继续嵌套
	if stageIndex >= len(ids) {
		stepLogger.Debug("AsyncHandler terminated - stage index out of bounds", "stageIndex", stageIndex, "idsLen", len(ids))
		return
	}

	if rcder.AsyncRecord == nil {
		rcder.AsyncRecord = record.NewRecord(rcder.ID, "-async", 0)
		stepLogger.Debug("Created async record")
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
			stepLogger.Error("Async handler failed", "error", err)
		} else {
			rcder.AsyncRecord.Status = "done"
			stepLogger.Info("Async handler completed successfully")
		}
	}()

	stepLogger.Debug("Executing async handler")
	err = s.execute.AsyncHandler(ctx, resp)
}

func (s *Step) StepsCount() int {
	return 0
}
