package example

import (
	"log/slog"

	"workflow"
	"workflow/logger"
)

func main() {
	// Create a text logger for more readable output
	logger := logger.NewTextLogger(slog.LevelInfo)
	wf := workflow.NewWorkflow(logger, workflow.WorkflowConfig{})

	mytask := NewMyTask()
	wf.CreatePipeline("mytask", mytask)
}
