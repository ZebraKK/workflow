package record

import (
	"testing"
)

func TestNewRecord(t *testing.T) {
	tests := []struct {
		name      string
		prefix    string
		index     string
		size      int
		wantID    string
		wantSize  int
		wantState string
	}{
		{
			name:      "basic record creation",
			prefix:    "task-0",
			index:     "",
			size:      3,
			wantID:    "task-0-1",
			wantSize:  3,
			wantState: StatusCreated,
		},
		{
			name:      "record with index",
			prefix:    "job-5",
			index:     "step1",
			size:      2,
			wantID:    "job-5-6.step1",
			wantSize:  2,
			wantState: StatusCreated,
		},
		{
			name:      "empty prefix",
			prefix:    "",
			index:     "",
			size:      1,
			wantID:    "-0",
			wantSize:  1,
			wantState: StatusCreated,
		},
		{
			name:      "zero size",
			prefix:    "task-1",
			index:     "",
			size:      0,
			wantID:    "task-1-2",
			wantSize:  0,
			wantState: StatusCreated,
		},
		{
			name:      "async record",
			prefix:    "task-1",
			index:     "-async",
			size:      0,
			wantID:    "task-1-async",
			wantSize:  0,
			wantState: StatusCreated,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := NewRecord(tt.prefix, tt.index, tt.size)
			if r == nil {
				t.Fatal("NewRecord returned nil")
			}
			if r.ID != tt.wantID {
				t.Errorf("NewRecord() ID = %v, want %v", r.ID, tt.wantID)
			}
			if len(r.Records) != tt.wantSize {
				t.Errorf("NewRecord() Records size = %v, want %v", len(r.Records), tt.wantSize)
			}
			if r.Status != tt.wantState {
				t.Errorf("NewRecord() Status = %v, want %v", r.Status, tt.wantState)
			}
		})
	}
}

