package task

import (
	"sync"

	"workflow/step"
)

type ParallelTask struct {
	Task
	stepsMap map[string]step.Step // key: stepIndex
}

func (t *ParallelTask) TaskRun() error {
	var wg sync.WaitGroup
	for _, st := range t.Task.Steps {
		wg.Add(1)
		s := st
		go func(s step.Step) {
			defer wg.Done()
			s.Run()
		}(s)
	}
	wg.Wait()

	return nil
}

// 异步处理，也可能并发的来
// 任务状态不能直接更新
func (t *ParallelTask) AsyncHandler(resp string) {
	index := "" // 从resp中解析出stepIndex
	step := t.stepsMap[index]

	step.DealWithResp(resp)

	t.updateTaskStatus()
}

func (t *ParallelTask) updateTaskStatus() {
	// 遍历所有step，判断状态
}
