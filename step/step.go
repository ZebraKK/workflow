package step

/*
type mgrServicer interface {
	CompletionHandler(wg *sync.WaitGroup, id string)
	PostHook()
}
*/

// 最小执行单元
type Step struct {
	Description string
	ID          string
	State       string // created, processing, done, failed, canceled, retry, async_waiting

	IsAsync       bool
	StepActor     func() error // 统一的执行入口
	DealAsyncResp func(resp string) error

	//mgrService mgrServicer // 外部服务能力来设置回调

}

func NewStep(description, id string, execute func() error) *Step {
	return &Step{
		Description: description,
		ID:          id,
		StepActor:   execute,
		State:       "created",
	}
}

func (s *Step) Run() error {

	var err error
	defer func() {
		if err != nil {
			s.State = "failed"
		} else {
			if s.IsAsync {
				s.State = "async_waiting"
			} else {
				s.State = "done"
			}
		}
	}()

	s.State = "processing"
	if s.StepActor != nil {
		err = s.StepActor()
	}

	return err
}

// step 处理异步响应 (设置step给的自定义的函数)
func (s *Step) DealWithResp(resp string) {
	var err error
	defer func() {
		if err != nil {
			s.State = "failed"
		} else {
			s.State = "done"
		}
	}()

	if s.DealAsyncResp != nil {
		err = s.DealAsyncResp(resp)
		return
	}
}

func (s *Step) IsAsyncWaiting() bool {
	return s.State == "async_waiting"
}
