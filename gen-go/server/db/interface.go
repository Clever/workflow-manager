package db

import (
	"context"
	"fmt"

	"github.com/Clever/workflow-manager/gen-go/models"
)

//go:generate $GOPATH/bin/mockgen -source=$GOFILE -destination=mock_db.go -package=db

// Interface for interacting with the workflow-manager database.
type Interface interface {
	// SaveWorkflowDefinition saves a WorkflowDefinition to the database.
	SaveWorkflowDefinition(ctx context.Context, m models.WorkflowDefinition) error
	// GetWorkflowDefinition retrieves a WorkflowDefinition from the database.
	GetWorkflowDefinition(ctx context.Context, name string, version int64) (*models.WorkflowDefinition, error)
	// DeleteWorkflowDefinition deletes a WorkflowDefinition from the database.
	DeleteWorkflowDefinition(ctx context.Context, name string, version int64) error
}

// ErrWorkflowDefinitionNotFound is returned when the database fails to find a WorkflowDefinition.
type ErrWorkflowDefinitionNotFound struct {
	Name    string
	Version int64
}

var _ error = ErrWorkflowDefinitionNotFound{}

// Error returns a description of the error.
func (e ErrWorkflowDefinitionNotFound) Error() string {
	return fmt.Sprintf("could not find WorkflowDefinition: %v", e)
}

// ErrWorkflowDefinitionAlreadyExists is returned when trying to overwrite a WorkflowDefinition.
type ErrWorkflowDefinitionAlreadyExists struct {
	Name    string
	Version int64
}

var _ error = ErrWorkflowDefinitionAlreadyExists{}

// Error returns a description of the error.
func (e ErrWorkflowDefinitionAlreadyExists) Error() string {
	return fmt.Sprintf("WorkflowDefinition already exists: %v", e)
}
