package workflow


type Record struct {
    ID string
    Content map[string]interface{}
}

// json 格式的追加存储
func(r *Record) AppendInfo() {

}