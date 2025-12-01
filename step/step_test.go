package step

import (
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"workflow/logger"
	"workflow/record"
)

// Mock Actioner implementations for testing

// MockSuccessActioner always succeeds
type MockSuccessActioner struct {
	handleCalled bool
	lastCtx      interface{}
}

func (m *MockSuccessActioner) Handle(ctx interface{}) error {
	m.handleCalled = true
	m.lastCtx = ctx
	return nil
}

// MockSuccessAsyncActioner always succeeds
type MockSuccessAsyncActioner struct {
	asyncHandleCalled bool
	lastCtx           interface{}
	lastResp          interface{}
}

func (m *MockSuccessAsyncActioner) AsyncHandle(ctx interface{}, resp interface{}) error {
	m.asyncHandleCalled = true
	m.lastCtx = ctx
	m.lastResp = resp
	return nil
}

// MockFailActioner always fails
type MockFailActioner struct {
	handleError error
}

func (m *MockFailActioner) Handle(ctx interface{}) error {
	return m.handleError
}

// MockFailAsyncActioner always fails
type MockFailAsyncActioner struct {
	asyncHandleError error
}

func (m *MockFailAsyncActioner) AsyncHandle(ctx interface{}, resp interface{}) error {
	return m.asyncHandleError
}

// MockSlowActioner simulates slow execution
type MockSlowActioner struct {
	delay time.Duration
}

func (m *MockSlowActioner) Handle(ctx interface{}) error {
	time.Sleep(m.delay)
	return nil
}

// MockConfigurableActioner allows configuration of behavior
type MockConfigurableActioner struct {
	handleFunc func(ctx interface{}) error
}

func (m *MockConfigurableActioner) Handle(ctx interface{}) error {
	if m.handleFunc != nil {
		return m.handleFunc(ctx)
	}
	return nil
}

// MockConfigurableAsyncActioner allows configuration of async behavior
type MockConfigurableAsyncActioner struct {
	asyncHandleFunc func(ctx interface{}, resp interface{}) error
}

func (m *MockConfigurableAsyncActioner) AsyncHandle(ctx interface{}, resp interface{}) error {
	if m.asyncHandleFunc != nil {
		return m.asyncHandleFunc(ctx, resp)
	}
	return nil
}

// Tests

func TestNewStep(t *testing.T) {
	tests := []struct {
		name        string
		stepName    string
		description string
		timeout     time.Duration
		actor       Actioner
		asyncActor  AsyncActioner
		wantNil     bool
	}{
		{
			name:        "valid sync step",
			stepName:    "test-step",
			description: "Test Step",
			timeout:     5 * time.Second,
			actor:       &MockSuccessActioner{},
			asyncActor:  nil,
			wantNil:     false,
		},
		{
			name:        "valid async step",
			stepName:    "test-async-step",
			description: "Test Async Step",
			timeout:     10 * time.Second,
			actor:       &MockSuccessActioner{},
			asyncActor:  &MockSuccessAsyncActioner{},
			wantNil:     false,
		},
		{
			name:        "nil actor",
			stepName:    "test-step",
			description: "Test",
			timeout:     5 * time.Second,
			actor:       nil,
			asyncActor:  nil,
			wantNil:     true,
		},
		{
			name:        "zero timeout gets default",
			stepName:    "test-step",
			description: "Test",
			timeout:     0,
			actor:       &MockSuccessActioner{},
			asyncActor:  nil,
			wantNil:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			step := NewStep(tt.stepName, tt.description, tt.timeout, tt.actor, tt.asyncActor)
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
			}
		})
	}
}

