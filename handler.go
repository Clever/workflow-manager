package main

import (
	"context"
	"fmt"

	"github.com/Clever/workflow-manager/gen-go/models"
	"github.com/Clever/workflow-manager/resources"
	"github.com/Clever/workflow-manager/store"
)

// WorkflowManager implements the wag server
type WorkflowManager struct {
	store store.Store
}

// HealthCheck returns 200 if workflow-manager can respond to requests
func (wm WorkflowManager) HealthCheck(ctx context.Context) error {
	// TODO: check that dependency clients are initialized and working
	// 1. AWS Batch
	// 2. DB
	return nil
}

// NewWorkflow creates a new workflow
func (wm WorkflowManager) NewWorkflow(ctx context.Context, workflowReq *models.NewWorkflowRequest) (*models.Workflow, error) {

	if len(workflowReq.States) == 0 || workflowReq.Name == "" {
		return &models.Workflow{}, fmt.Errorf("Must define at least one state")
	}
	//TODO: validate states

	workflow, err := newWorkflowDefinitionFromRequest(*workflowReq)
	if err != nil {
		return &models.Workflow{}, err
	}
	if err := wm.store.CreateWorkflow(workflow); err != nil {
		return &models.Workflow{}, err
	}

	return &models.Workflow{
		Revision: int64(workflow.Version()),
		Name:     workflow.Name(),
	}, nil
}

func (wm WorkflowManager) GetWorkflowByName(ctx context.Context, name string) (*models.Workflow, error) {
	workflow, err := wm.store.LatestWorkflow(name)
	if err != nil {
		return &models.Workflow{}, err
	}

	return &models.Workflow{
		Name:     workflow.Name(),
		Revision: int64(workflow.Version()),
	}, nil
}

func newWorkflowDefinitionFromRequest(req models.NewWorkflowRequest) (resources.WorkflowDefinition, error) {
	states := map[string]resources.State{}
	for _, s := range req.States {
		states[s.Name] = resources.NewWorkerState(s.Name, s.Next, s.Resource, s.End)
	}

	return resources.NewWorkflowDefinition(req.Name, "description", states)
}
