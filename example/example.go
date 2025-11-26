package example

import (
    "workflow"
)

func main() {
    wf := workflow.NewWorkflow()

    mytask := NewMyTask()
    wf.CreatePipeline(mytask)
}
