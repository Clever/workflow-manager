package server

import (
	"context"

	"github.com/Clever/workflow-manager/gen-go/models"
)

//go:generate $GOPATH/bin/mockgen -source=$GOFILE -destination=mock_controller.go -package=server

// Controller defines the interface for the workflow-manager service.
type Controller interface {

	// HealthCheck handles GET requests to /_health
	// Checks if the service is healthy
	// 200: nil
	// 400: *models.BadRequest
	// 500: *models.InternalError
	// default: client side HTTP errors, for example: context.DeadlineExceeded.
	HealthCheck(ctx context.Context) error

	// PostStateResource handles POST requests to /state-resources
	//
	// 201: *models.StateResource
	// 400: *models.BadRequest
	// 500: *models.InternalError
	// default: client side HTTP errors, for example: context.DeadlineExceeded.
	PostStateResource(ctx context.Context, i *models.NewStateResource) (*models.StateResource, error)

	// DeleteStateResource handles DELETE requests to /state-resources/{namespace}/{name}
	//
	// 200: nil
	// 400: *models.BadRequest
	// 404: *models.NotFound
	// 500: *models.InternalError
	// default: client side HTTP errors, for example: context.DeadlineExceeded.
	DeleteStateResource(ctx context.Context, i *models.DeleteStateResourceInput) error

	// GetStateResource handles GET requests to /state-resources/{namespace}/{name}
	//
	// 200: *models.StateResource
	// 400: *models.BadRequest
	// 404: *models.NotFound
	// 500: *models.InternalError
	// default: client side HTTP errors, for example: context.DeadlineExceeded.
	GetStateResource(ctx context.Context, i *models.GetStateResourceInput) (*models.StateResource, error)

	// PutStateResource handles PUT requests to /state-resources/{namespace}/{name}
	//
	// 201: *models.StateResource
	// 400: *models.BadRequest
	// 500: *models.InternalError
	// default: client side HTTP errors, for example: context.DeadlineExceeded.
	PutStateResource(ctx context.Context, i *models.PutStateResourceInput) (*models.StateResource, error)

	// GetWorkflowDefinitions handles GET requests to /workflow-definitions
	// Get the latest versions of all available WorkflowDefinitions
	// 200: []models.WorkflowDefinition
	// 400: *models.BadRequest
	// 500: *models.InternalError
	// default: client side HTTP errors, for example: context.DeadlineExceeded.
	GetWorkflowDefinitions(ctx context.Context) ([]models.WorkflowDefinition, error)

	// NewWorkflowDefinition handles POST requests to /workflow-definitions
	//
	// 201: *models.WorkflowDefinition
	// 400: *models.BadRequest
	// 500: *models.InternalError
	// default: client side HTTP errors, for example: context.DeadlineExceeded.
	NewWorkflowDefinition(ctx context.Context, i *models.NewWorkflowDefinitionRequest) (*models.WorkflowDefinition, error)

	// GetWorkflowDefinitionVersionsByName handles GET requests to /workflow-definitions/{name}
	//
	// 200: []models.WorkflowDefinition
	// 400: *models.BadRequest
	// 404: *models.NotFound
	// 500: *models.InternalError
	// default: client side HTTP errors, for example: context.DeadlineExceeded.
	GetWorkflowDefinitionVersionsByName(ctx context.Context, i *models.GetWorkflowDefinitionVersionsByNameInput) ([]models.WorkflowDefinition, error)

	// UpdateWorkflowDefinition handles PUT requests to /workflow-definitions/{name}
	//
	// 201: *models.WorkflowDefinition
	// 400: *models.BadRequest
	// 404: *models.NotFound
	// 500: *models.InternalError
	// default: client side HTTP errors, for example: context.DeadlineExceeded.
	UpdateWorkflowDefinition(ctx context.Context, i *models.UpdateWorkflowDefinitionInput) (*models.WorkflowDefinition, error)

	// GetWorkflowDefinitionByNameAndVersion handles GET requests to /workflow-definitions/{name}/{version}
	//
	// 200: *models.WorkflowDefinition
	// 400: *models.BadRequest
	// 404: *models.NotFound
	// 500: *models.InternalError
	// default: client side HTTP errors, for example: context.DeadlineExceeded.
	GetWorkflowDefinitionByNameAndVersion(ctx context.Context, i *models.GetWorkflowDefinitionByNameAndVersionInput) (*models.WorkflowDefinition, error)

	// GetWorkflows handles GET requests to /workflows
	//
	// 200: []models.Workflow
	// 400: *models.BadRequest
	// 404: *models.NotFound
	// 500: *models.InternalError
	// default: client side HTTP errors, for example: context.DeadlineExceeded.
	GetWorkflows(ctx context.Context, i *models.GetWorkflowsInput) ([]models.Workflow, error)

	// StartWorkflow handles POST requests to /workflows
	//
	// 200: *models.Workflow
	// 400: *models.BadRequest
	// 404: *models.NotFound
	// 500: *models.InternalError
	// default: client side HTTP errors, for example: context.DeadlineExceeded.
	StartWorkflow(ctx context.Context, i *models.StartWorkflowParams) (*models.Workflow, error)

	// CancelWorkflow handles DELETE requests to /workflows/{workflowId}
	//
	// 200: nil
	// 400: *models.BadRequest
	// 404: *models.NotFound
	// 500: *models.InternalError
	// default: client side HTTP errors, for example: context.DeadlineExceeded.
	CancelWorkflow(ctx context.Context, i *models.CancelWorkflowInput) error

	// GetWorkflowByID handles GET requests to /workflows/{workflowId}
	//
	// 200: *models.Workflow
	// 400: *models.BadRequest
	// 404: *models.NotFound
	// 500: *models.InternalError
	// default: client side HTTP errors, for example: context.DeadlineExceeded.
	GetWorkflowByID(ctx context.Context, workflowId string) (*models.Workflow, error)
}