func TestIsAsync(t *testing.T) {
	tests := []struct {
		name       string
		asyncActor AsyncActioner
		want       bool
	}{
		{
			name:       "synchronous step",
			asyncActor: nil,
			want:       false,
		},
		{
			name:       "asynchronous step",
			asyncActor: &MockSuccessAsyncActioner{},
			want:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			step := NewStep("test", "test", 5*time.Second, &MockSuccessActioner{}, tt.asyncActor)
			if got := step.IsAsync(); got != tt.want {
				t.Errorf("IsAsync() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStepHandle_Success_Sync(t *testing.T) {
	actor := &MockSuccessActioner{}
	step := NewStep("test-step", "Test", 5*time.Second, actor, nil)

	rcder := record.NewRecord("test-0", "", 0)
	testLogger := logger.NewNoOpLogger()
	startTime := time.Now().UnixMilli()

	err := step.Handle("test-ctx", rcder, testLogger)

	if err != nil {
		t.Errorf("Handle() error = %v, want nil", err)
	}
	if !actor.handleCalled {
		t.Error("Handle was not called")
	}
	if actor.lastCtx != "test-ctx" {
		t.Errorf("Context not passed correctly, got %v", actor.lastCtx)
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

func TestStepHandle_Success_Async(t *testing.T) {
	actor := &MockSuccessActioner{}
	asyncActor := &MockSuccessAsyncActioner{}
	step := NewStep("test-step", "Test", 5*time.Second, actor, asyncActor)

	rcder := record.NewRecord("test-0", "", 0)
	testLogger := logger.NewNoOpLogger()

	err := step.Handle("test-ctx", rcder, testLogger)

	if err != nil {
		t.Errorf("Handle() error = %v, want nil", err)
	}
	if !actor.handleCalled {
		t.Error("Handle was not called")
	}
	if rcder.Status != record.StatusAsyncWaiting {
		t.Errorf("Record status = %v, want %v", rcder.Status, record.StatusAsyncWaiting)
	}
}

func TestStepHandle_Failure(t *testing.T) {
	expectedErr := errors.New("step execution failed")
	actor := &MockFailActioner{handleError: expectedErr}
	step := NewStep("test-step", "Test", 5*time.Second, actor, nil)

	rcder := record.NewRecord("test-0", "", 0)
	testLogger := logger.NewNoOpLogger()

	err := step.Handle("test-ctx", rcder, testLogger)

	if err == nil {
		t.Error("Handle() should return error")
	}
	if !strings.Contains(err.Error(), "step execution failed") {
		t.Errorf("Handle() error = %v, want error containing 'step execution failed'", err)
	}
	if rcder.Status != record.StatusFailed {
		t.Errorf("Record status = %v, want %v", rcder.Status, record.StatusFailed)
	}
}

func TestStepHandle_NilRecord(t *testing.T) {
	actor := &MockSuccessActioner{}
	step := NewStep("test-step", "Test", 5*time.Second, actor, nil)
	testLogger := logger.NewNoOpLogger()

	err := step.Handle("test-ctx", nil, testLogger)

	if err == nil {
		t.Error("Handle() should return error for nil record")
	}
	if err.Error() != "record is nil" {
		t.Errorf("Handle() error = %v, want 'record is nil'", err)
	}
	if actor.handleCalled {
		t.Error("Handle should not be called when record is nil")
	}
}

func TestStepHandle_Timeout(t *testing.T) {
	// Create actor that takes longer than timeout
	actor := &MockSlowActioner{delay: 200 * time.Millisecond}
	step := NewStep("test-step", "Test", 100*time.Millisecond, actor, nil)

	rcder := record.NewRecord("test-0", "", 0)
	testLogger := logger.NewNoOpLogger()

	err := step.Handle("test-ctx", rcder, testLogger)

	if err == nil {
		t.Error("Handle() should return timeout error")
	}
	if !strings.Contains(err.Error(), "timeout") {
		t.Errorf("Handle() error = %v, should contain 'timeout'", err)
	}
	if rcder.Status != record.StatusFailed {
		t.Errorf("Record status = %v, want %v", rcder.Status, record.StatusFailed)
	}
}

func TestAsyncHandle_Success(t *testing.T) {
	actor := &MockSuccessActioner{}
	asyncActor := &MockSuccessAsyncActioner{}
	step := NewStep("test-step", "Test", 5*time.Second, actor, asyncActor)

	rcder := record.NewRecord("test-0", "", 0)
	testLogger := logger.NewNoOpLogger()
	ids := []int{0, 1, 2}

	step.AsyncHandle("test-ctx", "response-data", "test-0-1", ids, 2, rcder, testLogger)

	if !asyncActor.asyncHandleCalled {
		t.Error("AsyncHandle of actor was not called")
	}
	if asyncActor.lastCtx != "test-ctx" {
		t.Errorf("Context not passed correctly, got %v", asyncActor.lastCtx)
	}
	if asyncActor.lastResp != "response-data" {
		t.Errorf("Response not passed correctly, got %v", asyncActor.lastResp)
	}
	if rcder.AsyncRecord == nil {
		t.Error("AsyncRecord should be created")
	}
	if rcder.AsyncRecord.Status != record.StatusDone {
		t.Errorf("AsyncRecord status = %v, want %v", rcder.AsyncRecord.Status, record.StatusDone)
	}
}

func TestAsyncHandle_Failure(t *testing.T) {
	expectedErr := errors.New("async handler failed")
	actor := &MockSuccessActioner{}
	asyncActor := &MockFailAsyncActioner{asyncHandleError: expectedErr}
	step := NewStep("test-step", "Test", 5*time.Second, actor, asyncActor)

	rcder := record.NewRecord("test-0", "", 0)
	testLogger := logger.NewNoOpLogger()
	ids := []int{0, 1, 2}

	step.AsyncHandle("test-ctx", "response-data", "test-0-1", ids, 2, rcder, testLogger)

	if rcder.AsyncRecord == nil {
		t.Error("AsyncRecord should be created even on failure")
	}
	if rcder.AsyncRecord.Status != record.StatusFailed {
		t.Errorf("AsyncRecord status = %v, want %v", rcder.AsyncRecord.Status, record.StatusFailed)
	}
}

func TestAsyncHandle_NilRecord(t *testing.T) {
	actor := &MockSuccessActioner{}
	asyncActor := &MockSuccessAsyncActioner{}
	step := NewStep("test-step", "Test", 5*time.Second, actor, asyncActor)
	testLogger := logger.NewNoOpLogger()

	// Should not panic
	step.AsyncHandle("test-ctx", "response", "test-0-1", []int{0}, 1, nil, testLogger)

	if asyncActor.asyncHandleCalled {
		t.Error("AsyncHandle should not be called when record is nil")
	}
}

func TestAsyncHandle_TerminationCondition(t *testing.T) {
	actor := &MockSuccessActioner{}
	asyncActor := &MockSuccessAsyncActioner{}
	step := NewStep("test-step", "Test", 5*time.Second, actor, asyncActor)

	rcder := record.NewRecord("test-0", "", 0)
	testLogger := logger.NewSlogLogger(slog.LevelDebug)
	ids := []int{0, 1}

	// stageIndex >= len(ids) should terminate early
	step.AsyncHandle("test-ctx", "response", "test-0-1", ids, 3, rcder, testLogger)

	if rcder.AsyncRecord != nil {
		t.Error("AsyncRecord should not be created due to early termination")
	}
	if asyncActor.asyncHandleCalled {
		t.Error("AsyncHandle should not be called due to early termination")
	}
}

func TestSetTimeout(t *testing.T) {
	actor := &MockSuccessActioner{}
	step := NewStep("test-step", "Test", 5*time.Second, actor, nil)

	step.SetTimeout(10 * time.Second)

	if step.timeout != 10*time.Second {
		t.Errorf("SetTimeout() timeout = %v, want %v", step.timeout, 10*time.Second)
	}

	// Should not allow negative timeout
	step.SetTimeout(-1 * time.Second)
	if step.timeout != 10*time.Second {
		t.Error("SetTimeout() should not allow negative timeout")
	}

	// Should not allow zero timeout
	step.SetTimeout(0)
	if step.timeout != 10*time.Second {
		t.Error("SetTimeout() should not allow zero timeout")
	}
}

func TestStepsCount(t *testing.T) {
	actor := &MockSuccessActioner{}
	step := NewStep("test-step", "Test", 5*time.Second, actor, nil)

	count := step.StepsCount()
	if count != 0 {
		t.Errorf("StepsCount() = %v, want 0", count)
	}
}

func TestContextPassing(t *testing.T) {
	actor := &MockSuccessActioner{}
	step := NewStep("test-step", "Test", 5*time.Second, actor, nil)

	rcder := record.NewRecord("test-0", "", 0)
	testLogger := logger.NewNoOpLogger()

	testCtx := map[string]interface{}{
		"userID": "user123",
		"data":   "test-data",
	}

	err := step.Handle(testCtx, rcder, testLogger)

	if err != nil {
		t.Errorf("Handle() error = %v, want nil", err)
	}

	if ctxMap, ok := actor.lastCtx.(map[string]interface{}); ok {
		if ctxMap["userID"] != "user123" {
			t.Error("Context not preserved correctly")
		}
	} else {
		t.Error("Context not passed as expected type")
	}
}

// Benchmark tests

func BenchmarkNewStep(b *testing.B) {
	actor := &MockSuccessActioner{}
	for i := 0; i < b.N; i++ {
		NewStep("test-step", "description", 5*time.Second, actor, nil)
	}
}

func BenchmarkStepHandle_Success(b *testing.B) {
	actor := &MockSuccessActioner{}
	step := NewStep("test-step", "test", 5*time.Second, actor, nil)
	testLogger := logger.NewNoOpLogger()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rcder := record.NewRecord("test-0", "", 0)
		step.Handle("ctx", rcder, testLogger)
	}
}

func BenchmarkStepHandle_WithError(b *testing.B) {
	actor := &MockFailActioner{handleError: errors.New("error")}
	step := NewStep("test-step", "test", 5*time.Second, actor, nil)
	testLogger := logger.NewNoOpLogger()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rcder := record.NewRecord("test-0", "", 0)
		step.Handle("ctx", rcder, testLogger)
	}
}

func BenchmarkAsyncHandle(b *testing.B) {
	actor := &MockSuccessActioner{}
	asyncActor := &MockSuccessAsyncActioner{}
	step := NewStep("test-step", "test", 5*time.Second, actor, asyncActor)
	testLogger := logger.NewNoOpLogger()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rcder := record.NewRecord("test-0", "", 0)
		step.AsyncHandle("ctx", "response", "test-0-1", []int{0, 1, 2}, 2, rcder, testLogger)
	}
}
