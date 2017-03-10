package main

import (
	"context"
	"fmt"

	"github.com/Clever/workflow-manager/executor"
	"github.com/Clever/workflow-manager/gen-go/models"
	"github.com/Clever/workflow-manager/resources"
	"github.com/Clever/workflow-manager/store"
)

// WorkflowManager implements the wag server
type WorkflowManager struct {
	store   store.Store
	manager executor.JobManager
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

	workflow, err := newWorkflowFromRequest(*workflowReq)
	if err != nil {
		return &models.Workflow{}, err
	}
	if err := wm.store.CreateWorkflow(workflow); err != nil {
		return &models.Workflow{}, err
	}

	return apiWorkflowFromStore(workflow), nil
}

// GetWorkflowByName allows fetching an existing Workflow by providing it's name
func (wm WorkflowManager) GetWorkflowByName(ctx context.Context, name string) (*models.Workflow, error) {
	workflow, err := wm.store.LatestWorkflow(name)
	if err != nil {
		return &models.Workflow{}, err
	}

	return apiWorkflowFromStore(workflow), nil
}

// StartJobForWorkflow starts a new Job for the given workflow
func (wm WorkflowManager) StartJobForWorkflow(ctx context.Context, req *models.StartJobForWorkflowInput) (*models.Job, error) {
	workflow, err := wm.store.LatestWorkflow(req.WorkflowName)
	if err != nil {
		return &models.Job{}, err
	}

	data := ""
	if req.Input != nil {
		data = req.Input.Data
	}

	job, err := wm.manager.CreateJob(workflow, data)
	if err != nil {
		return &models.Job{}, err
	}

	// TODO: Don't actually store at this point, but should be done earlier. If saving
	// fails we should either
	// 1. reconcile somehow with the scheduled tasks
	// 2. kill the running tasks so that we don't have orphan tasks in AWS Batch
	err = wm.store.CreateJob(*job)
	if err != nil {
		return &models.Job{}, err
	}

	return apiJobFromStore(*job), nil
}

// GetJobsForWorkflow returns a summary of all active jobs for the given workflow
func (wm WorkflowManager) GetJobsForWorkflow(ctx context.Context, workflowName string) ([]models.Job, error) {
	return []models.Job{}, nil
}

// TODO: the functions below should probably just functions on the respective resources.<Struct>

func newWorkflowFromRequest(req models.NewWorkflowRequest) (resources.WorkflowDefinition, error) {
	states := map[string]resources.State{}
	for _, s := range req.States {
		workerState, err := resources.NewWorkerState(s.Name, s.Next, s.Resource, s.End)
		if err != nil {
			return resources.WorkflowDefinition{}, err
		}
		states[workerState.Name()] = workerState
	}

	// fill in dependencies for states
	for _, s := range states {
		if !s.IsEnd() {
			states[s.Next()].AddDependency(s)
		}
	}

	return resources.NewWorkflowDefinition(req.Name, "description", req.StartAt, states)
}

func apiWorkflowFromStore(wf resources.Workflow) *models.Workflow {
	states := []*models.State{}
	for _, s := range wf.States() {
		states = append(states, &models.State{
			Resource: s.Resource(),
			Name:     s.Name(),
			Next:     s.Next(),
			End:      s.IsEnd(),
			Type:     s.Type(),
		})
	}

	return &models.Workflow{
		Name:     wf.Name(),
		Revision: int64(wf.Version()),
		States:   states,
	}
}

func apiJobFromStore(job resources.Job) *models.Job {
	tasks := []*models.Task{}
	for _, task := range job.Tasks {
		tasks = append(tasks, &models.Task{
			ID:    task.ID,
			State: task.State,
			//Status: TODO: wat to do about status here
		})
	}

	return &models.Job{
		ID:       job.ID,
		Tasks:    tasks,
		Workflow: apiWorkflowFromStore(job.Workflow),
	}

}
