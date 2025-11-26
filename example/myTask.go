package example

import (
    "fmt"

    "workflow/step"
    "workflow/task"
)

type myAction struct{}

func (ma *myAction) StepActor() error {
    fmt.Println("Executing Step 1: get data from API")
    return nil
}

func (ma *myAction) DealAsyncResp(resp string) error {
    fmt.Println("Dealing with async response:", resp)
    return nil
}

func NewMyTask() *task.SerialTask {
    mytask := &task.SerialTask{
        Task: *task.NewTask("MyTask", ""),
    }

    myaction := &myAction{}
    step1 := step.NewStep("step1", "", myaction)
    mytask.AddStep(step1)

    step2 := step.NewStep("step2", "", myaction)
    mytask.AddStep(step2)

    step3 := step.NewStep("step3", "", myaction)
    mytask.AddStep(step3)

    return mytask
}
