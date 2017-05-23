package store

import (
	"fmt"

	"github.com/Clever/workflow-manager/gen-go/models"
	"github.com/Clever/workflow-manager/resources"
)

// WorkflowStore defines the interface for persistence of Workflow defintions
type Store interface {
	SaveWorkflow(def resources.WorkflowDefinition) error
	UpdateWorkflow(def resources.WorkflowDefinition) (resources.WorkflowDefinition, error)
	GetWorkflows() ([]resources.WorkflowDefinition, error)
	GetWorkflow(name string, version int) (resources.WorkflowDefinition, error)
	LatestWorkflow(name string) (resources.WorkflowDefinition, error)

	SaveJob(job resources.Job) error
	UpdateJob(job resources.Job) error
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

func NewNotFound(name string) models.NotFound {
	return models.NotFound{Message: name}
}
