package stage

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"time"

	"workflow/record"
)

const maxConcurrentJobs = 5 // 默认最大并发数5 TODO

func (s *Stage) parallelRun(input interface{}, rcder *record.Record, logger Logger) error {
	if rcder == nil {
		return errors.New("record is nil")
	}

	stageLogger := logger.With("stage", s.Name, "stageID", s.ID, "mode", "parallel")
	stageLogger.Info("Starting parallel stage execution", "stepCount", len(s.Steps))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	rcder.Status = "processing"
	rcder.StartAt = time.Now().UnixMilli()
	defer func() {
		rcder.EndAt = time.Now().UnixMilli()
		stageLogger.Info("Parallel stage completed", "status", rcder.Status)
	}()

	var wg sync.WaitGroup
	var mu sync.Mutex
	var err error
	sem := make(chan struct{}, maxConcurrentJobs)
	for i, st := range s.Steps {
		sem <- struct{}{}

		wg.Add(1)
		stp := st
		stepIndex := i

		go func(stp steper, idx int) {
			var tErr error
			stepLogger := stageLogger.With("stepIndex", idx, "stepCount", stp.StepsCount())
			nextRecord := record.NewRecord(rcder.ID, strconv.Itoa(idx), stp.StepsCount())
			nextRecord.Status = "processing"
			rcder.AddRecord(idx, nextRecord)

			defer func() {
				<-sem
			}()
			defer func() {
				wg.Done()
			}()

			defer func() {
				if tErr != nil {
					mu.Lock()
					if err == nil { // 只记录第一个错误
						err = tErr
						cancel() // 取消其他正在运行的步骤
						stepLogger.Error("Step failed, cancelling other steps", "error", tErr)
					}
					mu.Unlock()
				}
			}()

			select {
			case <-ctx.Done():
				tErr = ctx.Err()
				stepLogger.Info("Step cancelled due to context", "error", tErr)
			case ch := <-s.worker(stp, input, nextRecord, stepLogger):
				tErr = ch
				if tErr != nil {
					stepLogger.Error("Step execution failed", "error", tErr)
				} else {
					stepLogger.Debug("Step completed successfully")
				}
			}

		}(stp, stepIndex)
	}
	wg.Wait()

	// 更新任务状态
	// 异步任务 需要async_waiting
	// t.updateTaskStatus()

	if s.IsAsync() && err == nil {
		rcder.Status = "async_waiting"
		stageLogger.Info("Stage waiting for async callbacks")
	} else if err == nil {
		rcder.Status = "done"
	} else {
		rcder.Status = "failed"
	}

	return err
}

func (s *Stage) worker(stp steper, input interface{}, rcder *record.Record, logger Logger) <-chan error {
	done := make(chan error, 1)
	// task 时长 TODO
	done <- stp.Run(input, rcder, logger)

	return done
}

// 异步处理，也可能并发的来
// 任务状态不能直接更新
func (s *Stage) parallelAsyncHandler(ctx interface{}, resp interface{}, runningID string, ids []int, stageIndex int, rcder *record.Record, logger Logger) {
	stageLogger := logger.With("stage", s.Name, "stageID", s.ID, "mode", "parallel", "runningID", runningID)
	stageLogger.Info("Handling async callback for parallel stage")

	// status processing 需要等待么？

	// 递归终止条件
	if rcder == nil || stageIndex >= len(ids) {
		stageLogger.Debug("Async handler terminated early", "rcderNil", rcder == nil, "stageIndex", stageIndex, "idsLen", len(ids))
		return
	}

	index := ids[stageIndex]
	if index < 0 || index >= len(s.Steps) || index >= len(rcder.Records) {
		stageLogger.Warn("Invalid index in async handler", "index", index, "stepsLen", len(s.Steps), "recordsLen", len(rcder.Records))
		return
	}
	stp := s.Steps[index]
	stepLogger := stageLogger.With("stepIndex", index)

	nextRcrd := rcder.Records[index]
	stepLogger.Debug("Calling step async handler")
	stp.AsyncHandler(ctx, resp, runningID, ids, stageIndex+1, nextRcrd, stepLogger)

	// 并发的情况: 某一个任务的异步响应来了，之前的任务还没完成。还在running状态 to-defenses
	// update current-level status
	for _, r := range rcder.Records {
		if r.Status != "done" { // todo: 可能还有其他状态
			rcder.Status = r.Status
			stageLogger.Debug("Stage status updated from record", "status", r.Status)
			return
		}
	}

	stageLogger.Info("All parallel steps completed")
	// continue to next steps if all done
	// 并发的情况下，不继续执行后续步骤，由外层控制
}
