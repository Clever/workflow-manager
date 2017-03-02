package main

import (
	"context"
	"fmt"

	"github.com/Clever/workflow-manager/gen-go/models"
	"github.com/Clever/workflow-manager/resources"
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
func (wm WorkflowManager) NewWorkflow(ctx context.Context, workflowReq *models.NewWorkflowRequest) (*models.NewWorkflowResponse, error) {

	if len(workflowReq.States) == 0 || workflowReq.Name == "" {
		return &models.NewWorkflowResponse{}, fmt.Errorf("Must define at least one state")
	}
	//TODO: validate states

	workflow := newWorkflowDefinitionFromRequest(*workflowReq)
	fmt.Println(workflow)

	return &models.NewWorkflowResponse{
		Revision: int64(workflow.Version()),
		Name:     workflow.Name(),
	}, nil
}

func newWorkflowDefinitionFromRequest(req models.NewWorkflowRequest) resources.WorkflowDefinition {
	states := map[string]resources.State{}
	for _, s := range req.States {
		states[s.Name] = resources.NewWorkerState(s.Name, s.Next, s.Resource, s.End)
	}

	return resources.NewWorkflowDefinition(req.Name, "description", states)
}
