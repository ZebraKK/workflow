package workflow

import (
	"encoding/json"
	"sync"
	"time"

	"workflow/record"
)

// Store 持久化接口，由使用方实现（Redis、DB 等）。
// 不传 Store 时，Workflow 以纯内存模式运行。
type Store interface {
	Save(job *JobRecord) error
	Load(jobID string) (*JobRecord, error)
	Delete(jobID string) error
	ListByStatus(status string) ([]*JobRecord, error)
}

// JobRecord 是 Job 的可序列化快照，用于持久化存储。
// Tasker（Stage/Step 树）是代码不可序列化，恢复时由 PipelineID 重新关联。
type JobRecord struct {
	JobID      string          `json:"job_id"`
	PipelineID string          `json:"pipeline_id"`
	Status     string          `json:"status"`
	Ctx        json.RawMessage `json:"ctx"`              // 调用方序列化的请求参数
	Record     json.RawMessage `json:"record"`           // RecordSnapshot 序列化结果
	CreatedAt  int64           `json:"created_at"`       // UnixMilli
	UpdatedAt  int64           `json:"updated_at"`       // UnixMilli
}

// newJobRecord 从 Job 生成 JobRecord，序列化 Record 树快照。
func newJobRecord(j *Job) (*JobRecord, error) {
	snap := j.record.Snapshot()
	recBytes, err := json.Marshal(snap)
	if err != nil {
		return nil, err
	}
	now := time.Now().UnixMilli()
	jr := &JobRecord{
		JobID:      j.ID,
		PipelineID: j.Pipeline.ID,
		Status:     j.record.GetStatus(),
		Ctx:        j.ctx,
		Record:     recBytes,
		UpdatedAt:  now,
	}
	if jr.CreatedAt == 0 {
		jr.CreatedAt = now
	}
	return jr, nil
}

// restoreJob 从 JobRecord + Pipeline 重建 Job（用于启动恢复）。
func restoreJob(jr *JobRecord, pl *Pipeline) (*Job, error) {
	var snap record.RecordSnapshot
	if err := json.Unmarshal(jr.Record, &snap); err != nil {
		return nil, err
	}
	rec := record.RestoreRecord(&snap)
	return &Job{
		ID:       jr.JobID,
		Pipeline: *pl,
		ctx:      jr.Ctx,
		record:   rec,
	}, nil
}

// MemoryStore 是内置的内存实现，主要用于测试和开发。
// 生产环境请实现 Store 接口并接入真实存储后端。
type MemoryStore struct {
	mu   sync.RWMutex
	jobs map[string]*JobRecord
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{jobs: make(map[string]*JobRecord)}
}

func (m *MemoryStore) Save(job *JobRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	// 保留原始 CreatedAt
	if existing, ok := m.jobs[job.JobID]; ok {
		job.CreatedAt = existing.CreatedAt
	} else if job.CreatedAt == 0 {
		job.CreatedAt = time.Now().UnixMilli()
	}
	cp := *job
	m.jobs[job.JobID] = &cp
	return nil
}

func (m *MemoryStore) Load(jobID string) (*JobRecord, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	jr, ok := m.jobs[jobID]
	if !ok {
		return nil, nil
	}
	cp := *jr
	return &cp, nil
}

func (m *MemoryStore) Delete(jobID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.jobs, jobID)
	return nil
}

func (m *MemoryStore) ListByStatus(status string) ([]*JobRecord, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []*JobRecord
	for _, jr := range m.jobs {
		if jr.Status == status {
			cp := *jr
			result = append(result, &cp)
		}
	}
	return result, nil
}
