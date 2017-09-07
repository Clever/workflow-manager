package resources

import (
	"time"

	uuid "github.com/satori/go.uuid"
)

type JobStatus string

const (
	Queued    JobStatus = "QUEUED"
	Running   JobStatus = "RUNNING"
	Failed    JobStatus = "FAILED"
	Succeeded JobStatus = "SUCCEEDED"
	Cancelled JobStatus = "CANCELLED"
)

// Job contains information about a running instance of a WorkflowDefinition
type Job struct {
	ID                 string // GUID for the job
	CreatedAt          time.Time
	LastUpdated        time.Time
	WorkflowDefinition WorkflowDefinition // WorkflowDefinition executed as part of this job
	Input              []string           // Starting input for the job
	Tasks              []*Task            // list of states submitted as tasks
	Status             JobStatus
}

// NewJob creates a new Job struct for a WorkflowDefinition
func NewJob(wf WorkflowDefinition, input []string) *Job {
	return &Job{
		ID:                 uuid.NewV4().String(),
		WorkflowDefinition: wf,
		Input:              input,
		Status:             Queued,
		CreatedAt:          time.Now(),
	}
}

// AddTask adds a new task (representing a State) to the Job
func (j *Job) AddTask(t *Task) error {
	// TODO: run validation
	// 1. ensure this task actually corresponds to a State
	// 2. should have a 1:1 mapping with State unless RETRY

	// for now just keep track of the taskIds
	j.Tasks = append(j.Tasks, t)

	return nil
}

// StatusToInt converts the current JobStatus to an
// integer. This is useful for generating metrics.
func (j *Job) StatusToInt() int {
	switch j.Status {
	// non-completion return non-zero
	case Cancelled:
		return -1
	case Failed:
		return 1
	// states in path to completion return zero
	case Queued:
		return 0
	case Running:
		return 0
	case Succeeded:
		return 0
	default:
		return 0
	}
}

// IsDone can be used check if a job's state is expected to change
// true if the job is in a final state; false if its status might still change
func (j *Job) IsDone() bool {
	// Look at the individual tasks states as well as the job status
	// since the job status can be updated before the tasks have transitioned
	// into a final state
	for _, task := range j.Tasks {
		if !task.IsDone() {
			return false
		}
	}
	return (j.Status == Cancelled || j.Status == Failed || j.Status == Succeeded)
}
