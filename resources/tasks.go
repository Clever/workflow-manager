package resources

import "time"

type TaskStatus string

const (
	TaskStatusCreated     TaskStatus = "CREATED"             // initialized
	TaskStatusQueued      TaskStatus = "QUEUED"              // submitted to queue
	TaskStatusWaiting     TaskStatus = "WAITING_FOR_DEPS"    // waiting for dependencies
	TaskStatusRunning     TaskStatus = "RUNNING"             // running
	TaskStatusSucceeded   TaskStatus = "SUCCEEDED"           // completed successfully
	TaskStatusFailed      TaskStatus = "FAILED"              // failed
	TaskStatusAborted     TaskStatus = "ABORTED_DEPS_FAILED" // aborted due to a dependency failure
	TaskStatusUserAborted TaskStatus = "ABORTED_BY_USER"     // aborted due to user action
)

type TaskDetail struct {
	CreatedAt    time.Time
	StartedAt    time.Time
	StoppedAt    time.Time
	ContainerId  string // identification string for the running container
	StatusReason string
	Status       TaskStatus
}

// Task represents an active State and as part of a Job
type Task struct {
	TaskDetail
	ID            string
	Name          string
	Input         []string
	State         string
	StateResource StateResource
}

// NewTask creates a new Task
func NewTask(id, name, state string, stateResource StateResource, input []string) *Task {
	return &Task{
		ID:            id,
		Name:          name,
		Input:         input,
		State:         state,
		StateResource: stateResource,
		TaskDetail: TaskDetail{
			Status: TaskStatusCreated,
		},
	}
}

// IsComplete can be used check if a task is completed
// true if failed or succeded
func (t *Task) IsComplete() bool {
	return (t.Status == TaskStatusFailed || t.Status == TaskStatusSucceeded)
}

func (t *Task) SetStatus(status TaskStatus) {
	t.Status = status
}

func (t *Task) SetDetail(detail TaskDetail) {
	t.CreatedAt = detail.CreatedAt
	t.StartedAt = detail.StartedAt
	t.StoppedAt = detail.StoppedAt
	t.ContainerId = detail.ContainerId
	t.Status = detail.Status
	t.StatusReason = detail.StatusReason
}
