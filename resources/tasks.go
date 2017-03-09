package resources

// Task represents an active State and as part of a Job
type Task struct {
	ID       string
	Name     string
	Input    string
	State    string
	success  bool
	complete bool
}

// NewTask creates a new Task
func NewTask(id, name, state, input string) *Task {
	return &Task{
		ID:       id,
		Name:     name,
		Input:    input,
		complete: false,
	}
}

// IsComplete can be used check if a task is completed
// true if failed or succeded
func (t *Task) IsComplete() bool {
	return t.complete
}

// IsSuccess returns true if the task is successfully completed
func (t *Task) IsSuccess() bool {
	return t.complete && t.success
}

// MarkComplete marks a Task as successful or failed
func (t *Task) MarkComplete(success bool) {
	t.complete = true
	t.success = success
}
