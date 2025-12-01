package record

import (
	"fmt"
	"strconv"
	"strings"
)

const (
	StatusCreated      = "created"
	StatusProcessing   = "processing"
	StatusDone         = "done"
	StatusFailed       = "failed"
	StatusAsyncWaiting = "async_waiting"
)

type Record struct {
	ID          string
	StartAt     int64
	EndAt       int64
	Status      string // created, processing, done, failed, canceled, retry, async_waiting
	AsyncRecord *Record
	Records     []*Record // 子记录 和 stage中的 steps 一一对应
}

func NewRecord(prefix, index string, size int) *Record {
	r := &Record{
		Status:  "created",
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
	if index < 0 || index >= len(r.Records) {
		return
	}
	r.Records[index] = rcd
}

func (r *Record) IsAsyncWaiting() bool {
	return r.Status == "async_waiting"
}
