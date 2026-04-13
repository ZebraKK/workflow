# Generic Context Parameter Guide

## Overview

The workflow engine now supports **generic context parameters** that allow transparent passing of user-defined data structures from the top-level `LaunchPipeline()` call down through stages to individual step implementations.

## Key Changes

### 1. **Interface Changes**

All interfaces and methods that previously used `ctx string` have been updated to use `ctx interface{}`:

- `Tasker` interface: `Run(ctx interface{}, ...)` and `AsyncHandler(ctx interface{}, ...)`
- `steper` interface: `Run(ctx interface{}, ...)` and `AsyncHandler(ctx interface{}, ...)`
- `Actioner` interface: `Handle(ctx interface{})` and `AsyncHandler(ctx interface{}, resp string)` (Note: AsyncHandler is in the AsyncActioner interface)

### 2. **Workflow Methods**

- `LaunchPipeline(id string, ctx interface{}) error` - Now accepts any type of context

### 3. **Transparent Parameter Passing**

The context parameter flows through the entire execution chain:
```
LaunchPipeline(ctx) 
  → Job.ctx
    → Task.Run(ctx)
      → Stage.Run(ctx)
        → Step.Run(ctx)
          → Actioner.Handle(ctx)
```

For async workflows:
```
CallbackHandler(resp)
  → AsyncJob with Job.ctx
    → Task.AsyncHandler(ctx, resp)
      → Stage.AsyncHandler(ctx, resp)
        → Step.AsyncHandler(ctx, resp)
          → Actioner.AsyncHandler(ctx, resp)
```

## Usage Examples

### Example 1: Simple String Context

```go
wf.LaunchPipeline(pipelineID, "simple context data")
```

In your step implementation:
```go
func (a *MyAction) Handle(ctx interface{}) error {
    if str, ok := ctx.(string); ok {
        fmt.Println("Context:", str)
    }
    return nil
}
```

### Example 2: Custom Struct Context

Define your context structure:
```go
type RequestContext struct {
    UserID      string
    RequestID   string
    Timestamp   int64
    Metadata    map[string]interface{}
}
```

Launch pipeline with custom context:
```go
ctx := &RequestContext{
    UserID:    "user123",
    RequestID: "req-456",
    Timestamp: time.Now().Unix(),
    Metadata: map[string]interface{}{
        "source": "api",
        "priority": "high",
    },
}

err := wf.LaunchPipeline(pipelineID, ctx)
```

Access in step implementation:
```go
func (a *MyAction) Handle(ctx interface{}) error {
    if reqCtx, ok := ctx.(*RequestContext); ok {
        fmt.Printf("Processing request for user: %s\n", reqCtx.UserID)
        fmt.Printf("Request ID: %s\n", reqCtx.RequestID)

        // Access metadata
        if source, ok := reqCtx.Metadata["source"].(string); ok {
            fmt.Printf("Source: %s\n", source)
        }
    }
    return nil
}
```

### Example 3: Map Context

```go
ctx := map[string]interface{}{
    "userID": "user123",
    "action": "process_order",
    "orderID": 789,
    "params": map[string]string{
        "payment_method": "credit_card",
        "shipping": "express",
    },
}

err := wf.LaunchPipeline(pipelineID, ctx)
```

Access in step:
```go
func (a *MyAction) Handle(ctx interface{}) error {
    if ctxMap, ok := ctx.(map[string]interface{}); ok {
        userID := ctxMap["userID"].(string)
        orderID := ctxMap["orderID"].(int)

        fmt.Printf("Processing order %d for user %s\n", orderID, userID)
    }
    return nil
}
```

### Example 4: Nil Context

If no context is needed:
```go
err := wf.LaunchPipeline(pipelineID, nil)
```

Handle gracefully in step:
```go
func (a *MyAction) Handle(ctx interface{}) error {
    if ctx == nil {
        fmt.Println("No context provided, using defaults")
        return nil
    }
    // Process context
    return nil
}
```

### Example 5: Async Handler with Context

```go
func (a *MyAction) AsyncHandler(ctx interface{}, resp string) error {
    if reqCtx, ok := ctx.(*RequestContext); ok {
        fmt.Printf("Async response for request %s: %s\n", 
            reqCtx.RequestID, resp)
        
        // Use context data to process async response
        // ...
    }
    return nil
}
```

## Best Practices

### 1. **Type Assertion Safety**

Always use type assertion with the comma-ok idiom:
```go
if myCtx, ok := ctx.(*MyContext); ok {
    // Safe to use myCtx
} else {
    // Handle unexpected type or nil
}
```

### 2. **Define Clear Context Structures**

Create well-defined context structures for your use case:
```go
type OrderProcessingContext struct {
    OrderID       int64
    CustomerID    string
    PaymentInfo   PaymentDetails
    ShippingInfo  ShippingDetails
    CreatedAt     time.Time
}
```

