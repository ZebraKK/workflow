package stage

import (
	"errors"
	"log/slog"
	"testing"
	"time"

	"workflow/logger"
	"workflow/record"
	"workflow/step"
)

// Mock Actioner for testing
type MockActioner struct {
	handleFunc  func(ctx interface{}) error
	handleError error
}

func (m *MockActioner) Handle(ctx interface{}) error {
	if m.handleFunc != nil {
		return m.handleFunc(ctx)
	}
	return m.handleError
}

// Mock AsyncActioner for testing
type MockAsyncActioner struct {
	asyncHandleFunc  func(ctx interface{}, resp interface{}) error
	asyncHandleError error
}

func (m *MockAsyncActioner) AsyncHandle(ctx interface{}, resp interface{}) error {
	if m.asyncHandleFunc != nil {
		return m.asyncHandleFunc(ctx, resp)
	}
	return m.asyncHandleError
}

var noOpLogger = logger.NewNoOpLogger()
var slogger = logger.NewSlogLogger(slog.LevelDebug)

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
			step:      step.NewStep("step1", "Test Step", 5*time.Second, &MockActioner{}, nil),
			wantNil:   false,
			wantID:    "custom-id",
		},
		{
			name:      "valid stage without ID",
			stageName: "test-stage",
			id:        "",
			mode:      "parallel",
			step:      step.NewStep("step2", "Test Step 2", 5*time.Second, &MockActioner{}, nil),
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
	stp1 := step.NewStep("step1", "Step 1", 5*time.Second, &MockActioner{}, nil)
	stage := NewStage("test", "id", "serial", stp1)

	if len(stage.Steps) != 1 {
		t.Errorf("Initial steps count = %d, want 1", len(stage.Steps))
	}

	stp2 := step.NewStep("step2", "Step 2", 5*time.Second, &MockActioner{}, nil)
	stage.AddStep(stp2)
	if len(stage.Steps) != 2 {
		t.Errorf("After adding step, count = %d, want 2", len(stage.Steps))
	}

	if stage.IsAsync() {
		t.Error("Stage should not be async")
	}

	stp3 := step.NewStep("step3", "Async Step", 5*time.Second, &MockActioner{}, &MockAsyncActioner{})
	stage.AddStep(stp3)
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
			step: step.NewStep("step1", "Sync Step", 5*time.Second, &MockActioner{}, nil),
			want: false,
		},
		{
			name: "async step",
			step: step.NewStep("step2", "Async Step", 5*time.Second, &MockActioner{}, &MockAsyncActioner{}),
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
	stp1 := step.NewStep("step1", "Step 1", 5*time.Second, &MockActioner{}, nil)
	stage := NewStage("test", "", "serial", stp1)

	if stage.StepsCount() != 1 {
		t.Errorf("StepsCount() = %d, want 1", stage.StepsCount())
	}

	stp2 := step.NewStep("step2", "Step 2", 5*time.Second, &MockActioner{}, nil)
	stp3 := step.NewStep("step3", "Step 3", 5*time.Second, &MockActioner{}, nil)
	stage.AddStep(stp2)
	stage.AddStep(stp3)

	if stage.StepsCount() != 3 {
		t.Errorf("StepsCount() = %d, want 3", stage.StepsCount())
	}
}

// Test GetName and GetID
func TestGetters(t *testing.T) {
	stp := step.NewStep("step1", "Step 1", 5*time.Second, &MockActioner{}, nil)
	stage := NewStage("my-stage", "my-id", "serial", stp)

	if stage.GetName() != "my-stage" {
		t.Errorf("GetName() = %v, want my-stage", stage.GetName())
	}

	if stage.GetID() != "my-id" {
		t.Errorf("GetID() = %v, want my-id", stage.GetID())
	}
}

// Test Handle - Serial Mode
func TestHandle_Serial(t *testing.T) {
	actor := &MockActioner{}
	stp := step.NewStep("step1", "Step 1", 5*time.Second, actor, nil)
	stage := NewStage("test", "", "serial", stp)
	rcder := record.NewRecord("test-0", "", 1)

	err := stage.Handle("context", rcder, noOpLogger)
	if err != nil {
		t.Errorf("Handle() error = %v, want nil", err)
	}

	if rcder.Status != record.StatusDone {
		t.Errorf("Record status = %v, want %v", rcder.Status, record.StatusDone)
	}
}

