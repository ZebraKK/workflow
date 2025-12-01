package stage

import (
	"errors"
	"strconv"
	"time"

	"workflow/record"
)

// 有可能嵌套的
// 每一层都有超时设置
func (t *Stage) serialRun(ctx interface{}, index int, rcder *record.Record, logger Logger) error {
	if rcder == nil {
		return errors.New("record is nil")
	}

	stageLogger := logger.With("stage", t.Name, "stageID", t.ID, "mode", "serial")
	stageLogger.Info("Starting serial stage execution")

	rcder.Status = "processing"
	rcder.StartAt = time.Now().UnixMilli()
	var err error
	defer func() {
		rcder.EndAt = time.Now().UnixMilli()
		stageLogger.Info("Serial stage completed", "status", rcder.Status)
	}()

	for i := index; i < len(t.Steps); i++ {
		stp := t.Steps[i]
		stepLogger := stageLogger.With("stepIndex", i, "stepCount", stp.StepsCount())

		nextRecord := record.NewRecord(rcder.ID, strconv.Itoa(i), stp.StepsCount())
		rcder.AddRecord(i, nextRecord)

		stepLogger.Debug("Executing step")
		err_ := stp.Run(ctx, nextRecord, stepLogger)
		rcder.Status = nextRecord.Status
		if err_ != nil {
			err = err_
			stepLogger.Error("Step execution failed", "error", err_)
			break
		}

		switch nextRecord.Status {
		case "failed", "async_waiting":
			stepLogger.Info("Step ended with special status", "status", nextRecord.Status)
			return err
		case "done":
			stepLogger.Debug("Step completed successfully")
			// continue
		default:
			rcder.Status = "failed"
			err = errors.New("unknown step status: " + nextRecord.Status)
			stepLogger.Error("Unknown step status", "status", nextRecord.Status)
			return err
		}
	}

	// workflow 的AsyncRegister

	return err
}

// 调用step异步的回调处理, 根据结果, 继续task的执行
// 状态回溯
func (t *Stage) serialAsyncHandler(ctx interface{}, resp interface{}, runningID string, ids []int, stageIndex int, rcder *record.Record, logger Logger) {
	stageLogger := logger.With("stage", t.Name, "stageID", t.ID, "mode", "serial", "runningID", runningID)
	stageLogger.Info("Handling async callback for serial stage")

	// 递归终止条件
	if rcder == nil || stageIndex >= len(ids) {
		stageLogger.Debug("Async handler terminated early", "rcderNil", rcder == nil, "stageIndex", stageIndex, "idsLen", len(ids))
		return
	}

	index := ids[stageIndex]
	if index < 0 || index >= len(t.Steps) || index >= len(rcder.Records) {
		stageLogger.Warn("Invalid index in async handler", "index", index, "stepsLen", len(t.Steps), "recordsLen", len(rcder.Records))
		return
	}
	stp := t.Steps[index]
	stepLogger := stageLogger.With("stepIndex", index)

	nextRcrd := rcder.Records[index]
	stepLogger.Debug("Calling step async handler")
	stp.AsyncHandler(ctx, resp, runningID, ids, stageIndex+1, nextRcrd, stepLogger)

	// update current-level status
	for _, r := range rcder.Records {
		if r.Status != "done" { // todo: 可能还有其他状态
			rcder.Status = r.Status
			stageLogger.Debug("Stage status updated from record", "status", r.Status)
			return
		}
	}

	if index < len(t.Steps)-1 { //serial
		// 继续执行后续步骤
		stageLogger.Info("Continuing with next steps", "nextIndex", index+1)
		t.serialRun(ctx, index+1, rcder, logger)
	}
}
