package resources

type TaskStatus string

const (
	TaskStatusCreated   TaskStatus = "CREATED"   // initialized
	TaskStatusQueued    TaskStatus = "QUEUED"    // submitted to queue
	TaskStatusWaiting   TaskStatus = "WAITING"   // waiting for dependencies
	TaskStatusRunning   TaskStatus = "RUNNING"   // running
	TaskStatusSucceeded TaskStatus = "SUCCEEDED" // completed successfully
	TaskStatusFailed    TaskStatus = "FAILED"    // failed
	TaskStatusCancelled TaskStatus = "CANCELLED" // cancelled due to user action
)

// Task represents an active State and as part of a Job
type Task struct {
	ID     string
	Name   string
	Input  string
	State  string
	status TaskStatus
}

// NewTask creates a new Task
func NewTask(id, name, state, input string) *Task {
	return &Task{
		ID:     id,
		Name:   name,
		Input:  input,
		status: TaskStatusCreated,
	}
}

// IsComplete can be used check if a task is completed
// true if failed or succeded
func (t *Task) IsComplete() bool {
	return (t.status == TaskStatusFailed || t.status == TaskStatusSucceeded)
}

func (t *Task) SetStatus(status TaskStatus) {
	t.status = status
}

func (t *Task) Status() TaskStatus {
	return t.status
}
