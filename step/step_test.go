package step

import (
	"errors"
	"testing"
	"time"
	"workflow/record"
)

// Mock Actioner implementations for testing

// MockSuccessActioner always succeeds
type MockSuccessActioner struct {
	stepCalled  bool
	asyncCalled bool
}

func (m *MockSuccessActioner) StepActor() error {
	m.stepCalled = true
	return nil
}

func (m *MockSuccessActioner) AsyncHandler(resp string) error {
	m.asyncCalled = true
	return nil
}

// MockFailActioner always fails
type MockFailActioner struct {
	stepError  error
	asyncError error
}

func (m *MockFailActioner) StepActor() error {
	return m.stepError
}

func (m *MockFailActioner) AsyncHandler(resp string) error {
	return m.asyncError
}

// MockConfigurableActioner allows configuration of behavior
type MockConfigurableActioner struct {
	stepFunc  func() error
	asyncFunc func(string) error
}

func (m *MockConfigurableActioner) StepActor() error {
	if m.stepFunc != nil {
		return m.stepFunc()
	}
	return nil
}

func (m *MockConfigurableActioner) AsyncHandler(resp string) error {
	if m.asyncFunc != nil {
		return m.asyncFunc(resp)
	}
	return nil
}

// Tests

func TestNewStep(t *testing.T) {
	tests := []struct {
		name        string
		stepName    string
		description string
		actor       Actioner
		wantNil     bool
	}{
		{
			name:        "valid step creation",
			stepName:    "test-step",
			description: "Test Step Description",
			actor:       &MockSuccessActioner{},
			wantNil:     false,
		},
		{
			name:        "nil actor",
			stepName:    "test-step",
			description: "Test Step",
			actor:       nil,
			wantNil:     true,
		},
		{
			name:        "empty name and description",
			stepName:    "",
			description: "",
			actor:       &MockSuccessActioner{},
			wantNil:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			step := NewStep(tt.stepName, tt.description, tt.actor)
			if tt.wantNil {
				if step != nil {
					t.Errorf("NewStep() should return nil for nil actor, got %v", step)
				}
			} else {
				if step == nil {
					t.Fatal("NewStep() returned nil unexpectedly")
				}
				if step.name != tt.stepName {
					t.Errorf("NewStep() name = %v, want %v", step.name, tt.stepName)
				}
				if step.description != tt.description {
					t.Errorf("NewStep() description = %v, want %v", step.description, tt.description)
				}
				if step.execute == nil {
					t.Error("NewStep() execute should not be nil")
				}
			}
		})
	}
}

