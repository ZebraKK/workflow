package record

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
)

type Record struct {
	ID          string
	StartAt     int64
	EndAt       int64
	Status      string // created, processing, done, failed, canceled, retry, async_waiting
	AsyncRecord *Record

	Records    []*Record // 子记录
	RecordsMap map[string]*Record
	mu         sync.RWMutex
}

func NewRecord(prefix, index string) *Record {
	r := &Record{
		Status:     "created",
		Records:    make([]*Record, 0),
		RecordsMap: make(map[string]*Record),
	}

	r.ID = r.NextRecordID(prefix, index)
	return r
}

func (r *Record) NextRecordID(prefix, index string) string {
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

// json 格式的追加存储
func (r *Record) AppendRecord(rcd *Record) {
	r.Records = append(r.Records, rcd)
}

func (r *Record) AddRecord(rcd *Record) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.RecordsMap == nil {
		r.RecordsMap = make(map[string]*Record)
	}
	r.RecordsMap[rcd.ID] = rcd
}

func (r *Record) IsAsyncWaiting() bool {
	return r.Status == "async_waiting"
}
