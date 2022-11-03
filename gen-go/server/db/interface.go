package db

import (
	"context"

	"github.com/Clever/workflow-manager/gen-go/models"
)

//go:generate mockgen -source=$GOFILE -destination=mock_db.go -package db --build_flags=--mod=mod -imports=models=github.com/Clever/workflow-manager/gen-go/models

// Interface for interacting with the workflow-manager database.
type Interface interface {
	// SaveWorkflowDefinition saves a WorkflowDefinition to the database.
	SaveWorkflowDefinition(ctx context.Context, m models.WorkflowDefinition) error
	// GetWorkflowDefinition retrieves a WorkflowDefinition from the database.
	GetWorkflowDefinition(ctx context.Context, name string, version int64) (*models.WorkflowDefinition, error)
	// GetWorkflowDefinitionsByNameAndVersion retrieves a page of WorkflowDefinitions from the database.
	GetWorkflowDefinitionsByNameAndVersion(ctx context.Context, input GetWorkflowDefinitionsByNameAndVersionInput, fn func(m *models.WorkflowDefinition, lastWorkflowDefinition bool) bool) error
	// DeleteWorkflowDefinition deletes a WorkflowDefinition from the database.
	DeleteWorkflowDefinition(ctx context.Context, name string, version int64) error
}

// Int64 returns a pointer to the int64 value passed in.
func Int64(i int64) *int64 { return &i }

// String returns a pointer to the string value passed in.
func String(s string) *string { return &s }

// GetWorkflowDefinitionsByNameAndVersionInput is the query input to GetWorkflowDefinitionsByNameAndVersion.
type GetWorkflowDefinitionsByNameAndVersionInput struct {
	// Name is required
	Name              string
	VersionStartingAt *int64
	// StartingAfter is a required specification of an exclusive starting point.
	StartingAfter *models.WorkflowDefinition
	Descending    bool
	// DisableConsistentRead turns off the default behavior of running a consistent read.
	DisableConsistentRead bool
	// Limit is an optional limit of how many items to evaluate.
	Limit *int64
}

// ErrWorkflowDefinitionNotFound is returned when the database fails to find a WorkflowDefinition.
type ErrWorkflowDefinitionNotFound struct {
	Name    string
	Version int64
}

var _ error = ErrWorkflowDefinitionNotFound{}

// Error returns a description of the error.
func (e ErrWorkflowDefinitionNotFound) Error() string {
	return "could not find WorkflowDefinition"
}

// ErrWorkflowDefinitionAlreadyExists is returned when trying to overwrite a WorkflowDefinition.
type ErrWorkflowDefinitionAlreadyExists struct {
	Name    string
	Version int64
}

var _ error = ErrWorkflowDefinitionAlreadyExists{}

// Error returns a description of the error.
func (e ErrWorkflowDefinitionAlreadyExists) Error() string {
	return "WorkflowDefinition already exists"
}
