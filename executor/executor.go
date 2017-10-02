package executor

import "github.com/Clever/workflow-manager/gen-go/models"

// Executor defines the interface for submitting and managing workflows
type Executor interface {
	SubmitWorkflow(name string, definition string, dependencies []string, input string, queue string, attempts int64) (string, error)
	Status(ids []*models.Job) []error
	Cancel(ids []*models.Job, reason string) []error
}