// Test Handle - Parallel Mode
func TestHandle_Parallel(t *testing.T) {
	actor := &MockActioner{}
	stp := step.NewStep("step1", "Step 1", 5*time.Second, actor, nil)
	stage := NewStage("test", "", "parallel", stp)
	rcder := record.NewRecord("test-0", "", 1)

	err := stage.Handle("context", rcder, noOpLogger)
	if err != nil {
		t.Errorf("Handle() error = %v, want nil", err)
	}
}

// Test Handle - Unknown Mode
func TestHandle_UnknownMode(t *testing.T) {
	actor := &MockActioner{}
	stp := step.NewStep("step1", "Step 1", 5*time.Second, actor, nil)
	stage := NewStage("test", "", "unknown", stp)
	rcder := record.NewRecord("test-0", "", 1)

	err := stage.Handle("context", rcder, noOpLogger)
	if err != nil {
		t.Errorf("Handle() with unknown mode should not error, got %v", err)
	}
}

// Test Serial Handle - Multiple Steps
func TestSerialHandle_MultipleSteps(t *testing.T) {
	actor1 := &MockActioner{}
	actor2 := &MockActioner{}
	actor3 := &MockActioner{}

	stp1 := step.NewStep("step1", "Step 1", 5*time.Second, actor1, nil)
	stp2 := step.NewStep("step2", "Step 2", 5*time.Second, actor2, nil)
	stp3 := step.NewStep("step3", "Step 3", 5*time.Second, actor3, nil)

	stage := NewStage("test", "", "serial", stp1)
	stage.AddStep(stp2)
	stage.AddStep(stp3)

	rcder := record.NewRecord("test-0", "", 3)
	err := stage.Handle("context", rcder, noOpLogger)

	if err != nil {
		t.Errorf("Handle() error = %v, want nil", err)
	}

	if rcder.Status != record.StatusDone {
		t.Errorf("Record status = %v, want %v", rcder.Status, record.StatusDone)
	}
}

// Test Serial Handle - Step Failure
func TestSerialHandle_StepFailure(t *testing.T) {
	actor1 := &MockActioner{}
	actor2 := &MockActioner{handleError: errors.New("step failed")}
	actor3 := &MockActioner{}

	stp1 := step.NewStep("step1", "Step 1", 5*time.Second, actor1, nil)
	stp2 := step.NewStep("step2", "Step 2", 5*time.Second, actor2, nil)
	stp3 := step.NewStep("step3", "Step 3", 5*time.Second, actor3, nil)

	stage := NewStage("test", "", "serial", stp1)
	stage.AddStep(stp2)
	stage.AddStep(stp3)

	rcder := record.NewRecord("test-0", "", 3)
	err := stage.Handle("context", rcder, noOpLogger)

	if err == nil {
		t.Error("Handle() should return error when step fails")
	}

	if rcder.Status != record.StatusFailed {
		t.Errorf("Record status = %v, want %v", rcder.Status, record.StatusFailed)
	}
}

// Test Serial Handle - Nil Record
func TestSerialHandle_NilRecord(t *testing.T) {
	actor := &MockActioner{}
	stp := step.NewStep("step1", "Step 1", 5*time.Second, actor, nil)
	stage := NewStage("test", "", "serial", stp)

	err := stage.Handle("context", nil, noOpLogger)
	if err == nil {
		t.Error("Handle() should return error for nil record")
	}
	if err.Error() != "record is nil" {
		t.Errorf("Handle() error = %v, want 'record is nil'", err)
	}
}

// Test Parallel Handle - Multiple Steps
func TestParallelHandle_MultipleSteps(t *testing.T) {
	actor1 := &MockActioner{}
	actor2 := &MockActioner{}
	actor3 := &MockActioner{}

	stp1 := step.NewStep("step1", "Step 1", 5*time.Second, actor1, nil)
	stp2 := step.NewStep("step2", "Step 2", 5*time.Second, actor2, nil)
	stp3 := step.NewStep("step3", "Step 3", 5*time.Second, actor3, nil)

	stage := NewStage("test", "", "parallel", stp1)
	stage.AddStep(stp2)
	stage.AddStep(stp3)

	rcder := record.NewRecord("test-0", "", 3)
	err := stage.Handle("context", rcder, noOpLogger)

	if err != nil {
		t.Errorf("Handle() error = %v, want nil", err)
	}

	if rcder.Status != record.StatusDone {
		t.Errorf("Record status = %v, want %v", rcder.Status, record.StatusDone)
	}
}

