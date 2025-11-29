package workflow

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"math/rand"
	"strconv"
	"strings"
	"time"

	"workflow/record"
)

var GlobalHash = sha256.New()
var GlobalRand = rand.New(rand.NewSource(time.Now().UnixNano()))

type Tasker interface {
	Run(ctx string, rcder *record.Record) error
	AsyncHandler(resp string, runningID string, ids []int, stageIndex int, rcder *record.Record)
	StepsCount() int
}

type Pipeline struct {
	Name        string
	ID          string
	task        Tasker
	defaultCtx  string
	runningMode string // serial / parallel or queue // lasted or every
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

func generateID(value string) string {
	rdNum := GlobalRand.Int63()
	value = value + strconv.FormatInt(rdNum, 10) + time.Now().String()
	GlobalHash.Write([]byte(value))
	hash := GlobalHash.Sum(nil)

	id := hex.EncodeToString(hash)
	return id[:32]
}

func (w *Workflow) CreatePipeline(name string, t Tasker) error {
	if name == "" {
		w.logger.Error("CreatePipeline failed: empty pipeline name")
		return errors.New("pipeline name cannot be empty")
	}

	// check if pipeline with same ID exists
	w.muPl.RLock()
	_, exists := w.pipelineMap[name]
	w.muPl.RUnlock()
	if exists {
		w.logger.Warn("CreatePipeline failed: duplicate pipeline name", "name", name)
		return errors.New("duplicate pipeline")
	}

	pl := &Pipeline{
		Name:        name,
		ID:          generateID(name),
		task:        t,
		defaultCtx:  "",
		runningMode: "serial",
	}
	w.muPl.Lock()
	defer w.muPl.Unlock()
	w.pipelineMap[pl.ID] = pl

	w.logger.Info("Pipeline created", "name", name, "id", pl.ID)
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
	pl, exists := w.pipelineMap[id]
	w.muPl.RUnlock()
	if !exists {
		w.logger.Warn("DeletePipeline failed: pipeline not found", "id", id)
		return errors.New("pipeline not found")
	}

	w.muPl.Lock()
	defer w.muPl.Unlock()
	delete(w.pipelineMap, id)

	w.logger.Info("Pipeline deleted", "name", pl.Name, "id", id)
	return nil
}

func (w *Workflow) UpdatePipeline(id string, t Tasker) error {
	w.muPl.RLock()
	pl, exists := w.pipelineMap[id]
	w.muPl.RUnlock()
	if !exists {
		w.logger.Warn("UpdatePipeline failed: pipeline not found", "id", id)
		return errors.New("pipeline not found")
	}

	w.muPl.Lock()
	defer w.muPl.Unlock()
	w.pipelineMap[id].task = t

	w.logger.Info("Pipeline updated", "name", pl.Name, "id", id)
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
		w.logger.Warn("LaunchPipeline failed: pipeline not found", "id", id)
		return errors.New("pipeline not found")
	}

	plInstance := *pl
	job := &Job{
		ID:          generateID(pl.ID),
		Pipeline:    plInstance,
		ctx:         ctx,
		description: "Job for pipeline " + id,
	}
	job.record = record.NewRecord(job.ID, "", plInstance.task.StepsCount())

	w.logger.Info("Launching pipeline", "pipeline", pl.Name, "jobID", job.ID)

	select {
	case w.JobCh <- job:
		w.logger.Debug("Job queued successfully", "jobID", job.ID)
		return nil
	default:
		w.logger.Error("LaunchPipeline failed: job channel is full",
			"pipeline", pl.Name, "jobID", job.ID)
		return errors.New("job channel is full")
	}

}

// 管理回调, 解析任务, 调用任务的AsyncHandler
// 在并发的环境下调用
func (w *Workflow) CallbackHandler(id string, resp string) error {
	if id == "" {
		w.logger.Error("CallbackHandler failed: invalid id")
		return errors.New("invalid id")
	}

	// jobID 找到mgr 里的job对象
	// recordID 找到 record 中的step

	jobID := strings.Split(id, "-")[0]
	w.muJs.RLock()
	job, exists := w.jobsStore[jobID]
	w.muJs.RUnlock()
	if !exists {
		w.logger.Warn("CallbackHandler failed: job not found",
			"callbackID", id, "jobID", jobID)
		return errors.New("job not found")
	}

	asyncJob := &AsyncJob{
		Job:       job,
		Resp:      resp,
		RunningID: id,
	}

	w.logger.Info("Callback received", "callbackID", id, "jobID", jobID)

	select {
	case w.AsyncCh <- asyncJob:
		w.logger.Debug("Async callback queued", "callbackID", id)
		return nil
	case <-time.After(time.Second * 5):
		w.logger.Error("CallbackHandler failed: async channel full or timeout",
			"callbackID", id)
		return errors.New("async channel full or timeout")
	}
}
