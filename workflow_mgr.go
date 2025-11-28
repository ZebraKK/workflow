package workflow

import (
	"errors"
	"strings"
	"time"

	"workflow/record"
)

type Tasker interface {
	GetID() string
	Run(ctx string, rcder *record.Record) error
	AsyncHandler(resp string, runningID string, ids []int, stageIndex int, rcder *record.Record)
	StepsCount() int
}

type Pipeline struct {
	Name        string
	ID          string
	task        Tasker
	defaultCtx  string
	runningMode string // serial / parallel or others // lasted or every
}

type Job struct {
	ID          string
	Pipeline    Pipeline
	description string
	ctx         string
	record      *record.Record
}

type AsyncJob struct {
	Job       *Job
	Resp      string
	RunningID string
}

func (w *Workflow) CreatePipeline(t Tasker) error {
	// check if pipeline with same ID exists
	w.muPl.RLock()
	_, exists := w.pipelineMap[t.GetID()]
	w.muPl.RUnlock()
	if exists {
		return errors.New("duplicate pipeline")
	}

	pl := &Pipeline{
		Name:        "pipeline-name", // todo
		ID:          t.GetID(),
		task:        t,
		defaultCtx:  "",
		runningMode: "serial",
	}
	w.muPl.Lock()
	defer w.muPl.Unlock()
	w.pipelineMap[pl.ID] = pl
	return nil
}

func (w *Workflow) GetPipeline(id string) (*Pipeline, bool) {
	w.muPl.RLock()
	defer w.muPl.RUnlock()

	pl, ok := w.pipelineMap[id]
	return pl, ok
}

func (w *Workflow) DeletePipeline(id string) error {
	w.muPl.RLock()
	_, exists := w.pipelineMap[id]
	w.muPl.RUnlock()
	if !exists {
		return errors.New("pipeline not found")
	}

	w.muPl.Lock()
	defer w.muPl.Unlock()
	delete(w.pipelineMap, id)
	return nil
}

func (w *Workflow) UpdatePipeline(id string, t Tasker) error {
	w.muPl.RLock()
	_, exists := w.pipelineMap[id]
	w.muPl.RUnlock()
	if !exists {
		return errors.New("pipeline not found")
	}

	w.muPl.Lock()
	defer w.muPl.Unlock()
	w.pipelineMap[id].task = t

	return nil
}

func (w *Workflow) ListPipelines() []*Pipeline {
	w.muPl.RLock()
	defer w.muPl.RUnlock()

	pipelines := make([]*Pipeline, 0, len(w.pipelineMap))
	for _, pl := range w.pipelineMap {
		pipelines = append(pipelines, pl)
	}
	return pipelines
}

func (w *Workflow) LaunchPipeline(id string, ctx string) error {
	w.muPl.RLock()
	pl, exists := w.pipelineMap[id]
	w.muPl.RUnlock()
	if !exists {
		return errors.New("pipeline not found")
	}

	plInstance := *pl
	job := &Job{
		ID:          id + "-job", // todo: unique id
		Pipeline:    plInstance,
		ctx:         ctx,
		description: "Job for pipeline " + id,
	}
	job.record = record.NewRecord(job.ID, "", plInstance.task.StepsCount())

	select {
	case w.JobCh <- job:
		return nil
	default:
		return errors.New("job channel is full")
	}

}

// 管理回调, 解析任务, 调用任务的AsyncHandler
// 在并发的环境下调用
func (w *Workflow) CallbackHandler(id string, resp string) error {
	if id == "" {
		return errors.New("invalid id")
	}

	// jobID 找到mgr 里的job对象
	// recordID 找到 record 中的step

	jobID := strings.Split(id, "-")[0]
	w.muJs.RLock()
	job, exists := w.jobsStore[jobID]
	w.muJs.RUnlock()
	if !exists {
		return errors.New("pipeline not found")
	}

	asyncJob := &AsyncJob{
		Job:       job,
		Resp:      resp,
		RunningID: id,
	}

	select {
	case w.AsyncCh <- asyncJob:
		return nil
	case <-time.After(time.Second * 5):
		// handle full channel if needed
	}
	return nil
}
