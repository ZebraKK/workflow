package stage

import (
	"errors"
	"testing"
	"workflow/record"
)

// Mock steper for testing
type MockSteper struct {
	isAsync     bool
	stepsCount  int
	runFunc     func(ctx string, rcder *record.Record) error
	asyncFunc   func(resp string, runningID string, ids []int, stageIndex int, rcder *record.Record)
	runCalled   bool
	asyncCalled bool
}

func (m *MockSteper) IsAsync() bool {
	return m.isAsync
}

func (m *MockSteper) Run(ctx string, rcder *record.Record) error {
	m.runCalled = true
	if m.runFunc != nil {
		return m.runFunc(ctx, rcder)
	}
	if rcder != nil {
		rcder.Status = record.StatusDone
	}
	return nil
}

func (m *MockSteper) AsyncHandler(resp string, runningID string, ids []int, stageIndex int, rcder *record.Record) {
	m.asyncCalled = true
	if m.asyncFunc != nil {
		m.asyncFunc(resp, runningID, ids, stageIndex, rcder)
	}
	if rcder != nil && rcder.AsyncRecord != nil {
		rcder.AsyncRecord.Status = record.StatusDone
	}
}

func (m *MockSteper) StepsCount() int {
	return m.stepsCount
}

// Test NewStage
func TestNewStage(t *testing.T) {
	tests := []struct {
		name      string
		stageName string
		id        string
		mode      string
		step      steper
		wantNil   bool
		wantID    string
	}{
		{
			name:      "valid stage with ID",
			stageName: "test-stage",
			id:        "custom-id",
			mode:      "serial",
			step:      &MockSteper{stepsCount: 3},
			wantNil:   false,
			wantID:    "custom-id",
		},
		{
			name:      "valid stage without ID",
			stageName: "test-stage",
			id:        "",
			mode:      "parallel",
			step:      &MockSteper{stepsCount: 2},
			wantNil:   false,
			wantID:    "test-stage",
		},
		{
			name:      "nil step",
			stageName: "test-stage",
			id:        "id",
			mode:      "serial",
			step:      nil,
			wantNil:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stage := NewStage(tt.stageName, tt.id, tt.mode, tt.step)

			if tt.wantNil {
				if stage != nil {
					t.Errorf("NewStage() should return nil for nil step")
				}
				return
			}

			if stage == nil {
				t.Fatal("NewStage() returned nil unexpectedly")
			}

			if stage.Name != tt.stageName {
				t.Errorf("Name = %v, want %v", stage.Name, tt.stageName)
			}

			if stage.ID != tt.wantID {
				t.Errorf("ID = %v, want %v", stage.ID, tt.wantID)
			}

			if stage.Mode != tt.mode {
				t.Errorf("Mode = %v, want %v", stage.Mode, tt.mode)
			}

			if len(stage.Steps) != 1 {
				t.Errorf("Steps length = %d, want 1", len(stage.Steps))
			}
		})
	}
}

// Test AddStep
func TestAddStep(t *testing.T) {
	stage := NewStage("test", "id", "serial", &MockSteper{})

	if len(stage.Steps) != 1 {
		t.Errorf("Initial steps count = %d, want 1", len(stage.Steps))
	}

	stage.AddStep(&MockSteper{isAsync: false})
	if len(stage.Steps) != 2 {
		t.Errorf("After adding step, count = %d, want 2", len(stage.Steps))
	}

	if stage.IsAsync() {
		t.Error("Stage should not be async")
	}

	stage.AddStep(&MockSteper{isAsync: true})
	if !stage.IsAsync() {
		t.Error("Stage should be async after adding async step")
	}
}

// Test IsAsync
func TestIsAsync(t *testing.T) {
	tests := []struct {
		name string
		step steper
		want bool
	}{
		{
			name: "sync step",
			step: &MockSteper{isAsync: false},
			want: false,
		},
		{
			name: "async step",
			step: &MockSteper{isAsync: true},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stage := NewStage("test", "", "serial", tt.step)
			if stage.IsAsync() != tt.want {
				t.Errorf("IsAsync() = %v, want %v", stage.IsAsync(), tt.want)
			}
		})
	}
}

