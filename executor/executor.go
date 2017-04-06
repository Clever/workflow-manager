package executor

import "github.com/Clever/workflow-manager/resources"

// Executor defines the interface for submitting and managing tasks
type Executor interface {
	SubmitJob(name string, definition string, dependencies []string, input []string) (string, error)
	Status(ids []*resources.Task) []error
}
