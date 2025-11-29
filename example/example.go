package example

import (
	"log/slog"
	"workflow"
)

func main() {
	// Create a text logger for more readable output
	logger := workflow.NewTextLogger(slog.LevelInfo)
	wf := workflow.NewWorkflow(logger)

	mytask := NewMyTask()
	wf.CreatePipeline("mytask", mytask)
}