// Test StepsCount
func TestStepsCount(t *testing.T) {
	stage := NewStage("test", "", "serial", &MockSteper{})

	if stage.StepsCount() != 1 {
		t.Errorf("StepsCount() = %d, want 1", stage.StepsCount())
	}

	stage.AddStep(&MockSteper{})
	stage.AddStep(&MockSteper{})

	if stage.StepsCount() != 3 {
		t.Errorf("StepsCount() = %d, want 3", stage.StepsCount())
	}
}

// Test GetName and GetID
func TestGetters(t *testing.T) {
	stage := NewStage("my-stage", "my-id", "serial", &MockSteper{})

	if stage.GetName() != "my-stage" {
		t.Errorf("GetName() = %v, want my-stage", stage.GetName())
	}

	if stage.GetID() != "my-id" {
		t.Errorf("GetID() = %v, want my-id", stage.GetID())
	}
}

// Test Run - Serial Mode
func TestRun_Serial(t *testing.T) {
	step := &MockSteper{
		isAsync:    false,
		stepsCount: 0,
	}
	stage := NewStage("test", "", "serial", step)
	rcder := record.NewRecord("test-0", "", 1)

	err := stage.Run("context", rcder)
	if err != nil {
		t.Errorf("Run() error = %v, want nil", err)
	}

	if !step.runCalled {
		t.Error("Step.Run() was not called")
	}

	if rcder.Status != record.StatusDone {
		t.Errorf("Record status = %v, want %v", rcder.Status, record.StatusDone)
	}
}

// Test Run - Parallel Mode
func TestRun_Parallel(t *testing.T) {
	step := &MockSteper{
		isAsync:    false,
		stepsCount: 0,
	}
	stage := NewStage("test", "", "parallel", step)
	rcder := record.NewRecord("test-0", "", 1)

	err := stage.Run("context", rcder)
	if err != nil {
		t.Errorf("Run() error = %v, want nil", err)
	}

	if !step.runCalled {
		t.Error("Step.Run() was not called")
	}
}

// Test Run - Unknown Mode
func TestRun_UnknownMode(t *testing.T) {
	step := &MockSteper{stepsCount: 0}
	stage := NewStage("test", "", "unknown", step)
	rcder := record.NewRecord("test-0", "", 1)

	err := stage.Run("context", rcder)
	if err != nil {
		t.Errorf("Run() with unknown mode should not error, got %v", err)
	}
}

// Test Serial Run - Multiple Steps
func TestSerialRun_MultipleSteps(t *testing.T) {
	step1 := &MockSteper{stepsCount: 0}
	step2 := &MockSteper{stepsCount: 0}
	step3 := &MockSteper{stepsCount: 0}

	stage := NewStage("test", "", "serial", step1)
	stage.AddStep(step2)
	stage.AddStep(step3)

	rcder := record.NewRecord("test-0", "", 3)
	err := stage.Run("context", rcder)

	if err != nil {
		t.Errorf("Run() error = %v, want nil", err)
	}

	if !step1.runCalled || !step2.runCalled || !step3.runCalled {
		t.Error("Not all steps were called")
	}
}

// Test Serial Run - Step Failure
func TestSerialRun_StepFailure(t *testing.T) {
	step1 := &MockSteper{stepsCount: 0}
	step2 := &MockSteper{
		stepsCount: 0,
		runFunc: func(ctx string, rcder *record.Record) error {
			rcder.Status = record.StatusFailed
			return errors.New("step failed")
		},
	}
	step3 := &MockSteper{stepsCount: 0}

	stage := NewStage("test", "", "serial", step1)
	stage.AddStep(step2)
	stage.AddStep(step3)

	rcder := record.NewRecord("test-0", "", 3)
	err := stage.Run("context", rcder)

	if err == nil {
		t.Error("Run() should return error when step fails")
	}

	if step3.runCalled {
		t.Error("Step 3 should not be called after step 2 fails")
	}
}

// Test Serial Run - Nil Record
func TestSerialRun_NilRecord(t *testing.T) {
	step := &MockSteper{stepsCount: 0}
	stage := NewStage("test", "", "serial", step)

	err := stage.Run("context", nil)
	if err == nil {
		t.Error("Run() should return error for nil record")
	}
	if err.Error() != "record is nil" {
		t.Errorf("Run() error = %v, want 'record is nil'", err)
	}
}

