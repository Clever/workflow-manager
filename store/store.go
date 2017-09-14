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

	SaveWorkflow(workflow resources.Workflow) error
	UpdateWorkflow(workflow resources.Workflow) error
	GetWorkflowByID(id string) (resources.Workflow, error)
	GetWorkflows(workflowName string) ([]resources.Workflow, error)
	GetPendingWorkflowIDs() ([]string, error)
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
