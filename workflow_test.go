package workflow

import (
	"errors"
	"log/slog"
	"sync"
	"testing"
	"time"
	"workflow/logger"
	"workflow/record"
	"workflow/stage"
	"workflow/step"
)

// Mock Actioner for testing
type MockActioner struct {
	handleFunc  func(ctx interface{}) error
	handleError error
	mu          sync.Mutex
}

func (m *MockActioner) Handle(ctx interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.handleFunc != nil {
		return m.handleFunc(ctx)
	}
	return m.handleError
}

// Mock AsyncActioner for testing
type MockAsyncActioner struct {
	asyncHandleFunc  func(ctx interface{}, resp interface{}) error
	asyncHandleError error
	mu               sync.Mutex
}

func (m *MockAsyncActioner) AsyncHandle(ctx interface{}, resp interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.asyncHandleFunc != nil {
		return m.asyncHandleFunc(ctx, resp)
	}
	return m.asyncHandleError
}

var noOpLogger = logger.NewNoOpLogger()
var slogger = logger.NewSlogLogger(slog.LevelDebug)

// RealTask implements Tasker using actual stage and step
type RealTask struct {
	stages []*stage.Stage
}

func NewRealTask() *RealTask {
	return &RealTask{
		stages: make([]*stage.Stage, 0),
	}
}

func (rt *RealTask) AddStage(stg *stage.Stage) {
	rt.stages = append(rt.stages, stg)
}

func (rt *RealTask) IsAsync() bool {
	for _, stg := range rt.stages {
		if stg.IsAsync() {
			return true
		}
	}
	return false
}

func (rt *RealTask) Handle(ctx interface{}, rcder *record.Record, logger Logger) error {
	if len(rt.stages) == 0 {
		rcder.Status = record.StatusDone
		return nil
	}

	// Run stages serially
	for i, stg := range rt.stages {
		nextRecord := record.NewRecord(rcder.ID, string(rune('0'+i)), stg.StepsCount())
		rcder.AddRecord(i, nextRecord)

		err := stg.Handle(ctx, nextRecord, logger)
		if err != nil {
			rcder.Status = record.StatusFailed
			return err
		}

		if nextRecord.Status == record.StatusFailed {
			rcder.Status = record.StatusFailed
			return errors.New("stage failed")
		}

		if nextRecord.Status == record.StatusAsyncWaiting {
			rcder.Status = record.StatusAsyncWaiting
			return nil
		}
	}

	rcder.Status = record.StatusDone
	return nil
}

func (rt *RealTask) AsyncHandle(ctx interface{}, resp interface{}, runningID string, ids []int, stageIndex int, rcder *record.Record, logger Logger) {
	if stageIndex >= len(ids) || stageIndex >= len(rt.stages) {
		return
	}

	index := ids[stageIndex]
	if index < 0 || index >= len(rt.stages) {
		return
	}

	stg := rt.stages[index]
	if index < len(rcder.Records) {
		nextRecord := rcder.Records[index]
		stg.AsyncHandle(ctx, resp, runningID, ids, stageIndex+1, nextRecord, logger)
	}
}

func (rt *RealTask) StepsCount() int {
	return len(rt.stages)
}

// Test NewWorkflow
func TestNewWorkflow(t *testing.T) {
	wf := NewWorkflow(noOpLogger, WorkflowConfig{})

	if wf == nil {
		t.Fatal("NewWorkflow() returned nil")
	}

	if wf.pipelineMap == nil {
		t.Error("pipelineMap not initialized")
	}

	if wf.jobsStore == nil {
		t.Error("jobsStore not initialized")
	}

	if wf.workerNum != 5 {
		t.Errorf("workerNum = %d, want 5", wf.workerNum)
	}

	if wf.JobCh == nil {
		t.Error("JobCh not initialized")
	}

	if wf.AsyncCh == nil {
		t.Error("AsyncCh not initialized")
	}

	if wf.quitJobCh == nil {
		t.Error("quitJobCh not initialized")
	}

	// Give goroutines time to start
	time.Sleep(10 * time.Millisecond)
}

