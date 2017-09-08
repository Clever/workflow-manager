package executor

import "github.com/Clever/workflow-manager/resources"

// Executor defines the interface for submitting and managing workflows
type Executor interface {
	SubmitWorkflow(name string, definition string, dependencies []string, input []string, queue string, attempts int64) (string, error)
	Status(ids []*resources.Job) []error
	Cancel(ids []*resources.Job, reason string) []error
}
