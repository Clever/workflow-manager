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

	// GetJobsForWorkflow makes a GET request to /jobs
	//
	// 200: []models.Job
	// 400: *models.BadRequest
	// 404: *models.NotFound
	// 500: *models.InternalError
	// default: client side HTTP errors, for example: context.DeadlineExceeded.
	GetJobsForWorkflow(ctx context.Context, i *models.GetJobsForWorkflowInput) ([]models.Job, error)

	// StartJobForWorkflow makes a POST request to /jobs
	//
	// 200: *models.Job
	// 400: *models.BadRequest
	// 404: *models.NotFound
	// 500: *models.InternalError
	// default: client side HTTP errors, for example: context.DeadlineExceeded.
	StartJobForWorkflow(ctx context.Context, i *models.JobInput) (*models.Job, error)

	// GetJob makes a GET request to /jobs/{jobId}
	//
	// 200: *models.Job
	// 400: *models.BadRequest
	// 404: *models.NotFound
	// 500: *models.InternalError
	// default: client side HTTP errors, for example: context.DeadlineExceeded.
	GetJob(ctx context.Context, jobId string) (*models.Job, error)

	// NewWorkflow makes a POST request to /workflows
	//
	// 201: *models.Workflow
	// 400: *models.BadRequest
	// 500: *models.InternalError
	// default: client side HTTP errors, for example: context.DeadlineExceeded.
	NewWorkflow(ctx context.Context, i *models.NewWorkflowRequest) (*models.Workflow, error)

	// GetWorkflowByName makes a GET request to /workflows/{name}
	//
	// 200: *models.Workflow
	// 400: *models.BadRequest
	// 404: *models.NotFound
	// 500: *models.InternalError
	// default: client side HTTP errors, for example: context.DeadlineExceeded.
	GetWorkflowByName(ctx context.Context, name string) (*models.Workflow, error)

	// UpdateWorkflow makes a PUT request to /workflows/{name}
	//
	// 201: *models.Workflow
	// 400: *models.BadRequest
	// 404: *models.NotFound
	// 500: *models.InternalError
	// default: client side HTTP errors, for example: context.DeadlineExceeded.
	UpdateWorkflow(ctx context.Context, i *models.UpdateWorkflowInput) (*models.Workflow, error)
}