// Test CreatePipeline
func TestCreatePipeline(t *testing.T) {
	wf := NewWorkflow(noOpLogger, WorkflowConfig{})
	defer time.Sleep(10 * time.Millisecond) // cleanup

	task := NewRealTask()
	actor := &MockActioner{}
	stp := step.NewStep("step1", "Step 1", 5*time.Second, actor, nil)
	stg := stage.NewStage("stage1", "stage-1", "serial", stp)
	task.AddStage(stg)

	err := wf.CreatePipeline("test-pipeline", task)
	if err != nil {
		t.Errorf("CreatePipeline() error = %v, want nil", err)
	}

	// Check pipeline was created
	wf.muPl.RLock()
	count := len(wf.pipelineMap)
	wf.muPl.RUnlock()

	if count != 1 {
		t.Errorf("pipelineMap length = %d, want 1", count)
	}
}

func TestCreatePipeline_NilTask(t *testing.T) {
	wf := NewWorkflow(noOpLogger, WorkflowConfig{})
	defer time.Sleep(10 * time.Millisecond)

	// This should work but pipeline will have nil task
	err := wf.CreatePipeline("test-pipeline", nil)
	if err != nil {
		t.Errorf("CreatePipeline() with nil task error = %v, want nil", err)
	}
}

// Test GetPipeline
func TestGetPipeline(t *testing.T) {
	wf := NewWorkflow(noOpLogger, WorkflowConfig{})
	defer time.Sleep(10 * time.Millisecond)

	task := NewRealTask()
	actor := &MockActioner{}
	stp := step.NewStep("step1", "Step 1", 5*time.Second, actor, nil)
	stg := stage.NewStage("stage1", "stage-1", "serial", stp)
	task.AddStage(stg)

	err := wf.CreatePipeline("test-pipeline", task)
	if err != nil {
		t.Fatalf("CreatePipeline() failed: %v", err)
	}

	// Get the pipeline ID
	var pipelineID string
	wf.muPl.RLock()
	for id := range wf.pipelineMap {
		pipelineID = id
		break
	}
	wf.muPl.RUnlock()

	pl, ok := wf.GetPipeline(pipelineID)
	if !ok {
		t.Error("GetPipeline() ok = false, want true")
	}
	if pl == nil {
		t.Error("GetPipeline() returned nil pipeline")
	} else if pl.Name != "test-pipeline" {
		t.Errorf("Pipeline name = %v, want test-pipeline", pl.Name)
	}
}

func TestGetPipeline_NotFound(t *testing.T) {
	wf := NewWorkflow(noOpLogger, WorkflowConfig{})
	defer time.Sleep(10 * time.Millisecond)

	pl, ok := wf.GetPipeline("non-existent-id")
	if ok {
		t.Error("GetPipeline() ok = true, want false for non-existent pipeline")
	}
	if pl != nil {
		t.Error("GetPipeline() should return nil for non-existent pipeline")
	}
}

// Test DeletePipeline
func TestDeletePipeline(t *testing.T) {
	wf := NewWorkflow(noOpLogger, WorkflowConfig{})
	defer time.Sleep(10 * time.Millisecond)

	task := NewRealTask()
	actor := &MockActioner{}
	stp := step.NewStep("step1", "Step 1", 5*time.Second, actor, nil)
	stg := stage.NewStage("stage1", "stage-1", "serial", stp)
	task.AddStage(stg)

	wf.CreatePipeline("test-pipeline", task)

	// Get the pipeline ID
	var pipelineID string
	wf.muPl.RLock()
	for id := range wf.pipelineMap {
		pipelineID = id
		break
	}
	wf.muPl.RUnlock()

	err := wf.DeletePipeline(pipelineID)
	if err != nil {
		t.Errorf("DeletePipeline() error = %v, want nil", err)
	}

	// Verify deletion
	_, ok := wf.GetPipeline(pipelineID)
	if ok {
		t.Error("Pipeline should be deleted")
	}
}

func TestDeletePipeline_NotFound(t *testing.T) {
	wf := NewWorkflow(noOpLogger, WorkflowConfig{})
	defer time.Sleep(10 * time.Millisecond)

	err := wf.DeletePipeline("non-existent-id")
	if err == nil {
		t.Error("DeletePipeline() should return error for non-existent pipeline")
	}
	if err.Error() != "pipeline not found" {
		t.Errorf("DeletePipeline() error = %v, want 'pipeline not found'", err)
	}
}

