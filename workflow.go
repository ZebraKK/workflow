package workflow

import (
	"sync"

	"workflow/taskpool"
)

type Workflow struct {
	taskPools *taskpool.TaskPool // 罗列 存储pipeline

	pipelineMap map[string]*Pipeline // 管理pipeline
	muPl        sync.RWMutex
	jobsStore   map[string]*Job // 每次运行一个pipeline 会生成一个job。 存储
	muJs        sync.RWMutex

	workerNum int
	JobCh     chan *Job //缓冲大小 = 峰值写入 QPS * 平均处理耗时（如峰值 1000 QPS,处理耗时 10ms,缓冲设为1000*0.01=10）；
	quitJobCh chan struct{}
	AsyncCh   chan *Job
}

func NewWorkflow(store taskpool.TaskStorer) *Workflow {
	wf := &Workflow{
		taskPools:   taskpool.NewTaskPool(store, 100),
		pipelineMap: make(map[string]*Pipeline),
		jobsStore:   make(map[string]*Job),
		workerNum:   5, // todo 配置化 or cpu*2
		quitJobCh:   make(chan struct{}, 1),
		JobCh:       make(chan *Job, 10),
		AsyncCh:     make(chan *Job, 10),
	}

	// load existing pipelines from store if needed
	//wf.pipelineMap = map[string]*Pipeline{}

	go wf.jobStart()

	return wf
}

func (w *Workflow) jobStart() {
	var wg sync.WaitGroup
	for range w.workerNum {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case job, ok := <-w.JobCh:
					if !ok {
						// log
						return
					}
					defer func() {
						if r := recover(); r != nil {
							// log the panic
						}
					}()
					w.runJob(job)
				case <-w.quitJobCh:
					// log
					return
				}
			}
		}()
	}

	wg.Wait()
}

func (w *Workflow) runJob(job *Job) {

	//运行模式确定，当前任务： 排队，覆盖，并行
	// ctx 参数值： parameter default or passed

	// 存储
	w.muJs.Lock()
	w.jobsStore[job.ID] = job
	w.muJs.Unlock()

	err := job.Pipeline.task.Run()
	if err != nil {
		//
	}
	// 中间状态记录？
	// update job 到db
	// job 后续继续运行处理,如异步等待,失败重试等

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

/*
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
*/
