package stage

import (
	"errors"
	"strconv"
	"time"

	"workflow/record"
)

// 有可能嵌套的
// 每一层都有超时设置
func (t *Stage) serialRun(ctx string, index int, rcder *record.Record) error {
	if rcder == nil {
		return errors.New("record is nil")
	}

	rcder.Status = "processing"
	rcder.StartAt = time.Now().UnixMilli()
	var err error
	defer func() {
		rcder.EndAt = time.Now().UnixMilli()
	}()

	for i := index; i < len(t.Steps); i++ {
		stp := t.Steps[i]

		nextRecord := record.NewRecord(rcder.ID, strconv.Itoa(i), stp.StepsCount())
		rcder.AddRecord(i, nextRecord)

		err_ := stp.Run(ctx, nextRecord)
		rcder.Status = nextRecord.Status
		if err_ != nil {
			err = err_
			break
		}

		switch nextRecord.Status {
		case "failed", "async_waiting":
			return err
		case "done":
			// continue
		default:
			rcder.Status = "failed"
			err = errors.New("unknown step status: " + nextRecord.Status)
			return err
		}
	}

	// workflow 的AsyncRegister

	return err
}

// 调用step异步的回调处理, 根据结果, 继续task的执行
// 状态回溯
func (t *Stage) serialAsyncHandler(resp string, runningID string, ids []int, stageIndex int, rcder *record.Record) {
	// 递归终止条件
	if rcder == nil || stageIndex >= len(ids) {
		return
	}

	index := ids[stageIndex]
	if index < 0 || index >= len(t.Steps) || index >= len(rcder.Records) {
		return
	}
	stp := t.Steps[index]

	nextRcrd := rcder.Records[index]
	stp.AsyncHandler(resp, runningID, ids, stageIndex+1, nextRcrd)

	// update current-level status
	for _, r := range rcder.Records {
		if r.Status != "done" { // todo: 可能还有其他状态
			rcder.Status = r.Status
			return
		}
	}

	if index < len(t.Steps)-1 { //serial
		// 继续执行后续步骤
		t.serialRun("", index+1, rcder)
	}
}