// Test UpdatePipeline
func TestUpdatePipeline(t *testing.T) {
	wf := NewWorkflow(noOpLogger, WorkflowConfig{})
	defer time.Sleep(10 * time.Millisecond)

	task1 := NewRealTask()
	actor1 := &MockActioner{}
	stp1 := step.NewStep("step1", "Step 1", 5*time.Second, actor1, nil)
	stg1 := stage.NewStage("stage1", "stage-1", "serial", stp1)
	task1.AddStage(stg1)

	task2 := NewRealTask()
	actor2 := &MockActioner{}
	stp2 := step.NewStep("step2", "Step 2", 5*time.Second, actor2, nil)
	stg2 := stage.NewStage("stage2", "stage-2", "serial", stp2)
	task2.AddStage(stg2)
	task2.AddStage(stg2) // Add second stage

	wf.CreatePipeline("test-pipeline", task1)

	// Get the pipeline ID
	var pipelineID string
	wf.muPl.RLock()
	for id := range wf.pipelineMap {
		pipelineID = id
		break
	}
	wf.muPl.RUnlock()

	err := wf.UpdatePipeline(pipelineID, task2)
	if err != nil {
		t.Errorf("UpdatePipeline() error = %v, want nil", err)
	}

	// Verify update
	pl, _ := wf.GetPipeline(pipelineID)
	if pl.task.StepsCount() != 2 {
		t.Errorf("Updated task stepsCount = %d, want 2", pl.task.StepsCount())
	}
}

func TestUpdatePipeline_NotFound(t *testing.T) {
	wf := NewWorkflow(noOpLogger, WorkflowConfig{})
	defer time.Sleep(10 * time.Millisecond)

	task := NewRealTask()
	err := wf.UpdatePipeline("non-existent-id", task)
	if err == nil {
		t.Error("UpdatePipeline() should return error for non-existent pipeline")
	}
}

// Test ListPipelines
func TestListPipelines(t *testing.T) {
	wf := NewWorkflow(noOpLogger, WorkflowConfig{})
	defer time.Sleep(10 * time.Millisecond)

	task := NewRealTask()
	actor := &MockActioner{}
	stp := step.NewStep("step1", "Step 1", 5*time.Second, actor, nil)
	stg := stage.NewStage("stage1", "stage-1", "serial", stp)
	task.AddStage(stg)

	wf.CreatePipeline("pipeline1", task)
	wf.CreatePipeline("pipeline2", task)
	wf.CreatePipeline("pipeline3", task)

	pipelines := wf.ListPipelines()

	if len(pipelines) != 3 {
		t.Errorf("ListPipelines() length = %d, want 3", len(pipelines))
	}
}

func TestListPipelines_Empty(t *testing.T) {
	wf := NewWorkflow(noOpLogger, WorkflowConfig{})
	defer time.Sleep(10 * time.Millisecond)

	pipelines := wf.ListPipelines()

	if len(pipelines) != 0 {
		t.Errorf("ListPipelines() length = %d, want 0", len(pipelines))
	}
}

// Test LaunchPipeline
func TestLaunchPipeline(t *testing.T) {
	wf := NewWorkflow(noOpLogger, WorkflowConfig{})
	defer time.Sleep(50 * time.Millisecond) // Allow time for job processing

	task := NewRealTask()
	actor := &MockActioner{}
	stp := step.NewStep("step1", "Step 1", 5*time.Second, actor, nil)
	stg := stage.NewStage("stage1", "stage-1", "serial", stp)
	task.AddStage(stg)

	wf.CreatePipeline("test-pipeline", task)

	// Get the pipeline ID
	var pipelineID string
	wf.muPl.RLock()
	for id := range wf.pipelineMap {
		pipelineID = id
		break
	}
	wf.muPl.RUnlock()

	err := wf.LaunchPipeline(pipelineID, "test-context")
	if err != nil {
		t.Errorf("LaunchPipeline() error = %v, want nil", err)
	}

	// Give time for job to be processed
	time.Sleep(30 * time.Millisecond)
}

func TestLaunchPipeline_NotFound(t *testing.T) {
	wf := NewWorkflow(noOpLogger, WorkflowConfig{})
	defer time.Sleep(10 * time.Millisecond)

	err := wf.LaunchPipeline("non-existent-id", "context")
	if err == nil {
		t.Error("LaunchPipeline() should return error for non-existent pipeline")
	}
}

