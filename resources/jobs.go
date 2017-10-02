package resources

import (
	"github.com/Clever/workflow-manager/gen-go/models"
)

// NewJob creates a new Job
func NewJob(id, name, state string, stateResource *models.StateResource, input string) *models.Job {
	return &models.Job{
		ID:            id,
		Name:          name,
		State:         state,
		StateResource: stateResource,
		Input:         input,
		Status:        models.JobStatusCreated,
	}
}

// IsDone can be used check if a task's state is expected to change
// true if the task is in a final state; false if its status might still change
func JobIsDone(status models.JobStatus) bool {
	return (status == models.JobStatusFailed ||
		status == models.JobStatusSucceeded ||
		status == models.JobStatusAbortedDepsFailed ||
		status == models.JobStatusAbortedByUser)
}

func JobStatusToInt(status models.JobStatus) int {
	switch status {
	// non-completion return non-zero
	case models.JobStatusFailed:
		return 1
	case models.JobStatusAbortedByUser:
		return -1
	case models.JobStatusAbortedDepsFailed:
		return -2
	// states in path to completion return zero
	default:
		return 0
	}
}