// Test Parallel Handle - With Async Step
func TestParallelHandle_AsyncStep(t *testing.T) {
	actor := &MockActioner{}
	asyncActor := &MockAsyncActioner{}
	stp := step.NewStep("step1", "Async Step", 5*time.Second, actor, asyncActor)
	stage := NewStage("test", "", "parallel", stp)
	rcder := record.NewRecord("test-0", "", 1)

	err := stage.Handle("context", rcder, noOpLogger)
	if err != nil {
		t.Errorf("Handle() error = %v, want nil", err)
	}

	if rcder.Status != record.StatusAsyncWaiting {
		t.Errorf("Record status = %v, want %v for async step",
			rcder.Status, record.StatusAsyncWaiting)
	}
}

// Test Parallel Handle - Step Failure
func TestParallelHandle_StepFailure(t *testing.T) {
	actor1 := &MockActioner{}
	actor2 := &MockActioner{handleError: errors.New("step failed")}

	stp1 := step.NewStep("step1", "Step 1", 5*time.Second, actor1, nil)
	stp2 := step.NewStep("step2", "Step 2", 5*time.Second, actor2, nil)

	stage := NewStage("test", "", "parallel", stp1)
	stage.AddStep(stp2)

	rcder := record.NewRecord("test-0", "", 2)
	err := stage.Handle("context", rcder, noOpLogger)

	if err == nil {
		t.Error("Handle() should return error when a step fails")
	}

	if rcder.Status != record.StatusFailed {
		t.Errorf("Record status = %v, want %v", rcder.Status, record.StatusFailed)
	}
}

// Test AsyncHandle - Serial
func TestAsyncHandle_Serial(t *testing.T) {
	actor := &MockActioner{}
	asyncActor := &MockAsyncActioner{}
	stp := step.NewStep("step1", "Async Step", 5*time.Second, actor, asyncActor)
	stage := NewStage("test", "", "serial", stp)
	rcder := record.NewRecord("test-0", "", 1)
	rcder.AddRecord(0, record.NewRecord("test-0", "0", 0))

	stage.AsyncHandle("context", "response", "test-0-1", []int{0}, 0, rcder, noOpLogger)

	// AsyncHandle should update the async record status
	if rcder.Records[0].AsyncRecord == nil {
		t.Error("AsyncRecord should be created")
	}
}

// Test AsyncHandle - Parallel
func TestAsyncHandle_Parallel(t *testing.T) {
	actor := &MockActioner{}
	asyncActor := &MockAsyncActioner{}
	stp := step.NewStep("step1", "Async Step", 5*time.Second, actor, asyncActor)
	stage := NewStage("test", "", "parallel", stp)
	rcder := record.NewRecord("test-0", "", 1)
	rcder.AddRecord(0, record.NewRecord("test-0", "0", 0))

	stage.AsyncHandle("context", "response", "test-0-1", []int{0}, 0, rcder, slogger)

	// AsyncHandle should update the async record status
	if rcder.Records[0].AsyncRecord == nil {
		t.Error("AsyncRecord should be created")
	}
}

// Benchmark tests
func BenchmarkNewStage(b *testing.B) {
	actor := &MockActioner{}
	stp := step.NewStep("step1", "Step 1", 5*time.Second, actor, nil)
	for i := 0; i < b.N; i++ {
		NewStage("test-stage", "id", "serial", stp)
	}
}

func BenchmarkSerialHandle(b *testing.B) {
	actor1 := &MockActioner{}
	actor2 := &MockActioner{}
	actor3 := &MockActioner{}

	stp1 := step.NewStep("step1", "Step 1", 5*time.Second, actor1, nil)
	stp2 := step.NewStep("step2", "Step 2", 5*time.Second, actor2, nil)
	stp3 := step.NewStep("step3", "Step 3", 5*time.Second, actor3, nil)

	stage := NewStage("test", "", "serial", stp1)
	stage.AddStep(stp2)
	stage.AddStep(stp3)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rcder := record.NewRecord("test-0", "", 3)
		stage.Handle("context", rcder, noOpLogger)
	}
}

func BenchmarkParallelHandle(b *testing.B) {
	actor1 := &MockActioner{}
	actor2 := &MockActioner{}
	actor3 := &MockActioner{}

	stp1 := step.NewStep("step1", "Step 1", 5*time.Second, actor1, nil)
	stp2 := step.NewStep("step2", "Step 2", 5*time.Second, actor2, nil)
	stp3 := step.NewStep("step3", "Step 3", 5*time.Second, actor3, nil)

	stage := NewStage("test", "", "parallel", stp1)
	stage.AddStep(stp2)
	stage.AddStep(stp3)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rcder := record.NewRecord("test-0", "", 3)
		stage.Handle("context", rcder, noOpLogger)
	}
}
