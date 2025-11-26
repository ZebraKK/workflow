package record

type Record struct {
    ID string

    StartAt int64
    EndAt   int64
    Status  string // created, processing, done, failed, canceled, retry, async_waiting

    Records []*Record // 子记录
}

func NewRecord(id string) *Record {
    return &Record{
        ID:      id,
        Status:  "created",
        Records: make([]*Record, 0),
    }
}

// json 格式的追加存储
func (r *Record) AppendRecord(rcd *Record) {
    r.Records = append(r.Records, rcd)
}

func (r *Record) IsAsyncWaiting() bool {
    return r.Status == "async_waiting"
}
