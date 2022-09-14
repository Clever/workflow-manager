package store

import (
	"context"
	"fmt"
	"time"

	"github.com/Clever/workflow-manager/gen-go/models"
	"github.com/go-openapi/strfmt"
)

// Store defines the interface for persistence of Workflow Manager resources.
type Store interface {
	SaveWorkflowDefinition(ctx context.Context, wfd models.WorkflowDefinition) error
	UpdateWorkflowDefinition(ctx context.Context, wfd models.WorkflowDefinition) (models.WorkflowDefinition, error)
	GetWorkflowDefinitions(ctx context.Context) ([]models.WorkflowDefinition, error)
	GetWorkflowDefinitionVersions(ctx context.Context, name string) ([]models.WorkflowDefinition, error)
	GetWorkflowDefinition(ctx context.Context, name string, version int) (models.WorkflowDefinition, error)
	LatestWorkflowDefinition(ctx context.Context, name string) (models.WorkflowDefinition, error)

	SaveStateResource(ctx context.Context, res models.StateResource) error
	GetStateResource(ctx context.Context, name, namespace string) (models.StateResource, error)
	DeleteStateResource(ctx context.Context, name, namespace string) error

	SaveWorkflow(ctx context.Context, workflow models.Workflow) error
	DeleteWorkflowByID(ctx context.Context, workflowID string) error
	UpdateWorkflow(ctx context.Context, workflow models.Workflow) error
	GetWorkflowByID(ctx context.Context, id string) (models.Workflow, error)
	UpdateWorkflowAttributes(ctx context.Context, id string, update UpdateWorkflowAttributesInput) error
}

// UpdateWorkflowAttributesInput represents updates to one or more attributes of a workflow.
// A non-nil struct field represents an attribute to update.
type UpdateWorkflowAttributesInput struct {
	LastUpdated    *strfmt.DateTime
	Status         *models.WorkflowStatus
	StatusReason   *string
	StoppedAt      *strfmt.DateTime
	ResolvedByUser *bool
	Output         *string
	LastJob        *models.Job
}

// Map returns a map version of the update
func (u UpdateWorkflowAttributesInput) Map() map[string]interface{} {
	m := make(map[string]interface{})
	if u.LastUpdated != nil {
		m["last_updated"] = time.Time(*u.LastUpdated).Format(time.RFC3339Nano)
	}
	if u.Status != nil {
		m["status"] = string(*u.Status)
	}
	if u.StatusReason != nil {
		m["status_reason"] = *u.StatusReason
	}
	if u.StoppedAt != nil {
		m["stopped_at"] = time.Time(*u.StoppedAt).Format(time.RFC3339Nano)
	}
	if u.ResolvedByUser != nil {
		m["resolved_by_user"] = *u.ResolvedByUser
	}
	if u.Output != nil {
		m["output"] = *u.Output
	}
	return m
}

// ZeroValue returns whether the struct has not had any fields set.
func (u UpdateWorkflowAttributesInput) ZeroValue() bool {
	if u.LastUpdated != nil {
		return false
	}
	if u.Status != nil {
		return false
	}
	if u.StatusReason != nil {
		return false
	}
	if u.StoppedAt != nil {
		return false
	}
	if u.ResolvedByUser != nil {
		return false
	}
	if u.Output != nil {
		return false
	}
	return true
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
