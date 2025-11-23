package workflowMgr

type Pipeline struct {
	Name string
	ID string
	task taskpool.Tasker
}

type Job struct {
	ID string
	PipelineID string
	description string
}

func (w *Workflow) CreatePipeline(t taskpool.Tasker) error {

	return w.taskPools.PushTask(t)
}

func (w *Workflow) UpdatePipeline(t taskpool.Tasker) error {

	w.taskPools.SetStoreTask(t)

	return nil
}

// input: pipeline id/name
// output: job id
func (w *Workflow) RunPipeline(id string) (string, error) {

}