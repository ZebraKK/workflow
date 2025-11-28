package stage

import (
	"errors"
	"strconv"
	"time"

	"workflow/record"
)

// 有可能嵌套的
// 每一层都有超时设置
func (t *Stage) serialRun(ctx string, rcder *record.Record) error {
	if rcder == nil {
		return errors.New("record is nil")
	}

	rcder.Status = "processing"
	rcder.StartAt = time.Now().UnixMilli()
	defer func() {
		rcder.EndAt = time.Now().UnixMilli()
	}()

	for index := 0; index < len(t.Steps); index++ {
		stp := t.Steps[index]

		nextRecord := record.NewRecord(rcder.ID, strconv.Itoa(index), stp.StepsCount())
		nextRecord.Status = "processing"
		rcder.AddRecord(index, nextRecord)

		err := stp.Run(ctx, nextRecord)
		rcder.Status = nextRecord.Status
		if err != nil {
			return err
		}

		switch nextRecord.Status {
		case "failed", "async_waiting":
			return nil
		case "done":
			// continue
		default:
			rcder.Status = "failed"
			return errors.New("unknown step status: " + nextRecord.Status)
		}
	}

	// workflow 的AsyncRegister

	return nil
}

// 调用step异步的回调处理, 根据结果, 继续task的执行

// 非递归版本
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
	rcder.Status = "done"

	// 是否继续，是上一层来决定的事情。
	if rcder.Status == "done" && index != len(t.Steps)-1 {

		// todo: reuse Run() logic
		for i := index + 1; i < len(t.Steps); i++ {
			nextStep := t.Steps[i]
			nextRecord := record.NewRecord(rcder.ID, strconv.Itoa(i), nextStep.StepsCount())
			nextRecord.Status = "processing"
			rcder.AddRecord(i, nextRecord)

			err := nextStep.Run("", nextRecord)
			rcder.Status = nextRecord.Status
			if err != nil {
				return
			}

			switch nextRecord.Status {
			case "failed", "async_waiting":
				return
			case "done":
				// continue
			default:
				rcder.Status = "failed"
				return
			}
		}
	}
}

/*
// 递归版本 todo
func (t *SerialTask) AsyncHandler(resp string, stage int, runningID string, rcder *record.Record) {

	if rcder == nil {
		return
	}

	nextRcder := rcder.Records[0]
	if runningID == nextRcder.ID {

		t.Steps[0].AsyncHandler(resp)
		// update status


	} else {
		// 递归调用
		nextTask := NewSerialTask(t.Name, t.ID)
		nextTask.Steps = t.Steps[1:]
		nextTask.AsyncHandler(resp, stage-1, runningID, nextRcder)
	}

	// runningID format like as: xxx-0-1.1-2.3-3.2-4.1
	// 代表 taskid-stageindex.subindex-stageindex.subindex...
	ids := strings.Split(runningID, "-")

	// 可以递归(recursion)查找, 类似于Fibonacci heap
	// 下层的结果影响当前层的状态修改

}
*/