### 3. **Document Expected Context Types**

Document what context type your task/step expects:
```go
// ProcessOrderStep expects a *OrderProcessingContext
type ProcessOrderStep struct {
    // ...
}

func (s *ProcessOrderStep) Handle(ctx interface{}) error {
    orderCtx, ok := ctx.(*OrderProcessingContext)
    if !ok {
        return errors.New("expected *OrderProcessingContext")
    }
    // Process order
    return nil
}
```

### 4. **Handle Nil Context**

Always handle the case where ctx might be nil:
```go
func (a *MyAction) Handle(ctx interface{}) error {
    if ctx == nil {
        // Use default behavior or return error
        return errors.New("context is required")
    }
    // Process context
    return nil
}
```

### 5. **Context Immutability**

Treat context as read-only. Don't modify the context within steps to avoid race conditions in parallel stages:
```go
// DON'T DO THIS in parallel stages:
func (a *MyAction) Handle(ctx interface{}) error {
    if ctxMap, ok := ctx.(map[string]interface{}); ok {
        ctxMap["modified"] = true // UNSAFE in parallel execution
    }
    return nil
}

// Instead, create copies if needed:
func (a *MyAction) Handle(ctx interface{}) error {
    if ctxMap, ok := ctx.(map[string]interface{}); ok {
        localCopy := make(map[string]interface{})
        for k, v := range ctxMap {
            localCopy[k] = v
        }
        localCopy["modified"] = true // Safe
    }
    return nil
}
```

## Migration from Old Code

### Old Code (no context parameter):
```go
func (a *MyAction) Handle() error {
    // No access to context
    return nil
}

func (a *MyAction) AsyncHandler(resp string) error {
    // No access to context
    return nil
}
```

### New Code (generic context):
```go
func (a *MyAction) Handle(ctx interface{}) error {
    // Can now access context
    if myCtx, ok := ctx.(*MyContext); ok {
        // Use context data
    }
    return nil
}

func (a *MyAction) AsyncHandler(ctx interface{}, resp string) error {
    // Context available in async handler too
    if myCtx, ok := ctx.(*MyContext); ok {
        // Use context data with async response
    }
    return nil
}
```

## Benefits

1. **Type Safety**: Use strongly-typed structs for your context
2. **Flexibility**: Pass any type of data structure you need
3. **Transparency**: Workflow engine doesn't need to know about your data structures
4. **Testability**: Easy to mock different context scenarios in tests
5. **Scalability**: Works seamlessly with serial and parallel stages
6. **Async Support**: Context flows through async callbacks automatically

## Complete Example

```go
package main

import (
    "fmt"
    "workflow"
    "workflow/stage"
    "workflow/step"
)

// Define your context
type APIRequestContext struct {
    APIKey      string
    UserID      string
    RequestData map[string]interface{}
}

// Implement your action
type FetchDataAction struct{}

func (a *FetchDataAction) Handle(ctx interface{}) error {
    apiCtx, ok := ctx.(*APIRequestContext)
    if !ok {
        return fmt.Errorf("expected *APIRequestContext")
    }

    fmt.Printf("Fetching data for user %s using API key %s\n",
        apiCtx.UserID, apiCtx.APIKey)

    // Your business logic here
    return nil
}

func (a *FetchDataAction) AsyncHandler(ctx interface{}, resp string) error {
    apiCtx, ok := ctx.(*APIRequestContext)
    if !ok {
        return fmt.Errorf("expected *APIRequestContext")
    }
    
    fmt.Printf("Async response for user %s: %s\n", 
        apiCtx.UserID, resp)
    return nil
}

func main() {
    // Create workflow
    wf := workflow.NewWorkflow(nil, workflow.WorkflowConfig{})
    
    // Create task with steps
    action := &FetchDataAction{}
    step1 := step.NewStep("fetch", "Fetch data from API", action)
    task := stage.NewStage("api-task", "", "serial", step1)
    
    // Register pipeline
    wf.CreatePipeline("api-pipeline", task)
    
    // Launch with custom context
    ctx := &APIRequestContext{
        APIKey: "secret-key-123",
        UserID: "user-456",
        RequestData: map[string]interface{}{
            "query": "select * from users",
        },
    }
    
    pipelineID := "api-pipeline"
    if pl, ok := wf.GetPipelineByName("api-pipeline"); ok {
        pipelineID = pl.ID
    }
    
    err := wf.LaunchPipeline(pipelineID, ctx)
    if err != nil {
        fmt.Printf("Error: %v\n", err)
    }
}
```

## Summary

The generic context parameter feature transforms the workflow engine into a true orchestration platform that can handle any type of user data while maintaining type safety and transparency. The engine acts as a pure scheduler without needing to understand the specifics of your business logic or data structures.