func TestRecordID(t *testing.T) {
	tests := []struct {
		name   string
		prefix string
		index  string
		want   string
	}{
		{
			name:   "standard format increment",
			prefix: "task-1",
			index:  "",
			want:   "task-1-2",
		},
		{
			name:   "with index suffix",
			prefix: "job-5",
			index:  "step1",
			want:   "job-5-6.step1",
		},
		{
			name:   "async marker",
			prefix: "workflow-3",
			index:  "-async",
			want:   "workflow-3-async",
		},
		{
			name:   "no dash in prefix",
			prefix: "simple",
			index:  "",
			want:   "simple-0",
		},
		{
			name:   "non-numeric suffix",
			prefix: "task-abc",
			index:  "",
			want:   "task-abc",
		},
		{
			name:   "multiple dashes",
			prefix: "a-b-c-10",
			index:  "",
			want:   "a-b-c-10-11",
		},
		{
			name:   "large number",
			prefix: "task-999",
			index:  "",
			want:   "task-999-1000",
		},
		{
			name:   "zero suffix",
			prefix: "task-0",
			index:  "idx",
			want:   "task-0-1.idx",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &Record{}
			got := r.recordID(tt.prefix, tt.index)
			if got != tt.want {
				t.Errorf("recordID() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAddRecord(t *testing.T) {
	tests := []struct {
		name      string
		size      int
		index     int
		shouldAdd bool
	}{
		{
			name:      "add at valid index 0",
			size:      3,
			index:     0,
			shouldAdd: true,
		},
		{
			name:      "add at valid index middle",
			size:      5,
			index:     2,
			shouldAdd: true,
		},
		{
			name:      "add at valid index last",
			size:      3,
			index:     2,
			shouldAdd: true,
		},
		{
			name:      "negative index",
			size:      3,
			index:     -1,
			shouldAdd: false,
		},
		{
			name:      "index equals size",
			size:      3,
			index:     3,
			shouldAdd: false,
		},
		{
			name:      "index exceeds size",
			size:      3,
			index:     10,
			shouldAdd: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parent := NewRecord("parent-0", "", tt.size)
			child := NewRecord("child-0", "", 0)

			parent.AddRecord(tt.index, child)

			if tt.shouldAdd {
				if parent.Records[tt.index] != child {
					t.Errorf("AddRecord() failed to add record at index %d", tt.index)
				}
			} else {
				// For out-of-bounds indices, verify the array wasn't modified incorrectly
				if tt.index >= 0 && tt.index < len(parent.Records) {
					if parent.Records[tt.index] != nil {
						t.Errorf("AddRecord() should not have added record at invalid index %d", tt.index)
					}
				}
			}
		})
	}
}

func TestAddRecordOverwrite(t *testing.T) {
	parent := NewRecord("parent-0", "", 2)
	child1 := NewRecord("child1-0", "", 0)
	child2 := NewRecord("child2-0", "", 0)

	parent.AddRecord(0, child1)
	if parent.Records[0] != child1 {
		t.Error("First AddRecord failed")
	}

	// Overwrite with second record
	parent.AddRecord(0, child2)
	if parent.Records[0] != child2 {
		t.Error("AddRecord should allow overwriting existing record")
	}
}

func TestIsAsyncWaiting(t *testing.T) {
	tests := []struct {
		name   string
		status string
		want   bool
	}{
		{
			name:   "async_waiting status",
			status: StatusAsyncWaiting,
			want:   true,
		},
		{
			name:   "created status",
			status: StatusCreated,
			want:   false,
		},
		{
			name:   "processing status",
			status: StatusProcessing,
			want:   false,
		},
		{
			name:   "done status",
			status: StatusDone,
			want:   false,
		},
		{
			name:   "failed status",
			status: StatusFailed,
			want:   false,
		},
		{
			name:   "empty status",
			status: "",
			want:   false,
		},
		{
			name:   "unknown status",
			status: "unknown",
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &Record{Status: tt.status}
			if got := r.IsAsyncWaiting(); got != tt.want {
				t.Errorf("IsAsyncWaiting() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRecordStatusConstants(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  string
	}{
		{"StatusCreated", StatusCreated, "created"},
		{"StatusProcessing", StatusProcessing, "processing"},
		{"StatusDone", StatusDone, "done"},
		{"StatusFailed", StatusFailed, "failed"},
		{"StatusAsyncWaiting", StatusAsyncWaiting, "async_waiting"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.value != tt.want {
				t.Errorf("%s = %v, want %v", tt.name, tt.value, tt.want)
			}
		})
	}
}

func TestRecordFields(t *testing.T) {
	r := NewRecord("test-0", "", 2)

	// Test initial values
	if r.StartAt != 0 {
		t.Errorf("Initial StartAt should be 0, got %d", r.StartAt)
	}
	if r.EndAt != 0 {
		t.Errorf("Initial EndAt should be 0, got %d", r.EndAt)
	}
	if r.AsyncRecord != nil {
		t.Error("Initial AsyncRecord should be nil")
	}

	// Test setting values
	r.StartAt = 1000
	r.EndAt = 2000
	r.Status = StatusProcessing

	if r.StartAt != 1000 {
		t.Errorf("StartAt = %d, want 1000", r.StartAt)
	}
	if r.EndAt != 2000 {
		t.Errorf("EndAt = %d, want 2000", r.EndAt)
	}
	if r.Status != StatusProcessing {
		t.Errorf("Status = %s, want %s", r.Status, StatusProcessing)
	}
}

func TestRecordWithAsyncRecord(t *testing.T) {
	parent := NewRecord("parent-0", "", 1)
	asyncRec := NewRecord("parent-0", "-async", 0)

	parent.AsyncRecord = asyncRec

	if parent.AsyncRecord == nil {
		t.Error("AsyncRecord should not be nil")
	}
	if parent.AsyncRecord.ID != "parent-0-async" {
		t.Errorf("AsyncRecord ID = %s, want parent-0-async", parent.AsyncRecord.ID)
	}
}

// Benchmark tests
func BenchmarkNewRecord(b *testing.B) {
	for i := 0; i < b.N; i++ {
		NewRecord("task-0", "", 10)
	}
}

func BenchmarkRecordID(b *testing.B) {
	r := &Record{}
	for i := 0; i < b.N; i++ {
		r.recordID("task-100", "index")
	}
}

func BenchmarkAddRecord(b *testing.B) {
	parent := NewRecord("parent-0", "", 100)
	child := NewRecord("child-0", "", 0)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		parent.AddRecord(i%100, child)
	}
}
