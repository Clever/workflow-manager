package executor

import "github.com/Clever/workflow-manager/resources"

// Executor defines the interface for submitting and managing workflows
type Executor interface {
	SubmitWorkflow(name string, definition string, dependencies []string, input []string, queue string, attempts int64) (string, error)
	Status(ids []*resources.Task) []error
	Cancel(ids []*resources.Task, reason string) []error
}