func TestIsAsync(t *testing.T) {
	tests := []struct {
		name    string
		isAsync bool
		want    bool
	}{
		{
			name:    "synchronous step",
			isAsync: false,
			want:    false,
		},
		{
			name:    "asynchronous step",
			isAsync: true,
			want:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			step := &Step{
				name:    "test",
				isAsync: tt.isAsync,
				execute: &MockSuccessActioner{},
			}
			if got := step.IsAsync(); got != tt.want {
				t.Errorf("IsAsync() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStepRun_Success_Sync(t *testing.T) {
	actor := &MockSuccessActioner{}
	step := NewStep("test-step", "Test", actor)
	step.isAsync = false

	rcder := record.NewRecord("test-0", "", 0)
	startTime := time.Now().UnixMilli()

	err := step.Run("ctx", rcder)

	if err != nil {
		t.Errorf("Run() error = %v, want nil", err)
	}
	if !actor.stepCalled {
		t.Error("StepActor was not called")
	}
	if rcder.Status != record.StatusDone {
		t.Errorf("Record status = %v, want %v", rcder.Status, record.StatusDone)
	}
	if rcder.StartAt < startTime {
		t.Error("StartAt not set correctly")
	}
	if rcder.EndAt < rcder.StartAt {
		t.Error("EndAt should be >= StartAt")
	}
}

func TestStepRun_Success_Async(t *testing.T) {
	actor := &MockSuccessActioner{}
	step := NewStep("test-step", "Test", actor)
	step.isAsync = true

	rcder := record.NewRecord("test-0", "", 0)

	err := step.Run("ctx", rcder)

	if err != nil {
		t.Errorf("Run() error = %v, want nil", err)
	}
	if !actor.stepCalled {
		t.Error("StepActor was not called")
	}
	if rcder.Status != record.StatusAsyncWaiting {
		t.Errorf("Record status = %v, want %v", rcder.Status, record.StatusAsyncWaiting)
	}
}

func TestStepRun_Failure(t *testing.T) {
	expectedErr := errors.New("step execution failed")
	actor := &MockFailActioner{
		stepError: expectedErr,
	}
	step := NewStep("test-step", "Test", actor)
	step.isAsync = false

	rcder := record.NewRecord("test-0", "", 0)

	err := step.Run("ctx", rcder)

	if err == nil {
		t.Error("Run() should return error")
	}
	if err != expectedErr {
		t.Errorf("Run() error = %v, want %v", err, expectedErr)
	}
	if rcder.Status != record.StatusFailed {
		t.Errorf("Record status = %v, want %v", rcder.Status, record.StatusFailed)
	}
}

func TestStepRun_NilRecord(t *testing.T) {
	actor := &MockSuccessActioner{}
	step := NewStep("test-step", "Test", actor)

	err := step.Run("ctx", nil)

	if err == nil {
		t.Error("Run() should return error for nil record")
	}
	if err.Error() != "record is nil" {
		t.Errorf("Run() error = %v, want 'record is nil'", err)
	}
	if actor.stepCalled {
		t.Error("StepActor should not be called when record is nil")
	}
}

func TestStepRun_TimestampOrder(t *testing.T) {
	// Use a slow actor to ensure time passes
	actor := &MockConfigurableActioner{
		stepFunc: func() error {
			time.Sleep(10 * time.Millisecond)
			return nil
		},
	}
	step := NewStep("test-step", "Test", actor)

	rcder := record.NewRecord("test-0", "", 0)
	beforeRun := time.Now().UnixMilli()

	err := step.Run("ctx", rcder)

	afterRun := time.Now().UnixMilli()

	if err != nil {
		t.Errorf("Run() error = %v, want nil", err)
	}
	if rcder.StartAt < beforeRun {
		t.Error("StartAt should be >= time before run")
	}
	if rcder.EndAt > afterRun {
		t.Error("EndAt should be <= time after run")
	}
	if rcder.EndAt < rcder.StartAt {
		t.Error("EndAt should be >= StartAt")
	}
}

func TestAsyncHandler_Success(t *testing.T) {
	actor := &MockSuccessActioner{}
	step := NewStep("test-step", "Test", actor)
	step.isAsync = true

	rcder := record.NewRecord("test-0", "", 0)
	ids := []int{0, 1, 2}

	step.AsyncHandler("response-data", "test-0-1", ids, 2, rcder)

	if !actor.asyncCalled {
		t.Error("AsyncHandler of actor was not called")
	}
	if rcder.AsyncRecord == nil {
		t.Error("AsyncRecord should be created")
	}
	if rcder.AsyncRecord.Status != record.StatusDone {
		t.Errorf("AsyncRecord status = %v, want %v", rcder.AsyncRecord.Status, record.StatusDone)
	}
}

func TestAsyncHandler_Failure(t *testing.T) {
	expectedErr := errors.New("async handler failed")
	actor := &MockFailActioner{
		asyncError: expectedErr,
	}
	step := NewStep("test-step", "Test", actor)
	step.isAsync = true

	rcder := record.NewRecord("test-0", "", 0)
	ids := []int{0, 1, 2}

	step.AsyncHandler("response-data", "test-0-1", ids, 2, rcder)

	if rcder.AsyncRecord == nil {
		t.Error("AsyncRecord should be created even on failure")
	}
	if rcder.AsyncRecord.Status != record.StatusFailed {
		t.Errorf("AsyncRecord status = %v, want %v", rcder.AsyncRecord.Status, record.StatusFailed)
	}
}

func TestAsyncHandler_NilRecord(t *testing.T) {
	actor := &MockSuccessActioner{}
	step := NewStep("test-step", "Test", actor)
	step.isAsync = true

	// Should not panic
	step.AsyncHandler("response", "test-0-1", []int{0}, 1, nil)

	if actor.asyncCalled {
		t.Error("AsyncHandler should not be called when record is nil")
	}
}

func TestAsyncHandler_TerminationCondition(t *testing.T) {
	actor := &MockSuccessActioner{}
	step := NewStep("test-step", "Test", actor)
	step.isAsync = true

	rcder := record.NewRecord("test-0", "", 0)
	ids := []int{0, 1}

	// stageIndex >= len(ids) should terminate early
	step.AsyncHandler("response", "test-0-1", ids, 2, rcder)

	// AsyncRecord should NOT be created because early termination happens before creation
	if rcder.AsyncRecord != nil {
		t.Error("AsyncRecord should not be created due to early termination")
	}
	if actor.asyncCalled {
		t.Error("AsyncHandler should not be called due to early termination")
	}
}

func TestAsyncHandler_ExactTerminationBoundary(t *testing.T) {
	actor := &MockSuccessActioner{}
	step := NewStep("test-step", "Test", actor)
	step.isAsync = true

	rcder := record.NewRecord("test-0", "", 0)
	ids := []int{0}

	// stageIndex = 1, len(ids) = 1, so 1 >= 1 is true - early termination
	step.AsyncHandler("response", "test-0-1", ids, 1, rcder)

	if rcder.AsyncRecord != nil {
		t.Error("AsyncRecord should not be created due to early termination at boundary")
	}
	if actor.asyncCalled {
		t.Error("AsyncHandler should not be called at termination boundary")
	}
}

func TestAsyncHandler_Timestamps(t *testing.T) {
	actor := &MockConfigurableActioner{
		asyncFunc: func(resp string) error {
			time.Sleep(10 * time.Millisecond)
			return nil
		},
	}
	step := NewStep("test-step", "Test", actor)
	step.isAsync = true

	rcder := record.NewRecord("test-0", "", 0)
	ids := []int{0, 1, 2}

	beforeCall := time.Now().UnixMilli()
	step.AsyncHandler("response", "test-0-1", ids, 2, rcder)
	afterCall := time.Now().UnixMilli()

	if rcder.AsyncRecord == nil {
		t.Fatal("AsyncRecord should be created")
	}
	if rcder.AsyncRecord.StartAt < beforeCall {
		t.Error("AsyncRecord StartAt should be >= time before call")
	}
	if rcder.AsyncRecord.EndAt > afterCall {
		t.Error("AsyncRecord EndAt should be <= time after call")
	}
	if rcder.AsyncRecord.EndAt < rcder.AsyncRecord.StartAt {
		t.Error("AsyncRecord EndAt should be >= StartAt")
	}
}

func TestAsyncHandler_ResponsePassed(t *testing.T) {
	var receivedResp string
	actor := &MockConfigurableActioner{
		asyncFunc: func(resp string) error {
			receivedResp = resp
			return nil
		},
	}
	step := NewStep("test-step", "Test", actor)
	step.isAsync = true

	rcder := record.NewRecord("test-0", "", 0)
	expectedResp := "test-response-data"

	step.AsyncHandler(expectedResp, "test-0-1", []int{0, 1, 2}, 2, rcder)

	if receivedResp != expectedResp {
		t.Errorf("AsyncHandler received response = %v, want %v", receivedResp, expectedResp)
	}
}

func TestStepsCount(t *testing.T) {
	actor := &MockSuccessActioner{}
	step := NewStep("test-step", "Test", actor)

	count := step.StepsCount()
	if count != 0 {
		t.Errorf("StepsCount() = %v, want 0", count)
	}
}

func TestStepFields(t *testing.T) {
	actor := &MockSuccessActioner{}
	name := "test-step"
	description := "Test Description"

	step := NewStep(name, description, actor)

	if step.name != name {
		t.Errorf("step.name = %v, want %v", step.name, name)
	}
	if step.description != description {
		t.Errorf("step.description = %v, want %v", step.description, description)
	}
	if step.execute != actor {
		t.Error("step.execute should be the provided actor")
	}
	// Default isAsync should be false
	if step.isAsync != false {
		t.Error("default isAsync should be false")
	}
}

func TestStepRun_StatusTransitions(t *testing.T) {
	tests := []struct {
		name       string
		isAsync    bool
		actorError error
		wantStatus string
		wantError  bool
	}{
		{
			name:       "sync success",
			isAsync:    false,
			actorError: nil,
			wantStatus: record.StatusDone,
			wantError:  false,
		},
		{
			name:       "async success",
			isAsync:    true,
			actorError: nil,
			wantStatus: record.StatusAsyncWaiting,
			wantError:  false,
		},
		{
			name:       "sync failure",
			isAsync:    false,
			actorError: errors.New("sync error"),
			wantStatus: record.StatusFailed,
			wantError:  true,
		},
		{
			name:       "async failure",
			isAsync:    true,
			actorError: errors.New("async error"),
			wantStatus: record.StatusFailed,
			wantError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actor := &MockFailActioner{stepError: tt.actorError}
			step := NewStep("test", "test", actor)
			step.isAsync = tt.isAsync

			rcder := record.NewRecord("test-0", "", 0)
			err := step.Run("ctx", rcder)

			if tt.wantError && err == nil {
				t.Error("expected error but got nil")
			}
			if !tt.wantError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if rcder.Status != tt.wantStatus {
				t.Errorf("status = %v, want %v", rcder.Status, tt.wantStatus)
			}
		})
	}
}

// Benchmark tests

func BenchmarkNewStep(b *testing.B) {
	actor := &MockSuccessActioner{}
	for i := 0; i < b.N; i++ {
		NewStep("test-step", "description", actor)
	}
}

func BenchmarkStepRun_Success(b *testing.B) {
	actor := &MockSuccessActioner{}
	step := NewStep("test-step", "test", actor)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rcder := record.NewRecord("test-0", "", 0)
		step.Run("ctx", rcder)
	}
}

func BenchmarkStepRun_WithError(b *testing.B) {
	actor := &MockFailActioner{stepError: errors.New("error")}
	step := NewStep("test-step", "test", actor)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rcder := record.NewRecord("test-0", "", 0)
		step.Run("ctx", rcder)
	}
}

func BenchmarkAsyncHandler(b *testing.B) {
	actor := &MockSuccessActioner{}
	step := NewStep("test-step", "test", actor)
	step.isAsync = true

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rcder := record.NewRecord("test-0", "", 0)
		step.AsyncHandler("response", "test-0-1", []int{0, 1, 2}, 2, rcder)
	}
}
