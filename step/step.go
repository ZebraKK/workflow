package step

type Actioner interface {
	StepActor() error
	DealAsyncResp(resp string) error
}

// 最小执行单元
type Step struct {
	Description string
	ID          string
	State       string // created, processing, done, failed, canceled, retry, async_waiting
	IsAsync     bool

	execute Actioner
}

func NewStep(description string, actor Actioner) *Step {
	if actor == nil {
		return nil
	}
	return &Step{
		Description: description,
		State:       "created",
		execute:     actor,
	}
}

func (s *Step) SetID(id string) {
	s.ID = id
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
	err = s.execute.StepActor()

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

	err = s.execute.DealAsyncResp(resp)
}

func (s *Step) IsAsyncWaiting() bool {
	return s.State == "async_waiting"
}
