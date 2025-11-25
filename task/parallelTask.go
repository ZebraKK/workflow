package task

import (
	"context"
	"sync"

	"workflow/step"
)

type ParallelTask struct {
	Task
	stepsMap map[string]*step.Step // key: stepIndex

	asyncSteps map[string]*step.Step // key: stepIndex 标记为异步的step 方便异步的检查
}

func NewParallelTask(name, id string) *ParallelTask {
	pt := &ParallelTask{
		Task:       *NewTask(name, id),
		stepsMap:   make(map[string]*step.Step),
		asyncSteps: make(map[string]*step.Step),
	}
	return pt
}

func (t *ParallelTask) Run() error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup
	var mu sync.Mutex
	var err error
	for _, st := range t.Task.Steps {
		wg.Add(1)
		s := st
		if s.IsAsync {
			// 并发写入，需要封装函数
			t.asyncSteps[s.ID] = s
		}
		go func(s *step.Step) {
			defer wg.Done()

			var tErr error
			select {
			case <-ctx.Done():
				tErr = ctx.Err()
			case ch := <-t.worker(s):
				tErr = ch
			}

			if tErr != nil {
				mu.Lock()
				if err == nil { // 只记录第一个错误
					err = tErr
					cancel() // 取消其他正在运行的步骤
				}
				mu.Unlock()
			}
		}(s)
	}
	wg.Wait()

	// 更新任务状态
	// 异步任务 需要async_waiting
	// t.updateTaskStatus()

	if len(t.asyncSteps) > 0 && err == nil {
		t.State = "async_waiting"
	} else if err == nil {
		t.State = "done"
	} else {
		t.State = "failed"
	}

	return err
}

func (t *ParallelTask) worker(s *step.Step) <-chan error {
	done := make(chan error, 1)
	// task 时长 TODO
	done <- s.Run()

	return done
}

// 异步处理，也可能并发的来
// 任务状态不能直接更新
func (t *ParallelTask) AsyncHandler(resp string) {
	index := "" // 从resp中解析出stepIndex
	step := t.stepsMap[index]

	step.DealWithResp(resp)

	t.updateTaskStatus()
	// 都是并行的， 那么某个step 的异步响应来了。后续？
	// 下一步任务
}

func (t *ParallelTask) RunNext() {
}

func (t *ParallelTask) updateTaskStatus() {
	// 遍历所有step，判断状态
}
