package step

import (
    "errors"
    "time"
    "workflow/record"
)

type Actioner interface {
    StepActor() error
    DealAsyncResp(resp string) error
}

// 最小执行单元
type Step struct {
    Description string
    Name        string
    IsAsync     bool
    execute     Actioner
}

func NewStep(name, description string, actor Actioner) *Step {
    if actor == nil {
        return nil
    }
    return &Step{
        Name:        name,
        Description: description,
        execute:     actor,
    }
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
            if s.IsAsync {
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
func (s *Step) DealWithResp(resp string) {

    s.execute.DealAsyncResp(resp)
}
