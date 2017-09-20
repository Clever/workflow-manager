package resources

import (
	"time"

	"github.com/Clever/workflow-manager/gen-go/models"
)

type JobStatus string

const (
	JobStatusCreated     JobStatus = "CREATED"             // initialized
	JobStatusQueued      JobStatus = "QUEUED"              // submitted to queue
	JobStatusWaiting     JobStatus = "WAITING_FOR_DEPS"    // waiting for dependencies
	JobStatusRunning     JobStatus = "RUNNING"             // running
	JobStatusSucceeded   JobStatus = "SUCCEEDED"           // completed successfully
	JobStatusFailed      JobStatus = "FAILED"              // failed
	JobStatusAborted     JobStatus = "ABORTED_DEPS_FAILED" // aborted due to a dependency failure
	JobStatusUserAborted JobStatus = "ABORTED_BY_USER"     // aborted due to user action
)

type JobDetail struct {
	Attempts      []*models.JobAttempt
	ContainerId   string // identification string for the running container
	CreatedAt     time.Time
	DependencyIDs []string
	Input         []string
	Output        string
	QueueName     string
	StartedAt     time.Time
	Status        JobStatus
	StatusReason  string
	StoppedAt     time.Time
}

// Job represents an active State and as part of a Job
type Job struct {
	JobDetail
	ID            string
	Name          string
	State         string
	StateResource StateResource
}

// NewJob creates a new Job
func NewJob(id, name, state string, stateResource StateResource, input []string) *Job {
	return &Job{
		ID:            id,
		Name:          name,
		State:         state,
		StateResource: stateResource,
		JobDetail: JobDetail{
			Input:  input,
			Status: JobStatusCreated,
		},
	}
}

// IsDone can be used check if a task's state is expected to change
// true if the task is in a final state; false if its status might still change
func (job *Job) IsDone() bool {
	return (job.Status == JobStatusFailed ||
		job.Status == JobStatusSucceeded ||
		job.Status == JobStatusAborted ||
		job.Status == JobStatusUserAborted)
}

func (job *Job) SetStatus(status JobStatus) {
	job.Status = status
}

func (job *Job) StatusToInt() int {
	switch job.Status {
	// non-completion return non-zero
	case JobStatusFailed:
		return 1
	case JobStatusUserAborted:
		return -1
	case JobStatusAborted:
		return -2
	// states in path to completion return zero
	default:
		return 0
	}
}

func (job *Job) SetDetail(detail JobDetail) {
	job.Attempts = detail.Attempts
	job.ContainerId = detail.ContainerId
	job.CreatedAt = detail.CreatedAt
	job.DependencyIDs = detail.DependencyIDs
	job.Input = detail.Input
	job.Output = detail.Output
	job.QueueName = detail.QueueName
	job.StartedAt = detail.StartedAt
	job.Status = detail.Status
	job.StatusReason = detail.StatusReason
	job.StoppedAt = detail.StoppedAt
}
