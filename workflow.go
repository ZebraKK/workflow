package workflow

import (
	"strconv"
	"strings"
	"sync"

	"workflow/logger"
)

// Use logger.Logger from shared logger package
type Logger = logger.Logger

// Re-export logger constructors for convenience
var (
	NewSlogLogger = logger.NewSlogLogger
	NewTextLogger = logger.NewTextLogger
	NewNoOpLogger = logger.NewNoOpLogger
)

type WorkflowConfig struct {
	WorkerNum   int
	JobChSize   int
	AsyncChSize int
}

type Workflow struct {
	pipelineMap         map[string]*Pipeline // id -> pipeline
	pipelineMapWithName map[string]string    // name -> id
	muPl                sync.RWMutex
	jobsStore           map[string]*Job // 每次运行一个pipeline 会生成一个job。 存储
	muJs                sync.RWMutex

	workerNum   int
	JobCh       chan *Job //缓冲大小 = 峰值写入 QPS * 平均处理耗时（如峰值 1000 QPS,处理耗时 10ms,缓冲设为1000*0.01=10）；
	quitJobCh   chan struct{}
	quitAsyncCh chan struct{}
	AsyncCh     chan *AsyncJob
	logger      Logger
}

func NewWorkflow(logger Logger, cfg WorkflowConfig) *Workflow {
	if logger == nil {
		logger = NewNoOpLogger()
	}
	if cfg.WorkerNum <= 0 {
		cfg.WorkerNum = 5
	}
	if cfg.JobChSize <= 0 {
		cfg.JobChSize = 10
	}
	if cfg.AsyncChSize <= 0 {
		cfg.AsyncChSize = 10
	}

	wf := &Workflow{
		pipelineMap:         make(map[string]*Pipeline),
		pipelineMapWithName: make(map[string]string),
		jobsStore:           make(map[string]*Job),
		workerNum:           cfg.WorkerNum,
		quitJobCh:           make(chan struct{}, 1),
		quitAsyncCh:         make(chan struct{}, 1),
		JobCh:               make(chan *Job, cfg.JobChSize),
		AsyncCh:             make(chan *AsyncJob, cfg.AsyncChSize),
		logger:              logger,
	}

	// load existing pipelines from store if needed
	//wf.pipelineMap = map[string]*Pipeline{}

	logger.Info("Starting workflow", "workerNum", wf.workerNum)
	go wf.jobStart()
	go wf.asyncJobStart()

	return wf
}

func (w *Workflow) jobStart() {
	var wg sync.WaitGroup
	for i := range w.workerNum {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			w.logger.Debug("Job worker started", "workerID", workerID)
			defer w.logger.Debug("Job worker stopped", "workerID", workerID)

			for {
				select {
				case job, ok := <-w.JobCh:
					if !ok {
						w.logger.Info("Job channel closed, worker exiting", "workerID", workerID)
						return
					}
					func() {
						defer func() {
							if r := recover(); r != nil {
								w.logger.Error("Panic in job execution",
									"workerID", workerID,
									"jobID", job.ID,
									"error", r)
							}
						}()
						w.runJob(job)
					}()
				case <-w.quitJobCh:
					w.logger.Info("Quit signal received, worker exiting", "workerID", workerID)
					return
				}
			}
		}(i)
	}

	wg.Wait()
	w.logger.Info("All job workers stopped")
}

func (w *Workflow) runJob(job *Job) {
	jobLogger := w.logger.With("jobID", job.ID, "pipeline", job.Pipeline.Name)
	jobLogger.Info("Starting job execution")

	//运行模式确定，当前任务： 排队，覆盖，并行
	// ctx 参数值： parameter default or passed

	// 存储
	w.muJs.Lock()
	w.jobsStore[job.ID] = job
	w.muJs.Unlock()
	jobLogger.Debug("Job added to store")

	err := job.Pipeline.task.Run(job.ctx, job.record, jobLogger)
	if err != nil {
		jobLogger.Error("Job execution failed", "error", err)
		job.record.Status = "failed"
	}
	// 中间状态记录？
	// update job 到db

	// job 后续继续运行处理,如异步等待,失败重试等
	status := job.record.Status
	jobLogger.Info("Job completed", "status", status)

	switch status {
	case "async_waiting":
		// 加入到异步等待任务记录中去即可。或者db化
		// 不做任何操作，等待回调处理
		jobLogger.Debug("Job waiting for async callback")
	case "completed", "done":
		// 清理任务
		// 从任务池中删除
		w.muJs.Lock()
		delete(w.jobsStore, job.ID)
		w.muJs.Unlock()
		jobLogger.Info("Job completed and removed from store")
	case "failed":
		// 失败处理，告警等
		jobLogger.Error("Job failed", "recordStatus", status)
		w.muJs.Lock()
		delete(w.jobsStore, job.ID)
		w.muJs.Unlock()
	default:
		// 其他状态处理
		jobLogger.Warn("Job ended with unknown status", "status", status)
	}
}

