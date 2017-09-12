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
	CreatedAt    time.Time
	StartedAt    time.Time
	StoppedAt    time.Time
	ContainerId  string // identification string for the running container
	StatusReason string
	Status       JobStatus
	Attempts     []*models.JobAttempt
}

// Job represents an active State and as part of a Job
type Job struct {
	JobDetail
	ID            string
	Name          string
	Input         []string
	State         string
	StateResource StateResource
}

// NewJob creates a new Job
func NewJob(id, name, state string, stateResource StateResource, input []string) *Job {
	return &Job{
		ID:            id,
		Name:          name,
		Input:         input,
		State:         state,
		StateResource: stateResource,
		JobDetail: JobDetail{
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
	job.CreatedAt = detail.CreatedAt
	job.StartedAt = detail.StartedAt
	job.StoppedAt = detail.StoppedAt
	job.ContainerId = detail.ContainerId
	job.Status = detail.Status
	job.StatusReason = detail.StatusReason
	job.Attempts = detail.Attempts
}
