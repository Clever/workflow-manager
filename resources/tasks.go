package resources

type Task struct {
	Id       string
	Input    map[string]interface{}
	Output   map[string]interface{}
	complete bool
}

func NewTask(id string, input map[string]interface{}) Task {
	return Task{
		Id:       id,
		Input:    input,
		complete: false,
	}
}

func (t Task) IsComplete() bool {
	return t.complete
}

func (t Task) MarkComplete() {
	t.complete = true
}
