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

	// GetJobsForWorkflow handles GET requests to /jobs
	//
	// 200: []models.Job
	// 400: *models.BadRequest
	// 404: *models.NotFound
	// 500: *models.InternalError
	// default: client side HTTP errors, for example: context.DeadlineExceeded.
	GetJobsForWorkflow(ctx context.Context, i *models.GetJobsForWorkflowInput) ([]models.Job, error)

	// StartJobForWorkflow handles POST requests to /jobs
	//
	// 200: *models.Job
	// 400: *models.BadRequest
	// 404: *models.NotFound
	// 500: *models.InternalError
	// default: client side HTTP errors, for example: context.DeadlineExceeded.
	StartJobForWorkflow(ctx context.Context, i *models.JobInput) (*models.Job, error)

	// CancelJob handles DELETE requests to /jobs/{jobId}
	//
	// 200: nil
	// 400: *models.BadRequest
	// 404: *models.NotFound
	// 500: *models.InternalError
	// default: client side HTTP errors, for example: context.DeadlineExceeded.
	CancelJob(ctx context.Context, i *models.CancelJobInput) error

	// GetJob handles GET requests to /jobs/{jobId}
	//
	// 200: *models.Job
	// 400: *models.BadRequest
	// 404: *models.NotFound
	// 500: *models.InternalError
	// default: client side HTTP errors, for example: context.DeadlineExceeded.
	GetJob(ctx context.Context, jobId string) (*models.Job, error)

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

	// GetWorkflows handles GET requests to /workflows
	// Get the latest versions of all available workflows
	// 200: []models.Workflow
	// 400: *models.BadRequest
	// 500: *models.InternalError
	// default: client side HTTP errors, for example: context.DeadlineExceeded.
	GetWorkflows(ctx context.Context) ([]models.Workflow, error)

	// NewWorkflow handles POST requests to /workflows
	//
	// 201: *models.Workflow
	// 400: *models.BadRequest
	// 500: *models.InternalError
	// default: client side HTTP errors, for example: context.DeadlineExceeded.
	NewWorkflow(ctx context.Context, i *models.NewWorkflowRequest) (*models.Workflow, error)

	// GetWorkflowVersionsByName handles GET requests to /workflows/{name}
	//
	// 200: []models.Workflow
	// 400: *models.BadRequest
	// 404: *models.NotFound
	// 500: *models.InternalError
	// default: client side HTTP errors, for example: context.DeadlineExceeded.
	GetWorkflowVersionsByName(ctx context.Context, i *models.GetWorkflowVersionsByNameInput) ([]models.Workflow, error)

	// UpdateWorkflow handles PUT requests to /workflows/{name}
	//
	// 201: *models.Workflow
	// 400: *models.BadRequest
	// 404: *models.NotFound
	// 500: *models.InternalError
	// default: client side HTTP errors, for example: context.DeadlineExceeded.
	UpdateWorkflow(ctx context.Context, i *models.UpdateWorkflowInput) (*models.Workflow, error)

	// GetWorkflowByNameAndVersion handles GET requests to /workflows/{name}/{version}
	//
	// 200: *models.Workflow
	// 400: *models.BadRequest
	// 404: *models.NotFound
	// 500: *models.InternalError
	// default: client side HTTP errors, for example: context.DeadlineExceeded.
	GetWorkflowByNameAndVersion(ctx context.Context, i *models.GetWorkflowByNameAndVersionInput) (*models.Workflow, error)
}
