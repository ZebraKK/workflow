# Step Timeout Control Guide

## Overview

The workflow engine now supports **timeout control** for step execution. When a step's `Handle()` or `AsyncHandler()` takes longer than the configured timeout, the execution will be automatically terminated and marked as failed.

## Key Features

### 1. **Configurable Timeout**
- Each step can have its own timeout duration
- Default is no timeout (0 duration)
- Timeout can be set at creation or later

### 2. **Automatic Failure Handling**
- When timeout occurs, the step is automatically marked as failed
- Timeout error is logged with details
- Execution is safely terminated

### 3. **Panic Recovery**
- The timeout mechanism also catches and handles panics in step execution
- Panics are converted to errors and logged

## API Reference

### Creating Steps with Timeout

#### Method 1: NewStepWithTimeout
```go
func NewStepWithTimeout(name, description string, actor Actioner, timeout time.Duration) *Step
```

Create a step with timeout configured at initialization:
```go
step := step.NewStepWithTimeout("api-call", "Call external API", action, 5*time.Second)
```

#### Method 2: SetTimeout
```go
func (s *Step) SetTimeout(timeout time.Duration)
```

Create a step first, then set timeout:
```go
step := step.NewStep("api-call", "Call external API", action)
step.SetTimeout(5 * time.Second)
```

#### Method 3: NewStep (No Timeout)
```go
func NewStep(name, description string, actor Actioner) *Step
```

Create a step without timeout:
```go
step := step.NewStep("quick-task", "Fast operation", action)
// No timeout set, will run until completion
```

## Usage Examples

### Example 1: Basic Timeout Configuration

```go
package main

import (
    "fmt"
    "time"
    "workflow/step"
)

type MyAction struct{}

func (a *MyAction) Handle(ctx interface{}) error {
    // Simulate a long-running operation
    time.Sleep(10 * time.Second)
    fmt.Println("Task completed")
    return nil
}

func (a *MyAction) AsyncHandler(ctx interface{}, resp interface{}) error {
    fmt.Println("Async response received")
    return nil
}

func main() {
    action := &MyAction{}
    
    // This step will timeout after 3 seconds
    step1 := step.NewStepWithTimeout(
        "slow-task",
        "A task that takes too long",
        action,
        3*time.Second,
    )
    
    // When executed, this step will fail with timeout error:
    // "step execution timeout after 3s"
}
```

### Example 2: Different Timeouts for Different Steps

```go
func CreateWorkflowSteps() []*step.Step {
    action := &MyAction{}
    
    steps := []*step.Step{
        // Quick validation - 1 second timeout
        step.NewStepWithTimeout("validate", "Validate input", action, 1*time.Second),
        
        // API call - 5 seconds timeout
        step.NewStepWithTimeout("api-call", "Call external API", action, 5*time.Second),
        
        // Database operation - 10 seconds timeout
        step.NewStepWithTimeout("db-write", "Write to database", action, 10*time.Second),
        
        // Final step - no timeout
        step.NewStep("cleanup", "Cleanup resources", action),
    }
    
    return steps
}
```

### Example 3: Dynamic Timeout Based on Context

```go
type ConfigurableAction struct {
    defaultTimeout time.Duration
}

func (a *ConfigurableAction) Handle(ctx interface{}) error {
    // Your business logic here
    return nil
}

func (a *ConfigurableAction) AsyncHandler(ctx interface{}, resp interface{}) error {
    return nil
}

func CreateStepWithDynamicTimeout(config map[string]interface{}) *step.Step {
    action := &ConfigurableAction{}
    
    // Create step
    s := step.NewStep("configurable", "Configurable timeout step", action)
    
    // Set timeout based on configuration
    if timeoutSecs, ok := config["timeout_seconds"].(int); ok {
        s.SetTimeout(time.Duration(timeoutSecs) * time.Second)
    } else {
        // Default timeout
        s.SetTimeout(30 * time.Second)
    }
    
    return s
}
```

### Example 4: Handling Timeout in Complex Workflows

```go
import (
    "workflow"
    "workflow/stage"
    "workflow/step"
)

func CreateRobustPipeline() *stage.Stage {
    // Create actions
    validateAction := &ValidationAction{}
    apiAction := &APICallAction{}
    processAction := &ProcessAction{}
    
    // Create steps with appropriate timeouts
    validateStep := step.NewStepWithTimeout(
        "validate",
        "Validate request data",
        validateAction,
        2*time.Second, // Quick validation
    )
    
    apiStep := step.NewStepWithTimeout(
        "fetch-data",
        "Fetch data from external API",
        apiAction,
        10*time.Second, // Allow time for network calls
    )
    
    processStep := step.NewStepWithTimeout(
        "process",
        "Process fetched data",
        processAction,
        30*time.Second, // Complex processing allowed
    )
    
    // Create stage with steps
    stage := stage.NewStage("data-pipeline", "", "serial", validateStep)
    stage.AddStep(apiStep)
    stage.AddStep(processStep)
    
    return stage
}
```

### Example 5: Timeout with Retry Logic

```go
type RetryableAction struct {
    maxRetries int
    retryCount int
}

func (a *RetryableAction) Handle(ctx interface{}) error {
    for a.retryCount < a.maxRetries {
        err := a.doWork()
        if err == nil {
            return nil
        }
        
        // Check if it was a timeout
        if strings.Contains(err.Error(), "timeout") {
            a.retryCount++
            fmt.Printf("Timeout occurred, retry %d/%d\n", a.retryCount, a.maxRetries)
            continue
        }
        
        // Non-timeout error, fail immediately
        return err
    }
    
    return fmt.Errorf("max retries exceeded")
}

func (a *RetryableAction) doWork() error {
    // Actual work here
    return nil
}

func (a *RetryableAction) AsyncHandler(ctx interface{}, resp interface{}) error {
    return nil
}
```

