package example

import (
	"workflow"
)

func main() {
	store := NewMyStore()
	wf := workflow.NewWorkflow(store)

	mytask := NewMyTask()
	wf.Handler(mytask)
}