func (w *Workflow) asyncJobStart() {
	var wg sync.WaitGroup
	for i := range w.workerNum {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			w.logger.Debug("Async worker started", "workerID", workerID)
			defer w.logger.Debug("Async worker stopped", "workerID", workerID)

			for {
				select {
				case job, ok := <-w.AsyncCh:
					if !ok {
						w.logger.Info("Async channel closed, worker exiting", "workerID", workerID)
						return
					}
					func() {
						defer func() {
							if r := recover(); r != nil {
								w.logger.Error("Panic in async job execution",
									"workerID", workerID,
									"jobID", job.Job.ID,
									"runningID", job.RunningID,
									"error", r)
							}
						}()
						w.runAsyncJob(job)
					}()
				case <-w.quitAsyncCh:
					w.logger.Info("Quit signal received, async worker exiting", "workerID", workerID)
					return
				}
			}
		}(i)
	}

	wg.Wait()
	w.logger.Info("All async workers stopped")
}

func parseStageByRunningID(runningID string) []int {
	parts := strings.Split(runningID, "-")

	indices := make([]int, 0, len(parts)-1)
	for _, part := range parts[1:] {
		if idx, err := strconv.Atoi(part); err == nil {
			indices = append(indices, idx)
		}
	}
	return indices
}

func (w *Workflow) runAsyncJob(asyncJob *AsyncJob) {
	asyncLogger := w.logger.With(
		"jobID", asyncJob.Job.ID,
		"runningID", asyncJob.RunningID,
		"pipeline", asyncJob.Job.Pipeline.Name)
	asyncLogger.Info("Processing async job callback")

	ids := parseStageByRunningID(asyncJob.RunningID)
	stageIndex := 0 // 从第0个开始,递归调用
	asyncLogger.Debug("Parsed running ID", "ids", ids, "stageIndex", stageIndex)

	asyncJob.Job.Pipeline.task.AsyncHandler(asyncJob.Job.ctx, asyncJob.Resp, asyncJob.RunningID, ids, stageIndex, asyncJob.Job.record, asyncLogger)

	// 运行结果决定后续job的处理, 放到retry,done,wait等队列
	state := asyncJob.Job.record.Status
	asyncLogger.Info("Async callback processed", "newStatus", state)

	switch state {
	case "async_waiting":
		// 加入到异步等待任务记录中去即可。或者db化
		// 不做任何操作，等待回调处理
		asyncLogger.Debug("Job still waiting for async callback")
	case "completed", "done":
		if len(ids) < asyncJob.Job.Pipeline.task.StepsCount() {
			// 还没处理完，继续放回等待
			select {
			case w.AsyncCh <- asyncJob: // 放回jobchannel/ todo
				asyncLogger.Debug("Async job re-queued for further processing")
			default:
				asyncLogger.Error("Failed to re-queue async job - channel full")
			}
		} else {
			// 任务处理完毕，清理
			w.muJs.Lock()
			delete(w.jobsStore, asyncJob.Job.ID)
			w.muJs.Unlock()
			asyncLogger.Info("Async job completed and removed from store")
		}

	case "failed":
		// 失败处理，告警等
		asyncLogger.Error("Async job failed")
		w.muJs.Lock()
		delete(w.jobsStore, asyncJob.Job.ID)
		w.muJs.Unlock()

	default:
		// 其他状态处理
		asyncLogger.Warn("Async job ended with unknown status", "status", state)
	}
}

func (w *Workflow) Close() {
	w.logger.Info("Shutting down workflow")

	// 中断正在运行的任务
	// Signal all workers to stop
	close(w.quitJobCh)
	close(w.quitAsyncCh)
	w.logger.Debug("Quit signal sent to all workers")

	// 释放资源
	// Close input channels to prevent new jobs
	close(w.JobCh)
	close(w.AsyncCh)
	w.logger.Debug("Job and async channels closed")

	// Wait for workers to finish is handled by WaitGroup in jobStart/asyncJobStart
	// 其他清理等
	w.logger.Info("Workflow shutdown complete")
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