## Timeout Behavior

### Normal Execution (No Timeout)
```
Step Start → Handle Executes → Completes → Step Status: done
```

### Timeout Occurs
```
Step Start → Handle Executes → Timeout Triggered → Step Status: failed
                                 ↓
                        Error: "step execution timeout after Xs"
```

### With Panic Recovery
```
Step Start → Handle Executes → Panic Occurs → Caught → Step Status: failed
                                                   ↓
                                   Error: "panic in step actor: <panic message>"
```

## Important Notes

### 1. **Goroutine Safety**
The timeout mechanism executes the step in a separate goroutine. Ensure your `Handle` implementation is goroutine-safe if it accesses shared state.

### 2. **Resource Cleanup**
When a timeout occurs, the goroutine running the step may still be executing. Ensure proper resource cleanup:

```go
func (a *MyAction) Handle(ctx interface{}) error {
    // Use defer for cleanup
    resource := acquireResource()
    defer resource.Release()
    
    // Your work here
    return doWork(resource)
}
```

### 3. **Cancellation Support**
For better timeout handling, consider using context cancellation in your implementation:

```go
func (a *MyAction) Handle(ctx interface{}) error {
    // If you need fine-grained control, use context
    workCtx, cancel := context.WithCancel(context.Background())
    defer cancel()
    
    // Do work with workCtx
    return doWorkWithContext(workCtx)
}
```

### 4. **Timeout Values**
Choose appropriate timeout values:
- **Too short**: May cause unnecessary failures
- **Too long**: May delay error detection

Recommended ranges:
- Quick validations: 1-5 seconds
- API calls: 5-30 seconds
- Database operations: 10-60 seconds
- Complex processing: 30-300 seconds

### 5. **Async Handler Timeout**
The timeout also applies to `AsyncHandler()` calls:

```go
// Async handler with 5 second timeout
step := step.NewStepWithTimeout("async-step", "Async operation", action, 5*time.Second)
```

## Error Messages

When timeout occurs, you'll see:
- **Log message**: `Step execution timeout` with timeout duration and step name
- **Error returned**: `"step execution timeout after 5s"` (example)

When panic occurs:
- **Log message**: `Step execution failed` with panic details
- **Error returned**: `"panic in step actor: <panic message>"`

## Best Practices

### 1. **Set Reasonable Timeouts**
```go
// Good: Specific timeout based on expected duration
step := step.NewStepWithTimeout("api-call", "External API", action, 10*time.Second)

// Avoid: Too short, may cause false failures
step := step.NewStepWithTimeout("api-call", "External API", action, 100*time.Millisecond)
```

### 2. **Use Different Timeouts for Different Operations**
```go
// Fast operations
validateStep := step.NewStepWithTimeout("validate", "", action, 2*time.Second)

// Slow operations
processStep := step.NewStepWithTimeout("process", "", action, 60*time.Second)
```

### 3. **Log Timeout Information**
```go
func (a *MyAction) Handle(ctx interface{}) error {
    start := time.Now()
    defer func() {
        duration := time.Since(start)
        log.Printf("Step execution took %v", duration)
    }()
    
    // Your work here
    return nil
}
```

### 4. **Monitor and Adjust**
- Monitor timeout occurrences in production
- Adjust timeouts based on actual performance data
- Consider environment differences (dev vs prod)

## Complete Example

```go
package main

import (
    "fmt"
    "log"
    "time"
    
    "workflow"
    "workflow/logger"
    "workflow/stage"
    "workflow/step"
)

// Define your action
type DataProcessingAction struct {
    slowOperation bool
}

func (a *DataProcessingAction) Handle(ctx interface{}) error {
    fmt.Println("Starting data processing...")
    
    if a.slowOperation {
        // Simulate slow operation
        time.Sleep(15 * time.Second)
    } else {
        // Fast operation
        time.Sleep(1 * time.Second)
    }
    
    fmt.Println("Data processing completed")
    return nil
}

func (a *DataProcessingAction) AsyncHandler(ctx interface{}, resp interface{}) error {
    fmt.Println("Handling async response")
    return nil
}

func main() {
    // Create workflow
    logger := logger.NewTextLogger(log.LevelInfo)
    wf := workflow.NewWorkflow(logger, workflow.WorkflowConfig{})
    
    // Create actions
    fastAction := &DataProcessingAction{slowOperation: false}
    slowAction := &DataProcessingAction{slowOperation: true}
    
    // Create steps with timeouts
    step1 := step.NewStepWithTimeout("fast-step", "Fast operation", fastAction, 5*time.Second)
    step2 := step.NewStepWithTimeout("slow-step", "Slow operation", slowAction, 10*time.Second)
    // step2 will timeout because it takes 15s but timeout is 10s
    
    // Create stage
    task := stage.NewStage("processing-task", "", "serial", step1)
    task.AddStep(step2)
    
    // Register pipeline
    err := wf.CreatePipeline("data-pipeline", task)
    if err != nil {
        log.Fatal(err)
    }
    
    // Launch pipeline
    pl, _ := wf.GetPipelineByName("data-pipeline")
    err = wf.LaunchPipeline(pl.ID, map[string]interface{}{
        "data": "sample data",
    })
    
    if err != nil {
        log.Fatal(err)
    }
    
    // Wait for completion
    time.Sleep(30 * time.Second)
    
    wf.Close()
}
```

## Summary

The timeout control feature provides:
- ✅ Protection against hung or slow operations
- ✅ Automatic failure handling
- ✅ Panic recovery
- ✅ Flexible configuration per step
- ✅ Clear error messages
- ✅ Production-ready reliability

Use timeouts to make your workflows more robust and predictable!
