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
	Tasks              []*Task            // list of states submitted as tasks
	Status             WorkflowStatus
}

// NewWorkflow creates a new Workflow struct for a WorkflowDefinition
func NewWorkflow(wf WorkflowDefinition, input []string) *Workflow {
	return &Workflow{
		ID:                 uuid.NewV4().String(),
		WorkflowDefinition: wf,
		Input:              input,
		Status:             Queued,
		CreatedAt:          time.Now(),
	}
}

// AddTask adds a new task (representing a State) to the Workflow
func (w *Workflow) AddTask(t *Task) error {
	// TODO: run validation
	// 1. ensure this task actually corresponds to a State
	// 2. should have a 1:1 mapping with State unless RETRY

	// for now just keep track of the taskIds
	w.Tasks = append(w.Tasks, t)

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
	// Look at the individual tasks states as well as the workflow status
	// since the workflow status can be updated before the tasks have transitioned
	// into a final state
	for _, task := range w.Tasks {
		if !task.IsDone() {
			return false
		}
	}
	return (w.Status == Cancelled || w.Status == Failed || w.Status == Succeeded)
}
