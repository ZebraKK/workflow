package stage

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"time"

	"workflow/record"
)

const maxConcurrentJobs = 5 // 默认最大并发数5 TODO: system config

func (s *Stage) parallelRun(input string, rcder *record.Record) error {
	if rcder == nil {
		return errors.New("record is nil")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	rcder.Status = "processing"
	rcder.StartAt = time.Now().UnixMilli()
	defer func() {
		rcder.EndAt = time.Now().UnixMilli()
	}()

	var wg sync.WaitGroup
	var mu sync.Mutex
	var err error
	sem := make(chan struct{}, maxConcurrentJobs)
	for i, st := range s.Steps {
		sem <- struct{}{}

		wg.Add(1)
		stp := st

		go func(stp steper) {
			var tErr error
			nextRecord := record.NewRecord(rcder.ID, strconv.Itoa(i), stp.StepsCount())
			nextRecord.Status = "processing"
			rcder.AddRecord(i, nextRecord)

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
					}
					mu.Unlock()
				}
			}()

			select {
			case <-ctx.Done():
				tErr = ctx.Err()
			case ch := <-s.worker(stp, input, nextRecord):
				tErr = ch
			}

		}(stp)
	}
	wg.Wait()

	// 更新任务状态
	// 异步任务 需要async_waiting
	// t.updateTaskStatus()

	if s.IsAsync() && err == nil {
		rcder.Status = "async_waiting"
	} else if err == nil {
		rcder.Status = "done"
	} else {
		rcder.Status = "failed"
	}

	return err
}

func (s *Stage) worker(stp steper, input string, rcder *record.Record) <-chan error {
	done := make(chan error, 1)
	// task 时长 TODO
	done <- stp.Run(input, rcder)

	return done
}

// 异步处理，也可能并发的来
// 任务状态不能直接更新
func (s *Stage) parallelAsyncHandler(resp string, runningID string, ids []int, stageIndex int, rcder *record.Record) {
	// 递归终止条件
	if rcder == nil || stageIndex >= len(ids) {
		return
	}

	index := ids[stageIndex]
	if index < 0 || index >= len(s.Steps) || index >= len(rcder.Records) {
		return
	}
	stp := s.Steps[index]

	nextRcrd := rcder.Records[index]
	stp.AsyncHandler(resp, runningID, ids, stageIndex+1, nextRcrd)

	// 并发的情况: 某一个任务的异步响应来了，之前的任务还没完成。还在running状态 to-defenses
	// update current-level status
	for _, r := range rcder.Records {
		if r.Status != "done" { // todo: 可能还有其他状态
			rcder.Status = r.Status
			return
		}
	}

	// continue to next steps if all done

}
