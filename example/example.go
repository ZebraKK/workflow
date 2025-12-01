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

	// Example 1: Launch pipeline with a simple string context
	// wf.LaunchPipeline("pipeline-id", "simple string context")

	// Example 2: Launch pipeline with a custom struct context
	// type MyContext struct {
	// 	UserID    string
	// 	RequestID string
	// 	Data      map[string]interface{}
	// }
	// ctx := &MyContext{
	// 	UserID:    "user123",
	// 	RequestID: "req456",
	// 	Data:      map[string]interface{}{"key": "value"},
	// }
	// wf.LaunchPipeline("pipeline-id", ctx)

	// Example 3: Launch pipeline with a map context
	// ctx := map[string]interface{}{
	// 	"userID": "<USER_NAME>",
	// 	"params": map[string]string{"action": "process"},
	// }
	// wf.LaunchPipeline("pipeline-id", ctx)

	// Example 4: Launch pipeline with nil context (if no parameters needed)
	// wf.LaunchPipeline("pipeline-id", nil)
}
