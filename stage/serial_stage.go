package stage

import (
	"errors"
	"strconv"
	"time"

	"workflow/record"
)

// 有可能嵌套的
// 每一层都有超时设置
func (t *Stage) serialHandle(ctx interface{}, index int, rcder *record.Record, logger Logger) error {
	if rcder == nil {
		return errors.New("record is nil")
	}

	stageLogger := logger.With("stage", t.Name, "stageID", t.ID, "mode", "serial")
	stageLogger.Info("Starting serial stage execution")

	rcder.SetStatus(record.StatusProcessing)
	rcder.SetStartAt(time.Now().UnixMilli())
	var err error
	defer func() {
		rcder.SetEndAt(time.Now().UnixMilli())
		stageLogger.Info("Serial stage completed", "status", rcder.GetStatus())
	}()
	defer func() {
		if err != nil {
			rcder.SetStatus(record.StatusFailed)
			stageLogger.Error("Serial stage execution failed", "error", err)
		}
	}()

	for i := index; i < len(t.Steps); i++ {
		stp := t.Steps[i]
		stepLogger := stageLogger.With("stepIndex", i, "stepCount", stp.StepsCount())

		nextRecord := record.NewRecord(rcder.ID, strconv.Itoa(i), stp.StepsCount())
		rcder.AddRecord(i, nextRecord)

		stepLogger.Debug("Executing step")
		err_ := stp.Handle(ctx, nextRecord, stepLogger)
		rcder.SetStatus(nextRecord.GetStatus())
		if err_ != nil {
			err = err_
			stepLogger.Error("Step execution failed", "error", err_)
			break
		}

		nextStatus := nextRecord.GetStatus()
		switch nextStatus {
		case record.StatusFailed, record.StatusAsyncWaiting:
			stepLogger.Info("Step ended with special status", "status", nextStatus)
			return err
		case record.StatusDone:
			stepLogger.Debug("Step completed successfully")
			// continue
		default:
			rcder.SetStatus(record.StatusFailed)
			err = errors.New("unknown step status: " + nextStatus)
			stepLogger.Error("Unknown step status", "status", nextStatus)
			return err
		}
	}

	// workflow 的AsyncRegister

	return err
}

// 调用step异步的回调处理, 根据结果, 继续task的执行
// 状态回溯
func (t *Stage) serialAsyncHandle(ctx interface{}, resp interface{}, runningID string, ids []int, stageIndex int, rcder *record.Record, logger Logger) {
	stageLogger := logger.With("stage", t.Name, "stageID", t.ID, "mode", "serial", "runningID", runningID)
	stageLogger.Info("Handling async callback for serial stage")

	// 递归终止条件
	if rcder == nil || stageIndex >= len(ids) {
		stageLogger.Debug("Async handler terminated early", "rcderNil", rcder == nil, "stageIndex", stageIndex, "idsLen", len(ids))
		return
	}

	index := ids[stageIndex]
	if index < 0 || index >= len(t.Steps) {
		stageLogger.Warn("Invalid index in async handler", "index", index, "stepsLen", len(t.Steps))
		return
	}
	stp := t.Steps[index]
	stepLogger := stageLogger.With("stepIndex", index)

	nextRcrd := rcder.GetRecord(index)
	if nextRcrd == nil {
		stageLogger.Warn("Record not found at index", "index", index)
		return
	}
	stepLogger.Debug("Calling step async handler")
	stp.AsyncHandle(ctx, resp, runningID, ids, stageIndex+1, nextRcrd, stepLogger)

	// update current-level status
	for i := 0; i < len(t.Steps); i++ {
		r := rcder.GetRecord(i)
		if r != nil && r.GetStatus() != record.StatusDone {
			rcder.SetStatus(r.GetStatus())
			stageLogger.Debug("Stage status updated from record", "status", r.GetStatus())
			return
		}
	}

	if index < len(t.Steps)-1 { //serial
		// 继续执行后续步骤。
		// TODO(C5): 此处在 async worker goroutine 上内联执行同步阶段，会占用 async pool。
		// 根本修复需要将续接逻辑通过 JobCh 回流到 job worker pool，
		// 要求变更 AsyncHandle 接口签名或引入续接回调，待后续 Phase 重构。
		stageLogger.Info("Continuing with next steps", "nextIndex", index+1)
		t.serialHandle(ctx, index+1, rcder, logger)
	}
}