func TestLaunchPipeline_ChannelFull(t *testing.T) {
	wf := NewWorkflow(slogger, WorkflowConfig{
		WorkerNum:   1,
		JobChSize:   2,
		AsyncChSize: 2,
	})
	defer time.Sleep(10 * time.Millisecond)

	// Create a slow task that blocks the workers
	blockChan := make(chan struct{})
	defer close(blockChan)
	slowActor := &MockActioner{
		handleFunc: func(ctx interface{}) error {
			<-blockChan // Block until we release
			return nil
		},
	}

	task := NewRealTask()
	stp := step.NewStep("step1", "Slow Step", 60*time.Second, slowActor, nil)
	stg := stage.NewStage("stage1", "stage-1", "serial", stp)
	task.AddStage(stg)

	testName := "test-pipeline"
	wf.CreatePipeline(testName, task)

	var pipelineID string
	pl, ok := wf.GetPipelineByName(testName)
	if !ok {
		t.Fatal("Pipeline not found")
	} else {
		pipelineID = pl.ID
	}

	// Fill up the job channel and workers
	for i := 0; i < 5; i++ {
		wf.LaunchPipeline(pipelineID, "context")
	}

	// This should fail because channel is full
	err := wf.LaunchPipeline(pipelineID, "context")

	if err == nil {
		t.Error("LaunchPipeline() should return error when channel is full")
	}
}

// Test parseStageByRunningID
func TestParseStageByRunningID(t *testing.T) {
	tests := []struct {
		name      string
		runningID string
		want      []int
	}{
		{
			name:      "simple case",
			runningID: "job-1-2-3",
			want:      []int{1, 2, 3},
		},
		{
			name:      "single index",
			runningID: "job-5",
			want:      []int{5},
		},
		{
			name:      "no indices",
			runningID: "job",
			want:      []int{},
		},
		{
			name:      "mixed valid and invalid",
			runningID: "job-1-abc-3",
			want:      []int{1, 3},
		},
		{
			name:      "empty string",
			runningID: "",
			want:      []int{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseStageByRunningID(tt.runningID)
			if len(got) != len(tt.want) {
				t.Errorf("parseStageByRunningID() length = %v, want %v", len(got), len(tt.want))
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("parseStageByRunningID()[%d] = %v, want %v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

// Test CallbackHandler
func TestCallbackHandler(t *testing.T) {
	wf := NewWorkflow(noOpLogger, WorkflowConfig{})
	defer time.Sleep(50 * time.Millisecond)

	asyncActor := &MockAsyncActioner{}
	task := NewRealTask()
	stp := step.NewStep("step1", "Async Step", 5*time.Second, &MockActioner{}, asyncActor)
	stg := stage.NewStage("stage1", "stage-1", "serial", stp)
	task.AddStage(stg)

	wf.CreatePipeline("test-pipeline", task)

	var pipelineID string
	wf.muPl.RLock()
	for id := range wf.pipelineMap {
		pipelineID = id
		break
	}
	wf.muPl.RUnlock()

	// Launch pipeline to create a job
	wf.LaunchPipeline(pipelineID, "context")
	time.Sleep(20 * time.Millisecond)

	// Get job ID
	wf.muJs.RLock()
	var jobID string
	for id := range wf.jobsStore {
		jobID = id
		break
	}
	wf.muJs.RUnlock()

	if jobID == "" {
		t.Skip("No job found in store")
		return
	}

	// Call the callback handler
	err := wf.CallbackHandler(jobID+"-0-0", "test-response")
	if err != nil {
		t.Errorf("CallbackHandler() error = %v, want nil", err)
	}

	time.Sleep(20 * time.Millisecond)
}

func TestCallbackHandler_EmptyID(t *testing.T) {
	wf := NewWorkflow(noOpLogger, WorkflowConfig{})
	defer time.Sleep(10 * time.Millisecond)

	err := wf.CallbackHandler("", "response")
	if err == nil {
		t.Error("CallbackHandler() should return error for empty ID")
	}
	if err.Error() != "invalid id" {
		t.Errorf("CallbackHandler() error = %v, want 'invalid id'", err)
	}
}

func TestCallbackHandler_JobNotFound(t *testing.T) {
	wf := NewWorkflow(noOpLogger, WorkflowConfig{})
	defer time.Sleep(10 * time.Millisecond)

	err := wf.CallbackHandler("non-existent-job-1", "response")
	if err == nil {
		t.Error("CallbackHandler() should return error for non-existent job")
	}
}

// Test generateID
func TestGenerateID(t *testing.T) {
	id1 := generateID("test")
	id2 := generateID("test")

	if id1 == id2 {
		t.Error("generateID() should produce unique IDs")
	}

	if len(id1) != 32 {
		t.Errorf("generateID() length = %d, want 32", len(id1))
	}
}

// Test concurrent access
func TestConcurrentPipelineOperations(t *testing.T) {
	wf := NewWorkflow(noOpLogger, WorkflowConfig{})
	defer time.Sleep(100 * time.Millisecond)

	var wg sync.WaitGroup
	numOps := 10

	// Concurrent creates
	for i := 0; i < numOps; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			task := NewRealTask()
			actor := &MockActioner{}
			stp := step.NewStep("step1", "Step 1", 5*time.Second, actor, nil)
			stg := stage.NewStage("stage1", "stage-1", "serial", stp)
			task.AddStage(stg)
			wf.CreatePipeline("pipeline-"+string(rune(n)), task)
		}(i)
	}

	wg.Wait()

	pipelines := wf.ListPipelines()
	if len(pipelines) != numOps {
		t.Errorf("After concurrent creates, got %d pipelines, want %d", len(pipelines), numOps)
	}
}

func TestConcurrentJobLaunches(t *testing.T) {
	wf := NewWorkflow(noOpLogger, WorkflowConfig{})
	defer time.Sleep(100 * time.Millisecond)

	task := NewRealTask()
	actor := &MockActioner{}
	stp := step.NewStep("step1", "Step 1", 5*time.Second, actor, nil)
	stg := stage.NewStage("stage1", "stage-1", "serial", stp)
	task.AddStage(stg)

	wf.CreatePipeline("test-pipeline", task)

	var pipelineID string
	wf.muPl.RLock()
	for id := range wf.pipelineMap {
		pipelineID = id
		break
	}
	wf.muPl.RUnlock()

	var wg sync.WaitGroup
	numLaunches := 5

	for i := 0; i < numLaunches; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			wf.LaunchPipeline(pipelineID, "context")
		}()
	}

	wg.Wait()
	time.Sleep(50 * time.Millisecond)
}

// Benchmark tests
func BenchmarkCreatePipeline(b *testing.B) {
	wf := NewWorkflow(noOpLogger, WorkflowConfig{})
	task := NewRealTask()
	actor := &MockActioner{}
	stp := step.NewStep("step1", "Step 1", 5*time.Second, actor, nil)
	stg := stage.NewStage("stage1", "stage-1", "serial", stp)
	task.AddStage(stg)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		wf.CreatePipeline("pipeline-"+string(rune(i)), task)
	}
}

func BenchmarkLaunchPipeline(b *testing.B) {
	wf := NewWorkflow(noOpLogger, WorkflowConfig{})
	task := NewRealTask()
	actor := &MockActioner{}
	stp := step.NewStep("step1", "Step 1", 5*time.Second, actor, nil)
	stg := stage.NewStage("stage1", "stage-1", "serial", stp)
	task.AddStage(stg)

	wf.CreatePipeline("test-pipeline", task)

	var pipelineID string
	wf.muPl.RLock()
	for id := range wf.pipelineMap {
		pipelineID = id
		break
	}
	wf.muPl.RUnlock()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		wf.LaunchPipeline(pipelineID, "context")
	}
}

