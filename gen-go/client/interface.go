package client

import (
	"context"

	"github.com/Clever/workflow-manager/gen-go/models"
)

//go:generate mockgen -source=$GOFILE -destination=mock_client.go -package client --build_flags=--mod=mod

// Client defines the methods available to clients of the workflow-manager service.
type Client interface {

	// HealthCheck makes a GET request to /_health
	// Checks if the service is healthy
	// 200: nil
	// 400: *models.BadRequest
	// 500: *models.InternalError
	// default: client side HTTP errors, for example: context.DeadlineExceeded.
	HealthCheck(ctx context.Context) error

	// PostStateResource makes a POST request to /state-resources
	//
	// 201: *models.StateResource
	// 400: *models.BadRequest
	// 500: *models.InternalError
	// default: client side HTTP errors, for example: context.DeadlineExceeded.
	PostStateResource(ctx context.Context, i *models.NewStateResource) (*models.StateResource, error)

	// DeleteStateResource makes a DELETE request to /state-resources/{namespace}/{name}
	//
	// 200: nil
	// 400: *models.BadRequest
	// 404: *models.NotFound
	// 500: *models.InternalError
	// default: client side HTTP errors, for example: context.DeadlineExceeded.
	DeleteStateResource(ctx context.Context, i *models.DeleteStateResourceInput) error

	// GetStateResource makes a GET request to /state-resources/{namespace}/{name}
	//
	// 200: *models.StateResource
	// 400: *models.BadRequest
	// 404: *models.NotFound
	// 500: *models.InternalError
	// default: client side HTTP errors, for example: context.DeadlineExceeded.
	GetStateResource(ctx context.Context, i *models.GetStateResourceInput) (*models.StateResource, error)

	// PutStateResource makes a PUT request to /state-resources/{namespace}/{name}
	//
	// 201: *models.StateResource
	// 400: *models.BadRequest
	// 500: *models.InternalError
	// default: client side HTTP errors, for example: context.DeadlineExceeded.
	PutStateResource(ctx context.Context, i *models.PutStateResourceInput) (*models.StateResource, error)

	// GetWorkflowDefinitions makes a GET request to /workflow-definitions
	// Get the latest versions of all available WorkflowDefinitions
	// 200: []models.WorkflowDefinition
	// 400: *models.BadRequest
	// 500: *models.InternalError
	// default: client side HTTP errors, for example: context.DeadlineExceeded.
	GetWorkflowDefinitions(ctx context.Context) ([]models.WorkflowDefinition, error)

	// NewWorkflowDefinition makes a POST request to /workflow-definitions
	//
	// 201: *models.WorkflowDefinition
	// 400: *models.BadRequest
	// 500: *models.InternalError
	// default: client side HTTP errors, for example: context.DeadlineExceeded.
	NewWorkflowDefinition(ctx context.Context, i *models.NewWorkflowDefinitionRequest) (*models.WorkflowDefinition, error)

	// GetWorkflowDefinitionVersionsByName makes a GET request to /workflow-definitions/{name}
	//
	// 200: []models.WorkflowDefinition
	// 400: *models.BadRequest
	// 404: *models.NotFound
	// 500: *models.InternalError
	// default: client side HTTP errors, for example: context.DeadlineExceeded.
	GetWorkflowDefinitionVersionsByName(ctx context.Context, i *models.GetWorkflowDefinitionVersionsByNameInput) ([]models.WorkflowDefinition, error)

	// UpdateWorkflowDefinition makes a PUT request to /workflow-definitions/{name}
	//
	// 201: *models.WorkflowDefinition
	// 400: *models.BadRequest
	// 404: *models.NotFound
	// 500: *models.InternalError
	// default: client side HTTP errors, for example: context.DeadlineExceeded.
	UpdateWorkflowDefinition(ctx context.Context, i *models.UpdateWorkflowDefinitionInput) (*models.WorkflowDefinition, error)

	// GetWorkflowDefinitionByNameAndVersion makes a GET request to /workflow-definitions/{name}/{version}
	//
	// 200: *models.WorkflowDefinition
	// 400: *models.BadRequest
	// 404: *models.NotFound
	// 500: *models.InternalError
	// default: client side HTTP errors, for example: context.DeadlineExceeded.
	GetWorkflowDefinitionByNameAndVersion(ctx context.Context, i *models.GetWorkflowDefinitionByNameAndVersionInput) (*models.WorkflowDefinition, error)

	// GetWorkflows makes a GET request to /workflows
	//
	// 200: []models.Workflow
	// 400: *models.BadRequest
	// 404: *models.NotFound
	// 500: *models.InternalError
	// default: client side HTTP errors, for example: context.DeadlineExceeded.
	GetWorkflows(ctx context.Context, i *models.GetWorkflowsInput) ([]models.Workflow, error)

	NewGetWorkflowsIter(ctx context.Context, i *models.GetWorkflowsInput) (GetWorkflowsIter, error)

	// StartWorkflow makes a POST request to /workflows
	//
	// 200: *models.Workflow
	// 400: *models.BadRequest
	// 404: *models.NotFound
	// 500: *models.InternalError
	// default: client side HTTP errors, for example: context.DeadlineExceeded.
	StartWorkflow(ctx context.Context, i *models.StartWorkflowRequest) (*models.Workflow, error)

	// CancelWorkflow makes a DELETE request to /workflows/{workflowID}
	//
	// 200: nil
	// 400: *models.BadRequest
	// 404: *models.NotFound
	// 500: *models.InternalError
	// default: client side HTTP errors, for example: context.DeadlineExceeded.
	CancelWorkflow(ctx context.Context, i *models.CancelWorkflowInput) error

	// GetWorkflowByID makes a GET request to /workflows/{workflowID}
	//
	// 200: *models.Workflow
	// 400: *models.BadRequest
	// 404: *models.NotFound
	// 500: *models.InternalError
	// default: client side HTTP errors, for example: context.DeadlineExceeded.
	GetWorkflowByID(ctx context.Context, workflowID string) (*models.Workflow, error)

	// ResumeWorkflowByID makes a POST request to /workflows/{workflowID}
	//
	// 200: *models.Workflow
	// 400: *models.BadRequest
	// 404: *models.NotFound
	// 500: *models.InternalError
	// default: client side HTTP errors, for example: context.DeadlineExceeded.
	ResumeWorkflowByID(ctx context.Context, i *models.ResumeWorkflowByIDInput) (*models.Workflow, error)

	// ResolveWorkflowByID makes a POST request to /workflows/{workflowID}/resolved
	//
	// 201: nil
	// 400: *models.BadRequest
	// 404: *models.NotFound
	// 409: *models.Conflict
	// 500: *models.InternalError
	// default: client side HTTP errors, for example: context.DeadlineExceeded.
	ResolveWorkflowByID(ctx context.Context, workflowID string) error
}

// GetWorkflowsIter defines the methods available on GetWorkflows iterators.
type GetWorkflowsIter interface {
	Next(*models.Workflow) bool
	Err() error
}
