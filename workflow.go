package workflow

import (
	"time"

	"workflow/taskpool"
)

type Workflow struct {
	taskPools *taskpool.TaskPool
}

func NewWorkflow(store taskpool.TaskStorer) *Workflow {
	wf := &Workflow{
		taskPools: taskpool.NewTaskPool(store, 100),
	}
	wf.start()

	return wf
}

func (w *Workflow) start() {
	go func() {
		for {
			// 1 从任务channel取任务
			task := w.taskPools.PickTask()
			// 2 goroutine运行任务
			if task == nil {
				time.Sleep(30 * time.Second)
				continue
			} else {
				go func() {
					w.runTask(task)
					if task.GetStatus() == "done" {
						w.taskPools.DeleteStoreTask(task.GetID())
					}
				}()
			}
		}
	}()

	go func() {
		// 监听回调channel
		for {
			task := w.taskPools.PickAsyncCallback()
			resp := "" // 从外部获取response

			if task == nil {
				time.Sleep(10 * time.Second)
				continue
			} else {
				go func() {
					task.AsyncHandler(resp) // todo ,resp被改写到task里了 // 执行拿到异步结果后的处理

					if task.GetStatus() == "not_done" {
						w.taskPools.PushTask(task)
					} else {
						w.taskPools.DeleteStoreTask(task.GetID())
					}
				}()
			}

		}
	}()
}

func (w *Workflow) runTask(t taskpool.Tasker) {
	t.Run()
	state := "" // t.GetState() // 获取任务状态
	switch state {
	case "async_waiting":
		// 加入到异步等待任务记录中去即可。或者db化
		// 不做任何操作，等待回调处理
	case "completed":
		// 清理任务
		// 从任务池中删除
		w.taskPools.DeleteStoreTask(t.GetID())
	case "failed":
		// 失败处理，告警等

	default:
		// 其他状态处理

	}
}

// 管理回调, 解析任务, 调用任务的AsyncHandler
// 在并发的环境下调用
func (w *Workflow) CallbackHandler(id string) {
	// 解析id，找到对应的wg，调用wg.Done()
	// id格式： taskID-stageIndex-stepIndex
	taskID := "" // 从外部获取taskID
	resp := ""
	task, ok := w.taskPools.GetStoreTask(taskID)
	if !ok {
		return
	}
	task.UpdateAsyncResp(resp)
	w.taskPools.PushAsyncCallback(task)
}



func (w *Workflow) Close() {
	// 中断正在运行的任务
	// 释放资源
	// 关闭channel
	// 其他清理等
}


/*
	定义
	workflow
		调度管理平台
	pipeline
		workflow 调度运行的对象
		必须包含一个task 对象
	task
		task 和pipeline 有重叠的理解
		task 可以是task再嵌套
		包括并行任务，串行任务

	job
		workflow运行一个pipeline，即一次job的执行



	workflow调度
		服务自管理
		无状态，支持水平扩展
		负载情况，（统计，max， running， total）

	workflow管理 
		接口交互
		pipeline, 代表一个任务序列
			执行一次，有一次job description





	接口: pipeline
		新建
		运行
		修改
		查询
		clone
		trigger （context ）

		重试
		终止
		跳过
		回滚 // 


	--------------------------------------------
	任务
		串行
		并行
		cron
		on demand
		任务通知（pre，post）

*/
