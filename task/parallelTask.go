package task

import (
    "context"
    "errors"
    "sync"
    "time"

    "workflow/record"
    "workflow/step"
)

type ParallelTask struct {
    Task
    stepsMap map[string]*step.Step // key: stepIndex

    asyncSteps map[string]*step.Step // key: stepIndex 标记为异步的step 方便异步的检查

    maxConcurrentJobs int // 最大并发数
}

func NewParallelTask(name, id string) *ParallelTask {
    pt := &ParallelTask{
        Task:              *NewTask(name, id),
        stepsMap:          make(map[string]*step.Step),
        asyncSteps:        make(map[string]*step.Step),
        maxConcurrentJobs: 5, // 默认最大并发数5 TODO: 可配置
    }
    return pt
}

func (t *ParallelTask) Run(input string, rcder *record.Record) error {
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
    sem := make(chan struct{}, t.maxConcurrentJobs)
    for _, st := range t.Task.Steps {
        sem <- struct{}{}

        wg.Add(1)
        s := st
        if s.IsAsync {
            // 并发写入，需要封装函数
            t.asyncSteps[s.Name] = s
        }
        go func(s *step.Step) {
            var tErr error
            nextRecord := record.NewRecord(s.Name)
            nextRecord.Status = "processing"
            rcder.AppendRecord(nextRecord)

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
            case ch := <-t.worker(s, input, nextRecord):
                tErr = ch
            }

        }(s)
    }
    wg.Wait()

    // 更新任务状态
    // 异步任务 需要async_waiting
    // t.updateTaskStatus()

    if len(t.asyncSteps) > 0 && err == nil {
        rcder.Status = "async_waiting"
    } else if err == nil {
        rcder.Status = "done"
    } else {
        rcder.Status = "failed"
    }

    return err
}

func (t *ParallelTask) worker(s *step.Step, input string, rcder *record.Record) <-chan error {
    done := make(chan error, 1)
    // task 时长 TODO
    done <- s.Run(input, rcder)

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
