package store

import (
	"errors"
	"fmt"

	"github.com/Clever/workflow-manager/gen-go/models"
)

// Store defines the interface for persistence of Workflow Manager resources.
type Store interface {
	SaveWorkflowDefinition(wfd models.WorkflowDefinition) error
	UpdateWorkflowDefinition(wfd models.WorkflowDefinition) (models.WorkflowDefinition, error)
	GetWorkflowDefinitions() ([]models.WorkflowDefinition, error)
	GetWorkflowDefinitionVersions(name string) ([]models.WorkflowDefinition, error)
	GetWorkflowDefinition(name string, version int) (models.WorkflowDefinition, error)
	LatestWorkflowDefinition(name string) (models.WorkflowDefinition, error)

	SaveStateResource(res models.StateResource) error
	GetStateResource(name, namespace string) (models.StateResource, error)
	DeleteStateResource(name, namespace string) error

	SaveWorkflow(workflow models.Workflow) error
	DeleteWorkflowByID(workflowID string) error
	UpdateWorkflow(workflow models.Workflow) error
	GetWorkflowByID(id string) (models.Workflow, error)
	GetWorkflows(query *models.WorkflowQuery) ([]models.Workflow, string, error)
	GetPendingWorkflowIDs() ([]string, error)
	LockWorkflow(id string) error
	UnlockWorkflow(id string) error
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

// InvalidQueryStructureError is returned for workflow queries that have a disallowed structure,
//  like including both a Status and a ResolvedByUser value.
type InvalidQueryStructureError struct {
	cause string
}

// Error implements the error interface for InvalidQueryStructureError.
func (e InvalidQueryStructureError) Error() string {
	return fmt.Sprintf("Invalid query structure: %v", e.cause)
}

// NewInvalidQueryStructureError creates an InvalidQueryStructureErorr
func NewInvalidQueryStructureError(cause string) InvalidQueryStructureError {
	return InvalidQueryStructureError{cause}
}
