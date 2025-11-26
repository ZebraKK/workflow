package task

/*
import (
    "sync"
)

type Scheduler struct {
    Tasks map[string]*Task

    callbacks map[string]*sync.WaitGroup
}

func NewScheduler() *Scheduler {
    return &Scheduler{
        Tasks: make(map[string]*Task),
    }
}

func (s *Scheduler) AddTask(t *Task) {
    s.Tasks[t.ID] = t
}

func (s *Scheduler) RunAll() error {

    return nil
}

// 管理回调
// 注册回调 taskID, stageIndex, stepIndex, callback func
func (s *Scheduler) CompletionHandler(wg *sync.WaitGroup, id string) {
    s.callbacks[id] = wg
}

func (s *Scheduler) CallbackHandler() {
    // 解析id，找到对应的wg，调用wg.Done()
    id := "" // 从外部获取id
    wg := s.callbacks[id]
    if wg != nil {
        wg.Done()
    }

    // del map
    delete(s.callbacks, id)
}

// 设置step完成
// 通过id设置step完成
// 方便异步step完成后，通知task
// id格式： taskID-stageIndex-stepIndex
// 解析id
// 通过id找到对应的step，设置完成
// 这里需要一个map来存储step，方便查找
// stepsMap: map[stageIndex]map[stepIndex]steper
// 在NewTask中初始化这个map
// 在添加step时，更新这个map
// 通过id找到对应的step，设置完成
// id格式： taskID-stageIndex-stepIndex
// 解析id
func (t *Task) decodeID(id string) (string, string) {
    var gindex, sindex string
    n := 0
    for i := 0; i < len(id); i++ {
        if id[i] == '-' {
            n++
            continue
        }
        if n == 1 {
            gindex += string(id[i])
        } else if n == 2 {
            sindex += string(id[i])
        }
    }
    return gindex, sindex
}
*/

/*
// id: task-stageindex-stepindex
func (t *Task) SetStepCompleted(id string) {
    gindex, sindex := t.decodeID(id)
    step, ok := t.stepsMap[gindex][sindex]
    if !ok {
        return
    }
    step.SetCompleted()
}
*/
