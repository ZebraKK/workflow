package task

type SerialTask struct {
	Task

	CurrentStage int // 当前执行到第几阶段

}

func (t *SerialTask) TaskRun() error {

	for index := t.CurrentStage; index < len(t.Steps); index++ {
		step := t.Steps[index]

		err := step.Run()
		if err != nil {
			return err
		}
		if step.IsAsyncWaiting() {
			t.CurrentStage = index
			t.State = "async_waiting"
			//t.AsyncRegister(t.ID) //? no need
			break
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
	t.TaskRun()
}
