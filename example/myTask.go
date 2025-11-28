package example

import (
	"fmt"

	"workflow/stage"
	"workflow/step"
)

type myAction struct{}

func (ma *myAction) StepActor() error {
	fmt.Println("Executing Step 1: get data from API")
	return nil
}

func (ma *myAction) AsyncHandler(resp string) error {
	fmt.Println("Dealing with async response:", resp)
	return nil
}

func NewMyTask() *stage.Stage {
	myaction := &myAction{}
	step0 := step.NewStep("step0", "", myaction)
	mytask := stage.NewStage("myTask", "myTaskID", "serial", step0)

	step1 := step.NewStep("step1", "", myaction)
	mytask.AddStep(step1)

	step2 := step.NewStep("step2", "", myaction)
	mytask.AddStep(step2)

	step3 := step.NewStep("step3", "", myaction)
	mytask.AddStep(step3)

	return mytask
}
