package workflow

import (
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"workflow/logger"
	"workflow/record"
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
	jobsStore           map[string]*Job // 运行中/async_waiting 的 job（内存索引）
	muJs                sync.RWMutex

	workerNum int
	JobCh     chan *Job
	AsyncCh   chan *AsyncJob
	isClosed  atomic.Bool // 关闭标志，防止 Close() 后继续投递
	closeOnce sync.Once   // 保证 Close() 幂等
	store     Store       // 可选，nil = 纯内存模式
	logger    Logger
	jobWg     sync.WaitGroup
	asyncWg   sync.WaitGroup
}

// NewWorkflow 创建并启动 Workflow。
// store 可选：传 nil 则以纯内存模式运行（重启后 job 不可恢复）；
// 传实现了 Store 接口的后端，则在关键节点自动持久化，并在启动时恢复 async_waiting 的 job。
func NewWorkflow(logger Logger, cfg WorkflowConfig, store Store) *Workflow {
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
		JobCh:               make(chan *Job, cfg.JobChSize),
		AsyncCh:             make(chan *AsyncJob, cfg.AsyncChSize),
		store:               store,
		logger:              logger,
	}

	logger.Info("Starting workflow", "workerNum", wf.workerNum)
	// 同步调用：jobStart/asyncJobStart 仅做 Add+go，立即返回。
	// 不用 go 包装，保证所有 jobWg.Add(1) 在 NewWorkflow 返回前完成，
	// 避免 Close() 的 Wait() 与尚未执行的 Add(1) 形成 race。
	wf.jobStart()
	wf.asyncJobStart()

	// 从 store 恢复上次运行未完成的 async_waiting job（store=nil 时跳过）。
	// 注意：需在 CreatePipeline 注册完所有 pipeline 后再调用 NewWorkflow，
	// 否则找不到对应 pipeline 的 job 会被跳过。
	wf.recoverPendingJobs()

	return wf
}

func (w *Workflow) jobStart() {
	for i := range w.workerNum {
		w.jobWg.Add(1)
		go func(workerID int) {
			defer w.jobWg.Done()
			w.logger.Debug("Job worker started", "workerID", workerID)
			defer w.logger.Debug("Job worker stopped", "workerID", workerID)

			// for range 在 channel 关闭且排空后自然退出，保证 Close() 时不丢弃已入队 Job。
			for job := range w.JobCh {
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
			}
			w.logger.Info("Job channel drained, worker exiting", "workerID", workerID)
		}(i)
	}
}

func (w *Workflow) runJob(job *Job) {
	jobLogger := w.logger.With("jobID", job.ID, "pipeline", job.Pipeline.Name)
	jobLogger.Info("Starting job execution")

	w.muJs.Lock()
	w.jobsStore[job.ID] = job
	w.muJs.Unlock()

	// 持久化：创建时保存（created 状态）
	if w.store != nil {
		if jr, err := newJobRecord(job); err == nil {
			if err = w.store.Save(jr); err != nil {
				jobLogger.Error("Store.Save failed on job start", "error", err)
			}
		}
	}

	err := job.Pipeline.task.Handle(job.ctx, job.record, jobLogger)
	if err != nil {
		jobLogger.Error("Job execution failed", "error", err)
		job.record.SetStatus(record.StatusFailed)
	}

	status := job.record.GetStatus()
	jobLogger.Info("Job completed", "status", status)

	switch status {
	case record.StatusAsyncWaiting:
		// goroutine 暂时挂起，持久化当前 Record 快照以便恢复
		if w.store != nil {
			if jr, err := newJobRecord(job); err == nil {
				if err = w.store.Save(jr); err != nil {
					jobLogger.Error("Store.Save failed on async_waiting", "error", err)
				}
			}
		}
		jobLogger.Debug("Job waiting for async callback")
	case record.StatusDone:
		w.muJs.Lock()
		delete(w.jobsStore, job.ID)
		w.muJs.Unlock()
		if w.store != nil {
			if err := w.store.Delete(job.ID); err != nil {
				jobLogger.Error("Store.Delete failed on done", "error", err)
			}
		}
		jobLogger.Info("Job completed and removed from store")
	case record.StatusFailed:
		jobLogger.Error("Job failed", "recordStatus", status)
		w.muJs.Lock()
		delete(w.jobsStore, job.ID)
		w.muJs.Unlock()
		if w.store != nil {
			if err := w.store.Delete(job.ID); err != nil {
				jobLogger.Error("Store.Delete failed on failed", "error", err)
			}
		}
	default:
		jobLogger.Warn("Job ended with unknown status", "status", status)
	}
}