func BenchmarkGenerateID(b *testing.B) {
	for i := 0; i < b.N; i++ {
		generateID("test-value")
	}
}

// Integration tests using real stage and step

// Test with real stage and step - Serial execution
func TestIntegration_RealStageStep_Serial(t *testing.T) {
	wf := NewWorkflow(noOpLogger, WorkflowConfig{})
	defer time.Sleep(50 * time.Millisecond)

	// Create real steps with actors
	actor1 := &MockActioner{}
	actor2 := &MockActioner{}

	stp1 := step.NewStep("step1", "Step 1", 5*time.Second, actor1, nil)
	stp2 := step.NewStep("step2", "Step 2", 5*time.Second, actor2, nil)

	// Create real stage
	stg := stage.NewStage("stage1", "stage-id-1", "serial", stp1)
	stg.AddStep(stp2)

	// Create real task
	task := NewRealTask()
	task.AddStage(stg)

	// Create and launch pipeline
	err := wf.CreatePipeline("real-pipeline", task)
	if err != nil {
		t.Fatalf("CreatePipeline() failed: %v", err)
	}

	var pipelineID string
	wf.muPl.RLock()
	for id := range wf.pipelineMap {
		pipelineID = id
		break
	}
	wf.muPl.RUnlock()

	err = wf.LaunchPipeline(pipelineID, "test-context")
	if err != nil {
		t.Fatalf("LaunchPipeline() failed: %v", err)
	}

	time.Sleep(30 * time.Millisecond)
}

