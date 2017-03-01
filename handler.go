package main

import (
	"context"

	"github.com/Clever/workflow-manager/gen-go/models"
)

// WorkflowManager implements the wag server
type WorkflowManager struct{}

// HealthCheck returns 200 if workflow-manager can respond to requests
func (wm WorkflowManager) HealthCheck(ctx context.Context) error {
	// TODO: check that dependency clients are initialized and working
	// 1. AWS Batch
	// 2. DB
	return nil
}

// NewWorkflow creates a new workflow
func (wm WorkflowManager) NewWorkflow(ctx context.Context) (*models.NewWorkflowResponse, error) {

	// TODO: actually do something
	return &models.NewWorkflowResponse{}, nil

}
