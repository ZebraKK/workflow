package task

import (
    "errors"
    "time"
    "workflow/record"
)

type SerialTask struct {
    Task

    CurrentStage int // 当前执行到第几阶段

}

func NewSerialTask(name, id string) *SerialTask {
    st := &SerialTask{
        Task:         *NewTask(name, id),
        CurrentStage: 0,
    }
    return st
}

func (t *SerialTask) Run(ctx string, rcder *record.Record) error {
    if rcder == nil {
        return errors.New("record is nil")
    }

    rcder.Status = "processing"
    rcder.StartAt = time.Now().UnixMilli()
    defer func() {
        rcder.EndAt = time.Now().UnixMilli()
    }()

    for index := t.CurrentStage; index < len(t.Steps); index++ {
        step := t.Steps[index]

        nextRecord := record.NewRecord(step.Name)
        nextRecord.Status = "processing"
        rcder.AppendRecord(nextRecord)

        err := step.Run(ctx, nextRecord)
        if err != nil {
            return err
        }

        rcder.Status = nextRecord.Status
        switch nextRecord.Status {
        case "failed", "async_waiting":
            return nil
        case "done":
            // continue
        default:
            rcder.Status = "failed"
            return errors.New("unknown step status: " + nextRecord.Status)
        }

        t.CurrentStage++
    }

    // workflow 的AsyncRegister

    return nil
}

// 调用step异步的回调处理, 根据结果, 继续task的执行
func (t *SerialTask) AsyncHandler(resp string) {
    step := t.Steps[t.CurrentStage]
    step.DealWithResp(resp)

    t.CurrentStage++
    t.State = "processing"
    t.Run("", nil) // 返回池子？
}

func (t *SerialTask) RunNext() {
    if t.CurrentStage >= len(t.Steps) {
        t.State = "completed"
        return
    }

    t.CurrentStage++
    t.State = "processing"
    t.Run("", nil)

}