// Test Parallel Run - Multiple Steps
func TestParallelRun_MultipleSteps(t *testing.T) {
	step1 := &MockSteper{stepsCount: 0}
	step2 := &MockSteper{stepsCount: 0}
	step3 := &MockSteper{stepsCount: 0}

	stage := NewStage("test", "", "parallel", step1)
	stage.AddStep(step2)
	stage.AddStep(step3)

	rcder := record.NewRecord("test-0", "", 3)
	err := stage.Run("context", rcder)

	if err != nil {
		t.Errorf("Run() error = %v, want nil", err)
	}

	// All steps should be called (eventually, after goroutines complete)
	if !step1.runCalled || !step2.runCalled || !step3.runCalled {
		t.Error("Not all steps were called in parallel execution")
	}
}

// Test Parallel Run - With Async Step
func TestParallelRun_AsyncStep(t *testing.T) {
	step := &MockSteper{
		isAsync:    true,
		stepsCount: 0,
	}
	stage := NewStage("test", "", "parallel", step)
	rcder := record.NewRecord("test-0", "", 1)

	err := stage.Run("context", rcder)
	if err != nil {
		t.Errorf("Run() error = %v, want nil", err)
	}

	if rcder.Status != record.StatusAsyncWaiting {
		t.Errorf("Record status = %v, want %v for async step",
			rcder.Status, record.StatusAsyncWaiting)
	}
}

// Test Parallel Run - Step Failure
func TestParallelRun_StepFailure(t *testing.T) {
	step1 := &MockSteper{stepsCount: 0}
	step2 := &MockSteper{
		stepsCount: 0,
		runFunc: func(ctx string, rcder *record.Record) error {
			rcder.Status = record.StatusFailed
			return errors.New("step failed")
		},
	}

	stage := NewStage("test", "", "parallel", step1)
	stage.AddStep(step2)

	rcder := record.NewRecord("test-0", "", 2)
	err := stage.Run("context", rcder)

	if err == nil {
		t.Error("Run() should return error when a step fails")
	}

	if rcder.Status != record.StatusFailed {
		t.Errorf("Record status = %v, want %v", rcder.Status, record.StatusFailed)
	}
}

// Test AsyncHandler - Serial
func TestAsyncHandler_Serial(t *testing.T) {
	step := &MockSteper{
		isAsync:    true,
		stepsCount: 0,
	}
	stage := NewStage("test", "", "serial", step)
	rcder := record.NewRecord("test-0", "", 1)
	rcder.AddRecord(0, record.NewRecord("test-0", "0", 0))

	stage.AsyncHandler("response", "test-0-1", []int{0}, 0, rcder)

	if !step.asyncCalled {
		t.Error("Step.AsyncHandler() was not called")
	}
}

// Test AsyncHandler - Parallel
func TestAsyncHandler_Parallel(t *testing.T) {
	step := &MockSteper{
		isAsync:    true,
		stepsCount: 0,
	}
	stage := NewStage("test", "", "parallel", step)
	rcder := record.NewRecord("test-0", "", 1)
	rcder.AddRecord(0, record.NewRecord("test-0", "0", 0))

	stage.AsyncHandler("response", "test-0-1", []int{0}, 0, rcder)

	if !step.asyncCalled {
		t.Error("Step.AsyncHandler() was not called")
	}
}

// Benchmark tests
func BenchmarkNewStage(b *testing.B) {
	step := &MockSteper{stepsCount: 3}
	for i := 0; i < b.N; i++ {
		NewStage("test-stage", "id", "serial", step)
	}
}

func BenchmarkSerialRun(b *testing.B) {
	step := &MockSteper{stepsCount: 0}
	stage := NewStage("test", "", "serial", step)
	stage.AddStep(&MockSteper{stepsCount: 0})
	stage.AddStep(&MockSteper{stepsCount: 0})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rcder := record.NewRecord("test-0", "", 3)
		stage.Run("context", rcder)
	}
}

func BenchmarkParallelRun(b *testing.B) {
	step := &MockSteper{stepsCount: 0}
	stage := NewStage("test", "", "parallel", step)
	stage.AddStep(&MockSteper{stepsCount: 0})
	stage.AddStep(&MockSteper{stepsCount: 0})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rcder := record.NewRecord("test-0", "", 3)
		stage.Run("context", rcder)
	}
}
