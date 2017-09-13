package resources

import (
	"time"

	uuid "github.com/satori/go.uuid"
)

type WorkflowStatus string

const (
	Queued    WorkflowStatus = "QUEUED"
	Running   WorkflowStatus = "RUNNING"
	Failed    WorkflowStatus = "FAILED"
	Succeeded WorkflowStatus = "SUCCEEDED"
	Cancelled WorkflowStatus = "CANCELLED"
)

// Workflow contains information about a running instance of a WorkflowDefinition
type Workflow struct {
	ID                 string // GUID for the workflow
	CreatedAt          time.Time
	LastUpdated        time.Time
	WorkflowDefinition WorkflowDefinition // WorkflowDefinition executed as part of this workflow
	Input              []string           // Starting input for the workflow
	Jobs               []*Job             // list of states submitted as jobs
	Status             WorkflowStatus
	Namespace          string
	Queue              string
}

// NewWorkflow creates a new Workflow struct for a WorkflowDefinition
func NewWorkflow(wf WorkflowDefinition, input []string, namespace string, queue string) *Workflow { // @todo just take entire input piece?
	return &Workflow{
		ID:                 uuid.NewV4().String(),
		WorkflowDefinition: wf,
		Input:              input,
		Status:             Queued,
		CreatedAt:          time.Now(),
		Namespace:          namespace,
		Queue:              queue, // @todo or from namespace? at what stage?
	}
}

// AddJob adds a new job (representing a State) to the Workflow
func (w *Workflow) AddJob(t *Job) error {
	// TODO: run validation
	// 1. ensure this job actually corresponds to a State
	// 2. should have a 1:1 mapping with State unless RETRY

	// for now just keep track of the jobIds
	w.Jobs = append(w.Jobs, t)

	return nil
}

// StatusToInt converts the current WorkflowStatus to an
// integer. This is useful for generating metrics.
func (w *Workflow) StatusToInt() int {
	switch w.Status {
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

// IsDone can be used check if a workflow's state is expected to change
// true if the workflow is in a final state; false if its status might still change
func (w *Workflow) IsDone() bool {
	// Look at the individual jobs states as well as the workflow status
	// since the workflow status can be updated before the jobs have transitioned
	// into a final state
	for _, job := range w.Jobs {
		if !job.IsDone() {
			return false
		}
	}
	return (w.Status == Cancelled || w.Status == Failed || w.Status == Succeeded)
}