// Test with real stage and step - Parallel execution
func TestIntegration_RealStageStep_Parallel(t *testing.T) {
	wf := NewWorkflow(noOpLogger, WorkflowConfig{})
	defer time.Sleep(50 * time.Millisecond)

	// Create real steps with actors
	actor1 := &MockActioner{}
	actor2 := &MockActioner{}
	actor3 := &MockActioner{}

	stp1 := step.NewStep("step1", "Step 1", 5*time.Second, actor1, nil)
	stp2 := step.NewStep("step2", "Step 2", 5*time.Second, actor2, nil)
	stp3 := step.NewStep("step3", "Step 3", 5*time.Second, actor3, nil)

	// Create real stage with parallel mode
	stg := stage.NewStage("stage1", "stage-id-1", "parallel", stp1)
	stg.AddStep(stp2)
	stg.AddStep(stp3)

	// Create real task
	task := NewRealTask()
	task.AddStage(stg)

	// Create and launch pipeline
	err := wf.CreatePipeline("parallel-pipeline", task)
	if err != nil {
		t.Fatalf("CreatePipeline() failed: %v", err)
	}

	var pipelineID string
	wf.muPl.RLock()
	for id := range wf.pipelineMap {
		pipelineID = id
		break
	}
	wf.muPl.RUnlock()

	err = wf.LaunchPipeline(pipelineID, "test-context")
	if err != nil {
		t.Fatalf("LaunchPipeline() failed: %v", err)
	}

	time.Sleep(50 * time.Millisecond)
}

// Test with real stage and step - Error handling
func TestIntegration_RealStageStep_Error(t *testing.T) {
	wf := NewWorkflow(noOpLogger, WorkflowConfig{})
	defer time.Sleep(50 * time.Millisecond)

	// Create real step that fails
	failActor := &MockActioner{handleError: errors.New("step execution failed")}
	stp := step.NewStep("fail-step", "Failing Step", 5*time.Second, failActor, nil)

	// Create real stage
	stg := stage.NewStage("error-stage", "stage-id-2", "serial", stp)

	// Create real task
	task := NewRealTask()
	task.AddStage(stg)

	// Create and launch pipeline
	err := wf.CreatePipeline("error-pipeline", task)
	if err != nil {
		t.Fatalf("CreatePipeline() failed: %v", err)
	}

	var pipelineID string
	wf.muPl.RLock()
	for id := range wf.pipelineMap {
		pipelineID = id
		break
	}
	wf.muPl.RUnlock()

	err = wf.LaunchPipeline(pipelineID, "test-context")
	if err != nil {
		t.Fatalf("LaunchPipeline() failed: %v", err)
	}

	time.Sleep(30 * time.Millisecond)
}

// Test with real stage and step - Multiple stages
func TestIntegration_RealStageStep_MultipleStages(t *testing.T) {
	wf := NewWorkflow(noOpLogger, WorkflowConfig{})
	defer time.Sleep(100 * time.Millisecond)

	// Create real steps with actors
	actor1 := &MockActioner{}
	actor2 := &MockActioner{}
	actor3 := &MockActioner{}
	actor4 := &MockActioner{}

	// Stage 1 - Serial
	stp1 := step.NewStep("step1", "Step 1", 5*time.Second, actor1, nil)
	stp2 := step.NewStep("step2", "Step 2", 5*time.Second, actor2, nil)
	stg1 := stage.NewStage("stage1", "stage-1", "serial", stp1)
	stg1.AddStep(stp2)

	// Stage 2 - Parallel
	stp3 := step.NewStep("step3", "Step 3", 5*time.Second, actor3, nil)
	stp4 := step.NewStep("step4", "Step 4", 5*time.Second, actor4, nil)
	stg2 := stage.NewStage("stage2", "stage-2", "parallel", stp3)
	stg2.AddStep(stp4)

	// Create real task with multiple stages
	task := NewRealTask()
	task.AddStage(stg1)
	task.AddStage(stg2)

	// Create and launch pipeline
	err := wf.CreatePipeline("multi-stage-pipeline", task)
	if err != nil {
		t.Fatalf("CreatePipeline() failed: %v", err)
	}

	var pipelineID string
	wf.muPl.RLock()
	for id := range wf.pipelineMap {
		pipelineID = id
		break
	}
	wf.muPl.RUnlock()

	err = wf.LaunchPipeline(pipelineID, "test-context")
	if err != nil {
		t.Fatalf("LaunchPipeline() failed: %v", err)
	}

	time.Sleep(60 * time.Millisecond)
}
