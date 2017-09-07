package store

import (
	"fmt"

	"github.com/Clever/workflow-manager/gen-go/models"
	"github.com/Clever/workflow-manager/resources"
)

// WorkflowStore defines the interface for persistence of Workflow defintions
type Store interface {
	SaveWorkflowDefinition(def resources.WorkflowDefinition) error
	UpdateWorkflowDefinition(def resources.WorkflowDefinition) (resources.WorkflowDefinition, error)
	GetWorkflowDefinitions() ([]resources.WorkflowDefinition, error)
	GetWorkflowDefinitionVersions(name string) ([]resources.WorkflowDefinition, error)
	GetWorkflowDefinition(name string, version int) (resources.WorkflowDefinition, error)
	LatestWorkflowDefinition(name string) (resources.WorkflowDefinition, error)

	SaveStateResource(res resources.StateResource) error
	GetStateResource(name, namespace string) (resources.StateResource, error)
	DeleteStateResource(name, namespace string) error

	SaveJob(job resources.Job) error
	UpdateJob(job resources.Job) error
	GetJob(id string) (resources.Job, error)
	GetJobsForWorkflowDefinition(workflowName string) ([]resources.Job, error)
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
