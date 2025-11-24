package workflow

import (
    "errors"
    "workflow/taskpool"
)

type Pipeline struct {
    Name        string
    ID          string
    task        taskpool.Tasker
    defaultCtx  string
    runningMode string // serial / parallel or others // lasted or every
}

type Job struct {
    ID          string
    Pipeline  Pipeline
    description string
    ctx         string
    record      *Record
}

func (w *Workflow) CreatePipeline(t taskpool.Tasker) error {
    // check if pipeline with same ID exists
    w.muPl.RLock()
    _, exists := w.pipelineMap[t.GetID()]
    w.muPl.RUnlock()
    if exists {
        return errors.New("duplicate pipeline")
    }

    pl := &Pipeline{
        Name:        t.GetStatus(), // todo
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

func (w *Workflow) UpdatePipeline(id string, t taskpool.Tasker) error {
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

    select {
    case w.JobCh <- job:
        return nil
    default:
        return errors.New("job channel is full")
    }

}

// 管理回调, 解析任务, 调用任务的AsyncHandler
// 在并发的环境下调用
func (w *Workflow) CallbackHandler(jobId string, resp string) {
    // 解析id，找到对应的wg，调用wg.Done()
    // id格式： taskID-stageIndex-stepIndex
    taskID := "" // 从外部获取taskID
    task, ok := w.taskPools.GetStoreTask(taskID)
    if !ok {
        return
    }
    task.UpdateAsyncResp(resp)
    w.taskPools.PushAsyncCallback(task)
}
