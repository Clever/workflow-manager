package store

import (
	"fmt"

	"github.com/Clever/workflow-manager/resources"
)

// WorkflowStore defines the interface for persistence of Workflow defintions
type Store interface {
	CreateWorkflow(def resources.WorkflowDefinition) error
	UpdateWorkflow(def resources.WorkflowDefinition) (resources.WorkflowDefinition, error)
	GetWorkflow(name string, version int) (resources.WorkflowDefinition, error)
	LatestWorkflow(name string) (resources.WorkflowDefinition, error)

	CreateJob(job resources.Job) error
	GetJob(id string) (resources.Job, error)
	GetJobsForWorkflow(workflowName string) ([]resources.Job, error)
}

type ConflictError struct {
	name string
}

func (e ConflictError) Error() string {
	return fmt.Sprintf("Already Exists: %s", e.name)
}

func NewConflict(name string) ConflictError {
	return ConflictError{name}
}

type NotFoundError struct {
	name string
}

func (e NotFoundError) Error() string {
	return fmt.Sprintf("Not Found: %s", e.name)
}

func NewNotFound(name string) NotFoundError {
	return NotFoundError{name}
}
