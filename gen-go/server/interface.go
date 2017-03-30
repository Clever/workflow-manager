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

	// GetJobsForWorkflow handles GET requests to /jobs/{workflowName}
	//
	// 200: []models.Job
	// 400: *models.BadRequest
	// 404: *models.NotFound
	// 500: *models.InternalError
	// default: client side HTTP errors, for example: context.DeadlineExceeded.
	GetJobsForWorkflow(ctx context.Context, workflowName string) ([]models.Job, error)

	// StartJobForWorkflow handles PUT requests to /jobs/{workflowName}
	//
	// 200: *models.Job
	// 400: *models.BadRequest
	// 404: *models.NotFound
	// 500: *models.InternalError
	// default: client side HTTP errors, for example: context.DeadlineExceeded.
	StartJobForWorkflow(ctx context.Context, i *models.StartJobForWorkflowInput) (*models.Job, error)

	// GetJob handles GET requests to /jobs/{workflowName}/{jobId}
	//
	// 200: *models.Job
	// 400: *models.BadRequest
	// 404: *models.NotFound
	// 500: *models.InternalError
	// default: client side HTTP errors, for example: context.DeadlineExceeded.
	GetJob(ctx context.Context, i *models.GetJobInput) (*models.Job, error)

	// NewWorkflow handles POST requests to /workflows
	//
	// 201: *models.Workflow
	// 400: *models.BadRequest
	// 500: *models.InternalError
	// default: client side HTTP errors, for example: context.DeadlineExceeded.
	NewWorkflow(ctx context.Context, i *models.NewWorkflowRequest) (*models.Workflow, error)

	// GetWorkflowByName handles GET requests to /workflows/{name}
	//
	// 200: *models.Workflow
	// 400: *models.BadRequest
	// 404: *models.NotFound
	// 500: *models.InternalError
	// default: client side HTTP errors, for example: context.DeadlineExceeded.
	GetWorkflowByName(ctx context.Context, name string) (*models.Workflow, error)

	// UpdateWorkflow handles PUT requests to /workflows/{name}
	//
	// 201: *models.Workflow
	// 400: *models.BadRequest
	// 404: *models.NotFound
	// 500: *models.InternalError
	// default: client side HTTP errors, for example: context.DeadlineExceeded.
	UpdateWorkflow(ctx context.Context, i *models.UpdateWorkflowInput) (*models.Workflow, error)
}
