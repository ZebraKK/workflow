package example

import "workflow"

func main() {
	store := NewMyStore()
	wf := workflow.NewWorkflow(store)

	// 创建一个串行任务
	serialTask := task.NewSerialTask("task1")
	// 添加步骤到串行任务
	// serialTask.AddStep(step1)
	// serialTask.AddStep(step2)
	// ...

	// 将任务推送到工作流的任务池
	wf.taskPools.PushTask(serialTask)

	// 运行工作流（通常在后台运行）
	// wf.Start() // 假设有一个Start方法来启动工作流处理
}
