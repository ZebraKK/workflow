package record

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
)

const (
	StatusCreated      = "created"
	StatusProcessing   = "processing"
	StatusDone         = "done"
	StatusFailed       = "failed"
	StatusAsyncWaiting = "async_waiting"
)

type Record struct {
	mu          sync.RWMutex
	ID          string
	StartAt     int64
	EndAt       int64
	Status      string // created, processing, done, failed, canceled, retry, async_waiting
	AsyncRecord *Record
	Records     []*Record // 子记录 和 stage中的 steps 一一对应
}

func NewRecord(prefix, index string, size int) *Record {
	r := &Record{
		Status:  StatusCreated,
		Records: make([]*Record, size),
	}

	r.ID = r.recordID(prefix, index)
	return r
}

func (r *Record) recordID(prefix, index string) string {
	if index == "-async" {
		return fmt.Sprintf("%s-async", prefix)
	}
	// prefix  xxxx-n
	// 那么 next的id 为 xxxx-n-n+1
	// index  可以为空,如果不为空,那么就是 xxxx-n-n+1.index
	parts := strings.Split(prefix, "-")
	if len(parts) < 2 {
		return prefix + "-0"
	}
	n, err := strconv.Atoi(parts[len(parts)-1])
	if err != nil {
		return prefix
	}
	nextN := n + 1
	nextID := fmt.Sprintf("%s-%d", prefix, nextN)
	if index != "" {
		nextID = fmt.Sprintf("%s.%s", nextID, index)
	}
	return nextID
}

func (r *Record) AddRecord(index int, rcd *Record) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if index < 0 || index >= len(r.Records) {
		return
	}
	r.Records[index] = rcd
}

func (r *Record) GetRecord(index int) *Record {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.Records == nil || index < 0 || index >= len(r.Records) {
		return nil
	}
	return r.Records[index]
}

func (r *Record) GetRecordsLen() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.Records == nil {
		return 0
	}
	return len(r.Records)
}

func (r *Record) SetStatus(status string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.Status = status
}

func (r *Record) GetStatus() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.Status
}

func (r *Record) IsAsyncWaiting() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.Status == StatusAsyncWaiting
}

func (r *Record) SetStartAt(t int64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.StartAt = t
}

func (r *Record) SetEndAt(t int64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.EndAt = t
}

func (r *Record) SetAsyncRecord(asyncRecord *Record) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.AsyncRecord = asyncRecord
}

func (r *Record) GetAsyncRecord() *Record {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.AsyncRecord
}

// RecordSnapshot 是 Record 的可序列化快照，不含 mutex，用于持久化。
type RecordSnapshot struct {
	ID          string            `json:"id"`
	Status      string            `json:"status"`
	StartAt     int64             `json:"start_at"`
	EndAt       int64             `json:"end_at"`
	AsyncRecord *RecordSnapshot   `json:"async_record,omitempty"`
	Records     []*RecordSnapshot `json:"records,omitempty"`
}

// Snapshot 递归生成当前 Record 树的只读快照（持读锁）。
func (r *Record) Snapshot() *RecordSnapshot {
	r.mu.RLock()
	defer r.mu.RUnlock()

	snap := &RecordSnapshot{
		ID:      r.ID,
		Status:  r.Status,
		StartAt: r.StartAt,
		EndAt:   r.EndAt,
	}
	if r.AsyncRecord != nil {
		snap.AsyncRecord = r.AsyncRecord.Snapshot()
	}
	if len(r.Records) > 0 {
		snap.Records = make([]*RecordSnapshot, len(r.Records))
		for i, child := range r.Records {
			if child != nil {
				snap.Records[i] = child.Snapshot()
			}
		}
	}
	return snap
}

// RestoreRecord 从快照重建 Record 树（用于启动恢复）。
func RestoreRecord(s *RecordSnapshot) *Record {
	if s == nil {
		return nil
	}
	r := &Record{
		ID:      s.ID,
		Status:  s.Status,
		StartAt: s.StartAt,
		EndAt:   s.EndAt,
	}
	if s.AsyncRecord != nil {
		r.AsyncRecord = RestoreRecord(s.AsyncRecord)
	}
	if len(s.Records) > 0 {
		r.Records = make([]*Record, len(s.Records))
		for i, child := range s.Records {
			r.Records[i] = RestoreRecord(child)
		}
	}
	return r
}
