package step

import (
	"context"
	"errors"
	"fmt"
	"time"

	"workflow/logger"
	"workflow/record"
)

type Logger = logger.Logger

type Actioner interface {
	Handle(ctx interface{}) error
}

type AsyncActioner interface {
	AsyncHandle(ctx interface{}, resp interface{}) error
}

// 最小执行单元
type Step struct {
	name         string
	description  string
	timeout      time.Duration
	execute      Actioner
	asyncExecute AsyncActioner
}

func NewStep(name, description string, timeout time.Duration, actor Actioner, asyncActor AsyncActioner) *Step {
	if actor == nil {
		fmt.Println("Error: Actioner cannot be nil")
		return nil
	}
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return &Step{
		name:         name,
		description:  description,
		timeout:      timeout,
		execute:      actor,
		asyncExecute: asyncActor,
	}
}

func (s *Step) SetTimeout(timeout time.Duration) {
	if timeout > 0 {
		s.timeout = timeout
	}
}

func (s *Step) IsAsync() bool {
	return s.asyncExecute != nil
}

func (s *Step) Handle(ctx interface{}, rcder *record.Record, logger Logger) error {
	if rcder == nil {
		return errors.New("record is nil")
	}

	stepLogger := logger.With("step", s.name, "description", s.description, "isAsync", s.IsAsync())
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
			if s.IsAsync() {
				rcder.Status = "async_waiting"
				stepLogger.Info("Step completed - waiting for async callback")
			} else {
				rcder.Status = "done"
				stepLogger.Info("Step completed successfully")
			}
		}
	}()

	stepLogger.Debug("Executing step actor with timeout", "timeout", s.timeout)
	err = s.executeWithTimeout(ctx, stepLogger)

	return err
}

// step 处理异步响应 (设置step给的自定义的函数)
func (s *Step) AsyncHandle(ctx interface{}, resp interface{}, runningID string, ids []int, stageIndex int, rcder *record.Record, logger Logger) {
	stepLogger := logger.With("step", s.name, "description", s.description, "runningID", runningID)
	stepLogger.Info("Handling async callback for step")

	if rcder == nil {
		stepLogger.Warn("AsyncHandler called with nil record")
		return
	}

	// 递归终止条件, step 是最终的执行单元,可以不用考虑递归继续嵌套
	// stageIndex 需要减1 判断，step是最后一步，属于当前stage, stageIndex加1 是为了下一个stage准备的
	if stageIndex-1 >= len(ids) {
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

	stepLogger.Debug("Executing async handler with timeout", "timeout", s.timeout)
	err = s.executeAsyncWithTimeout(ctx, resp, stepLogger)

}

func (s *Step) StepsCount() int {
	return 0
}

func (s *Step) executeWithTimeout(ctx interface{}, logger Logger) error {
	timeoutCtx, cancel := context.WithTimeout(context.Background(), s.timeout)
	defer cancel()

	errChan := make(chan error, 1)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				errChan <- fmt.Errorf("panic in step actor: %v", r)
			}
		}()
		start := time.Now()
		errChan <- s.execute.Handle(ctx)
		duration := time.Since(start)
		logger.Debug("Step actor execution time", "duration", duration)
	}()

	select {
	case err := <-errChan:
		return err
	case <-timeoutCtx.Done():
		logger.Error("Step execution timeout", "timeout", s.timeout, "step", s.name)
		return fmt.Errorf("step execution timeout after %v", s.timeout)
	}
}

func (s *Step) executeAsyncWithTimeout(ctx interface{}, resp interface{}, logger Logger) error {
	timeoutCtx, cancel := context.WithTimeout(context.Background(), s.timeout)
	defer cancel()

	errChan := make(chan error, 1)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				errChan <- fmt.Errorf("panic in async handler: %v", r)
			}
		}()
		start := time.Now()
		errChan <- s.asyncExecute.AsyncHandle(ctx, resp)
		duration := time.Since(start)
		logger.Debug("Async handler execution time", "duration", duration)
	}()

	select {
	case err := <-errChan:
		return err
	case <-timeoutCtx.Done():
		logger.Error("Async handler execution timeout", "timeout", s.timeout, "step", s.name)
		return fmt.Errorf("async handler execution timeout after %v", s.timeout)
	}
}
