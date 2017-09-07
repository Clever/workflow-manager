package client

import (
	"context"

	"github.com/Clever/workflow-manager/gen-go/models"
)

//go:generate $GOPATH/bin/mockgen -source=$GOFILE -destination=mock_client.go -package=client

// Client defines the methods available to clients of the workflow-manager service.
type Client interface {

	// HealthCheck makes a GET request to /_health
	// Checks if the service is healthy
	// 200: nil
	// 400: *models.BadRequest
	// 500: *models.InternalError
	// default: client side HTTP errors, for example: context.DeadlineExceeded.
	HealthCheck(ctx context.Context) error

	// GetJobsForWorkflowDefinition makes a GET request to /jobs
	//
	// 200: []models.Job
	// 400: *models.BadRequest
	// 404: *models.NotFound
	// 500: *models.InternalError
	// default: client side HTTP errors, for example: context.DeadlineExceeded.
	GetJobsForWorkflowDefinition(ctx context.Context, i *models.GetJobsForWorkflowDefinitionInput) ([]models.Job, error)

	// StartJobForWorkflowDefinition makes a POST request to /jobs
	//
	// 200: *models.Job
	// 400: *models.BadRequest
	// 404: *models.NotFound
	// 500: *models.InternalError
	// default: client side HTTP errors, for example: context.DeadlineExceeded.
	StartJobForWorkflowDefinition(ctx context.Context, i *models.JobInput) (*models.Job, error)

	// CancelJob makes a DELETE request to /jobs/{jobId}
	//
	// 200: nil
	// 400: *models.BadRequest
	// 404: *models.NotFound
	// 500: *models.InternalError
	// default: client side HTTP errors, for example: context.DeadlineExceeded.
	CancelJob(ctx context.Context, i *models.CancelJobInput) error

	// GetJob makes a GET request to /jobs/{jobId}
	//
	// 200: *models.Job
	// 400: *models.BadRequest
	// 404: *models.NotFound
	// 500: *models.InternalError
	// default: client side HTTP errors, for example: context.DeadlineExceeded.
	GetJob(ctx context.Context, jobId string) (*models.Job, error)

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

	// GetWorkflowDefinitions makes a GET request to /workflows
	// Get the latest versions of all available WorkflowDefinitions
	// 200: []models.WorkflowDefinition
	// 400: *models.BadRequest
	// 500: *models.InternalError
	// default: client side HTTP errors, for example: context.DeadlineExceeded.
	GetWorkflowDefinitions(ctx context.Context) ([]models.WorkflowDefinition, error)

	// NewWorkflowDefinition makes a POST request to /workflows
	//
	// 201: *models.WorkflowDefinition
	// 400: *models.BadRequest
	// 500: *models.InternalError
	// default: client side HTTP errors, for example: context.DeadlineExceeded.
	NewWorkflowDefinition(ctx context.Context, i *models.NewWorkflowDefinitionRequest) (*models.WorkflowDefinition, error)

	// GetWorkflowDefinitionVersionsByName makes a GET request to /workflows/{name}
	//
	// 200: []models.WorkflowDefinition
	// 400: *models.BadRequest
	// 404: *models.NotFound
	// 500: *models.InternalError
	// default: client side HTTP errors, for example: context.DeadlineExceeded.
	GetWorkflowDefinitionVersionsByName(ctx context.Context, i *models.GetWorkflowDefinitionVersionsByNameInput) ([]models.WorkflowDefinition, error)

	// UpdateWorkflowDefinition makes a PUT request to /workflows/{name}
	//
	// 201: *models.WorkflowDefinition
	// 400: *models.BadRequest
	// 404: *models.NotFound
	// 500: *models.InternalError
	// default: client side HTTP errors, for example: context.DeadlineExceeded.
	UpdateWorkflowDefinition(ctx context.Context, i *models.UpdateWorkflowDefinitionInput) (*models.WorkflowDefinition, error)

	// GetWorkflowDefinitionByNameAndVersion makes a GET request to /workflows/{name}/{version}
	//
	// 200: *models.WorkflowDefinition
	// 400: *models.BadRequest
	// 404: *models.NotFound
	// 500: *models.InternalError
	// default: client side HTTP errors, for example: context.DeadlineExceeded.
	GetWorkflowDefinitionByNameAndVersion(ctx context.Context, i *models.GetWorkflowDefinitionByNameAndVersionInput) (*models.WorkflowDefinition, error)
}
