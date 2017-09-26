package store

import (
	"errors"
	"fmt"

	"github.com/Clever/workflow-manager/gen-go/models"
	"github.com/Clever/workflow-manager/resources"
)

// Store defines the interface for persistence of Workflow Manager resources.
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
	GetWorkflows(query *WorkflowQuery) ([]resources.Workflow, string, error)
	GetPendingWorkflowIDs() ([]string, error)
	LockWorkflow(id string) error
	UnlockWorkflow(id string) error
}

// WorkflowQuery contains filtering options for workflow queries.
type WorkflowQuery struct {
	DefinitionName string
	Limit          int
	OldestFirst    bool
	PageToken      string
	Status         string
}

// ErrWorkflowLocked is returned from LockWorfklow in the case of the workflow already being locked.
var ErrWorkflowLocked = errors.New("workflow already locked")

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

// InvalidPageTokenError is returned for workflow queries that contain a malformed or invalid page
// token.
type InvalidPageTokenError struct {
	cause error
}

// NewInvalidPageTokenError returns a new InvalidPageTokenError.
func NewInvalidPageTokenError(cause error) InvalidPageTokenError {
	return InvalidPageTokenError{
		cause: cause,
	}
}

// Error implements the error interface.
func (e InvalidPageTokenError) Error() string {
	return fmt.Sprintf("invalid page token: %v", e.cause)
}