func (w *Workflow) asyncJobStart() {
	for i := range w.workerNum {
		w.asyncWg.Add(1)
		go func(workerID int) {
			defer w.asyncWg.Done()
			w.logger.Debug("Async worker started", "workerID", workerID)
			defer w.logger.Debug("Async worker stopped", "workerID", workerID)

			for job := range w.AsyncCh {
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
			}
			w.logger.Info("Async channel drained, worker exiting", "workerID", workerID)
		}(i)
	}
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

	asyncJob.Job.Pipeline.task.AsyncHandle(asyncJob.Job.ctx, asyncJob.Resp, asyncJob.RunningID, ids, stageIndex, asyncJob.Job.record, asyncLogger)

	// 运行结果决定后续job的处理, 放到retry,done,wait等队列
	state := asyncJob.Job.record.GetStatus()
	asyncLogger.Info("Async callback processed", "newStatus", state)

	switch state {
	case record.StatusAsyncWaiting:
		// 还有后续异步步骤，更新持久化快照
		if w.store != nil {
			if jr, err := newJobRecord(asyncJob.Job); err == nil {
				if err = w.store.Save(jr); err != nil {
					asyncLogger.Error("Store.Save failed on async_waiting", "error", err)
				}
			}
		}
		asyncLogger.Debug("Job still waiting for async callback")
	case record.StatusDone:
		// 整个 job 已完成，清理
		w.muJs.Lock()
		delete(w.jobsStore, asyncJob.Job.ID)
		w.muJs.Unlock()
		if w.store != nil {
			if err := w.store.Delete(asyncJob.Job.ID); err != nil {
				asyncLogger.Error("Store.Delete failed on done", "error", err)
			}
		}
		asyncLogger.Info("Async job completed and removed from store")
	case record.StatusFailed:
		asyncLogger.Error("Async job failed")
		w.muJs.Lock()
		delete(w.jobsStore, asyncJob.Job.ID)
		w.muJs.Unlock()
		if w.store != nil {
			if err := w.store.Delete(asyncJob.Job.ID); err != nil {
				asyncLogger.Error("Store.Delete failed on failed", "error", err)
			}
		}
	default:
		asyncLogger.Warn("Async job ended with unknown status", "status", state)
	}
}

// recoverPendingJobs 在启动时从 store 加载 async_waiting 的 job 回内存，
// 等待外部 CallbackHandler 续接执行。
// Pipeline 找不到的 job 会打 Warn 日志后跳过（需确保 CreatePipeline 在 NewWorkflow 之前调用）。
func (w *Workflow) recoverPendingJobs() {
	if w.store == nil {
		return
	}
	jobs, err := w.store.ListByStatus(record.StatusAsyncWaiting)
	if err != nil {
		w.logger.Error("recoverPendingJobs: store.ListByStatus failed", "error", err)
		return
	}
	w.logger.Info("Recovering pending jobs", "count", len(jobs))
	for _, jr := range jobs {
		w.muPl.RLock()
		pl, ok := w.pipelineMap[jr.PipelineID]
		w.muPl.RUnlock()
		if !ok {
			w.logger.Warn("recoverPendingJobs: pipeline not found, skipping",
				"jobID", jr.JobID, "pipelineID", jr.PipelineID)
			continue
		}
		job, err := restoreJob(jr, pl)
		if err != nil {
			w.logger.Error("recoverPendingJobs: restoreJob failed",
				"jobID", jr.JobID, "error", err)
			continue
		}
		w.muJs.Lock()
		w.jobsStore[job.ID] = job
		w.muJs.Unlock()
		w.logger.Info("Job recovered", "jobID", job.ID, "pipeline", job.Pipeline.Name)
	}
}

func (w *Workflow) Close() {
	w.closeOnce.Do(func() {
		w.logger.Info("Shutting down workflow")

		// 标记关闭，阻止 LaunchPipeline / CallbackHandler 继续投递。
		w.isClosed.Store(true)

		// 关闭 JobCh：已入队的 Job 由 worker 排空后自然退出（for range 语义）。
		close(w.JobCh)
		w.logger.Debug("JobCh closed, waiting for job workers to drain")
		w.jobWg.Wait()
		w.logger.Info("All job workers stopped")

		// Job workers 全部退出后，不会再有新 AsyncJob 产生，安全关闭 AsyncCh。
		close(w.AsyncCh)
		w.logger.Debug("AsyncCh closed, waiting for async workers to drain")
		w.asyncWg.Wait()
		w.logger.Info("All async workers stopped")

		w.logger.Info("Workflow shutdown complete")
	})
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
