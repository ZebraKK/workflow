package task

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

func (t *SerialTask) Run() error {

	for index := t.CurrentStage; index < len(t.Steps); index++ {
		step := t.Steps[index]

		err := step.Run()
		if err != nil {
			return err
		}
		// todo: 改进为是否继续, 有单独逻辑
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
	t.Run() // 返回池子？
}

func (t *SerialTask) RunNext() {
	if t.CurrentStage >= len(t.Steps) {
		t.State = "completed"
		return
	}

	t.CurrentStage++
	t.State = "processing"
	t.Run()

}
