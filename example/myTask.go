package example

import (
	"fmt"
	"time"

	"workflow/stage"
	"workflow/step"
)

type myAction struct{}

func (ma *myAction) Handle(ctx interface{}) error {
	// You can now use ctx to access custom parameters from the upper layer
	// Example: if you pass a map[string]interface{} or a custom struct
	fmt.Println("Executing Step 1: get data from API")
	if ctx != nil {
		fmt.Printf("Context received: %+v\n", ctx)
	}
	return nil
}

func (ma *myAction) AsyncHandle(ctx interface{}, resp interface{}) error {
	fmt.Println("Dealing with async response:", resp)
	if ctx != nil {
		fmt.Printf("Context in async handler: %+v\n", ctx)
	}
	return nil
}

func NewMyTask() *stage.Stage {
	myaction := &myAction{}

	step0 := step.NewStep("step0", "First step without timeout", 0, myaction, nil)
	mytask := stage.NewStage("myTask", "myTaskID", "serial", step0)

	step1 := step.NewStep("step1", "Second step with 5s timeout", 5*time.Second, myaction, myaction)
	mytask.AddStep(step1)

	step2 := step.NewStep("step2", "Third step, set timeout later", 0, myaction, myaction)
	step2.SetTimeout(10 * time.Second)
	mytask.AddStep(step2)

	step3 := step.NewStep("step3", "Fourth step with 15s timeout", 15*time.Second, myaction, myaction)

	mytask.AddStep(step3)

	return mytask
}
