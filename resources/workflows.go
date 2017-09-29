package resources

import (
	"time"

	"github.com/Clever/workflow-manager/gen-go/models"
	"github.com/go-openapi/strfmt"
	uuid "github.com/satori/go.uuid"
)

// NewWorkflow creates a new Workflow struct for a WorkflowDefinition
func NewWorkflow(wfd *models.WorkflowDefinition, input string, namespace string, queue string, tags map[string]interface{}) *models.Workflow {
	return &models.Workflow{
		ID:                 uuid.NewV4().String(),
		CreatedAt:          strfmt.DateTime(time.Now()),
		LastUpdated:        strfmt.DateTime(time.Now()),
		WorkflowDefinition: wfd,
		Status:             models.WorkflowStatusQueued,
		Jobs:               []*models.Job{},
		Namespace:          namespace,
		Queue:              queue,
		Input:              input,
		Tags:               tags,
	}
}

// AddJob adds a new job (representing a State) to the Workflow
func AddJob(wf *models.Workflow, j *models.Job) error {
	// TODO: run validation
	// 1. ensure this job actually corresponds to a State
	// 2. should have a 1:1 mapping with State unless RETRY

	// for now just keep track of the jobIds
	wf.Jobs = append(wf.Jobs, j)

	return nil
}

// StatusToInt converts a WorkflowStatus to an integer. This is useful for generating metrics.
func WorkflowStatusToInt(status models.WorkflowStatus) int {
	switch status {
	// non-completion return non-zero
	case models.WorkflowStatusCancelled:
		return -1
	case models.WorkflowStatusFailed:
		return 1
	// states in path to completion return zero
	case models.WorkflowStatusQueued:
		return 0
	case models.WorkflowStatusRunning:
		return 0
	case models.WorkflowStatusSucceeded:
		return 0
	default:
		return 0
	}
}

// WorkflowIsDone can be used check if a workflow's state is expected to change
// true if the workflow is in a final state; false if its status might still change
func WorkflowIsDone(wf *models.Workflow) bool {
	// Look at the individual jobs states as well as the workflow status
	// since the workflow status can be updated before the jobs have transitioned
	// into a final state
	for _, job := range wf.Jobs {
		if !JobIsDone(job.Status) {
			return false
		}
	}
	return (wf.Status == models.WorkflowStatusCancelled ||
		wf.Status == models.WorkflowStatusFailed ||
		wf.Status == models.WorkflowStatusSucceeded)
}
