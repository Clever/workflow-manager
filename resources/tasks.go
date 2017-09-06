package resources

import (
	"time"

	"github.com/Clever/workflow-manager/gen-go/models"
)

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
	Attempts     []*models.TaskAttempt
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

// IsDone can be used check if a task's state is expected to change
// true if the task is in a final state; false if its status might still change
func (t *Task) IsDone() bool {
	return (t.Status == TaskStatusFailed ||
		t.Status == TaskStatusSucceeded ||
		t.Status == TaskStatusAborted ||
		t.Status == TaskStatusUserAborted)
}

func (t *Task) SetStatus(status TaskStatus) {
	t.Status = status
}

func (t *Task) StatusToInt() int {
	switch t.Status {
	// non-completion return non-zero
	case TaskStatusFailed:
		return 1
	case TaskStatusUserAborted:
		return -1
	case TaskStatusAborted:
		return -2
	// states in path to completion return zero
	default:
		return 0
	}
}

func (t *Task) SetDetail(detail TaskDetail) {
	t.CreatedAt = detail.CreatedAt
	t.StartedAt = detail.StartedAt
	t.StoppedAt = detail.StoppedAt
	t.ContainerId = detail.ContainerId
	t.Status = detail.Status
	t.StatusReason = detail.StatusReason
	t.Attempts = detail.Attempts
}
