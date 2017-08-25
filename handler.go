package main

import (
	"context"
	"fmt"

	"github.com/Clever/workflow-manager/executor"
	"github.com/Clever/workflow-manager/gen-go/models"
	"github.com/Clever/workflow-manager/resources"
	"github.com/Clever/workflow-manager/store"
	"github.com/go-openapi/strfmt"
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
	//TODO: validate states
	if len(workflowReq.States) == 0 {
		return &models.Workflow{}, fmt.Errorf("Must define at least one state")
	}
	if workflowReq.Name == "" {
		return &models.Workflow{}, fmt.Errorf("Workflow `name` is required")
	}

	workflow, err := newWorkflowFromRequest(*workflowReq)
	if err != nil {
		return &models.Workflow{}, err
	}

	if err := wm.store.SaveWorkflow(workflow); err != nil {
		return &models.Workflow{}, err
	}

	return apiWorkflowFromStore(workflow), nil
}

// UpdateWorkflow creates a new revision for an existing workflow
func (wm WorkflowManager) UpdateWorkflow(ctx context.Context, input *models.UpdateWorkflowInput) (*models.Workflow, error) {
	workflowReq := input.NewWorkflowRequest
	if workflowReq == nil || workflowReq.Name != input.Name {
		return &models.Workflow{}, fmt.Errorf("Name in path must match Workflow object")
	}
	if len(workflowReq.States) == 0 {
		return &models.Workflow{}, fmt.Errorf("Must define at least one state")
	}

	workflow, err := newWorkflowFromRequest(*workflowReq)
	if err != nil {
		return &models.Workflow{}, err
	}

	workflow, err = wm.store.UpdateWorkflow(workflow)
	if err != nil {
		return &models.Workflow{}, err
	}

	return apiWorkflowFromStore(workflow), nil
}

// GetWorkflows retrieves a list of the latest version of each workflow
func (wm WorkflowManager) GetWorkflows(ctx context.Context) ([]models.Workflow, error) {
	workflows, err := wm.store.GetWorkflows()

	if err != nil {
		return []models.Workflow{}, err
	}
	apiWorkflows := []models.Workflow{}
	for _, workflow := range workflows {
		apiWorkflow := apiWorkflowFromStore(workflow)
		apiWorkflows = append(apiWorkflows, *apiWorkflow)
	}
	return apiWorkflows, nil
}

// GetWorkflowVersionsByName fetches either:
//  1. A list of all versions of a workflow by name
//  2. The most recent version of a workflow by name
func (wm WorkflowManager) GetWorkflowVersionsByName(ctx context.Context, input *models.GetWorkflowVersionsByNameInput) ([]models.Workflow, error) {
	if *input.Latest == true {
		workflow, err := wm.store.LatestWorkflow(input.Name)
		if err != nil {
			return []models.Workflow{}, err
		}
		return []models.Workflow{*(apiWorkflowFromStore(workflow))}, nil
	}

	apiWorkflows := []models.Workflow{}
	workflows, err := wm.store.GetWorkflowVersions(input.Name)
	if err != nil {
		return []models.Workflow{}, err
	}
	for _, workflow := range workflows {
		apiWorkflow := apiWorkflowFromStore(workflow)
		apiWorkflows = append(apiWorkflows, *apiWorkflow)
	}

	return apiWorkflows, nil
}

// GetWorkflowByNameAndVersion allows fetching an existing Workflow by providing it's name and version
func (wm WorkflowManager) GetWorkflowByNameAndVersion(ctx context.Context, input *models.GetWorkflowByNameAndVersionInput) (*models.Workflow, error) {
	workflow, err := wm.store.GetWorkflow(input.Name, int(input.Version))
	if err != nil {
		return &models.Workflow{}, err
	}
	return apiWorkflowFromStore(workflow), nil
}

// PostStateResource creates a new state resource
func (wm WorkflowManager) PostStateResource(ctx context.Context, i *models.NewStateResource) (*models.StateResource, error) {
	stateResource := resources.NewBatchResource(i.Name, i.Namespace, i.URI)
	if err := wm.store.SaveStateResource(stateResource); err != nil {
		return &models.StateResource{}, err
	}

	return apiStateResourceFromStore(stateResource), nil
}

// PutStateResource creates or updates a state resource
func (wm WorkflowManager) PutStateResource(ctx context.Context, i *models.PutStateResourceInput) (*models.StateResource, error) {
	if i.Name != i.NewStateResource.Name {
		return &models.StateResource{}, models.BadRequest{
			Message: "StateResource.Name does not match name in path",
		}
	}
	if i.Namespace != i.NewStateResource.Namespace {
		return &models.StateResource{}, models.BadRequest{
			Message: "StateResource.Namespace does not match namespace in path",
		}
	}

	stateResource := resources.NewBatchResource(
		i.NewStateResource.Name, i.NewStateResource.Namespace, i.NewStateResource.URI)
	if err := wm.store.SaveStateResource(stateResource); err != nil {
		return &models.StateResource{}, err
	}

	return apiStateResourceFromStore(stateResource), nil
}

// GetStateResource fetches a StateResource given a name and namespace
func (wm WorkflowManager) GetStateResource(ctx context.Context, i *models.GetStateResourceInput) (*models.StateResource, error) {
	stateResource, err := wm.store.GetStateResource(i.Name, i.Namespace)
	if err != nil {
		return &models.StateResource{}, err
	}

	return apiStateResourceFromStore(stateResource), nil
}

// DeleteStateResource removes a StateResource given a name and namespace
func (wm WorkflowManager) DeleteStateResource(ctx context.Context, i *models.DeleteStateResourceInput) error {
	return wm.store.DeleteStateResource(i.Name, i.Namespace)
}

// StartJobForWorkflow starts a new Job for the given workflow
func (wm WorkflowManager) StartJobForWorkflow(ctx context.Context, input *models.JobInput) (*models.Job, error) {
	var workflow resources.WorkflowDefinition
	var err error
	if input.Workflow.Revision < 0 {
		workflow, err = wm.store.LatestWorkflow(input.Workflow.Name)
	} else {
		workflow, err = wm.store.GetWorkflow(input.Workflow.Name, int(input.Workflow.Revision))
	}
	if err != nil {
		return &models.Job{}, err
	}

	var data []string
	if input.Data != nil {
		// convert from []interface{} to []string (i.e. flattened json string array)
		data = jsonToArgs(input.Data)
	}

	job, err := wm.manager.CreateJob(workflow, data, input.Namespace, input.Queue)
	if err != nil {
		return &models.Job{}, err
	}

	return apiJobFromStore(*job), nil
}

// GetJobsForWorkflow returns a summary of all active jobs for the given workflow
func (wm WorkflowManager) GetJobsForWorkflow(ctx context.Context, input *models.GetJobsForWorkflowInput) ([]models.Job, error) {
	jobs, err := wm.store.GetJobsForWorkflow(input.WorkflowName)
	if err != nil {
		return []models.Job{}, err
	}

	results := []models.Job{}
	for _, job := range jobs {
		wm.manager.UpdateJobStatus(&job)
		results = append(results, *apiJobFromStore(job))
	}
	return results, nil
}

// GetJob returns current details about a Job with the given jobId
func (wm WorkflowManager) GetJob(ctx context.Context, jobID string) (*models.Job, error) {
	job, err := wm.store.GetJob(jobID)
	if err != nil {
		return &models.Job{}, err
	}

	err = wm.manager.UpdateJobStatus(&job)
	if err != nil {
		return &models.Job{}, err
	}

	return apiJobFromStore(job), nil
}

// CancelJob cancels all the tasks currently running or queued for the Job and
// marks the job as cancelled
func (wm WorkflowManager) CancelJob(ctx context.Context, input *models.CancelJobInput) error {
	job, err := wm.store.GetJob(input.JobId)
	if err != nil {
		return err
	}

	return wm.manager.CancelJob(&job, input.Reason.Reason)
}

func jsonToArgs(data []interface{}) []string {
	args := []string{}
	for _, v := range data {
		if arg, ok := v.(string); ok {
			args = append(args, arg)
		}
	}
	return args
}

// TODO: the functions below should probably just be functions on the respective resources.<Struct>

func newWorkflowFromRequest(req models.NewWorkflowRequest) (resources.WorkflowDefinition, error) {
	if req.StartAt == "" {
		return resources.WorkflowDefinition{}, fmt.Errorf("startAt is a required field")
	}

	states := map[string]resources.State{}
	for _, s := range req.States {
		if s.Type != "" && s.Type != "Task" {
			return resources.WorkflowDefinition{}, fmt.Errorf("Only States of `type=Task` are supported")
		}
		workerState, err := resources.NewWorkerState(s.Name, s.Next, s.Resource, s.End, s.Retry)
		if err != nil {
			return resources.WorkflowDefinition{}, err
		}
		states[workerState.Name()] = workerState
	}

	if _, ok := states[req.StartAt]; !ok {
		return resources.WorkflowDefinition{}, fmt.Errorf("startAt state %s not defined", req.StartAt)
	}

	// fill in dependencies for states
	for _, s := range states {
		if !s.IsEnd() {
			if _, ok := states[s.Next()]; !ok {
				return resources.WorkflowDefinition{}, fmt.Errorf("%s.Next=%s, but %s not defined",
					s.Name(), s.Next(), s.Next())
			}
			states[s.Next()].AddDependency(s)
		}
	}

	return resources.NewWorkflowDefinition(req.Name, req.Description, req.StartAt, states)
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
			Retry:    s.Retry(),
		})
	}

	return &models.Workflow{
		Name:      wf.Name(),
		Revision:  int64(wf.Version()),
		StartAt:   wf.StartAt().Name(),
		CreatedAt: strfmt.DateTime(wf.CreatedAt()),
		States:    states,
	}
}

func apiJobFromStore(job resources.Job) *models.Job {
	tasks := []*models.Task{}
	for _, task := range job.Tasks {
		tasks = append(tasks, &models.Task{
			ID:           task.ID,
			CreatedAt:    strfmt.DateTime(task.CreatedAt),
			StartedAt:    strfmt.DateTime(task.StartedAt),
			StoppedAt:    strfmt.DateTime(task.StoppedAt),
			State:        task.State,
			Status:       string(task.Status),
			StatusReason: task.StatusReason,
			Container:    task.ContainerId,
		})
	}

	return &models.Job{
		ID:          job.ID,
		CreatedAt:   strfmt.DateTime(job.CreatedAt),
		LastUpdated: strfmt.DateTime(job.LastUpdated),
		Tasks:       tasks,
		Workflow:    apiWorkflowFromStore(job.Workflow),
		Status:      string(job.Status),
	}
}

func apiStateResourceFromStore(stateResource resources.StateResource) *models.StateResource {
	return &models.StateResource{
		Name:        stateResource.Name,
		Namespace:   stateResource.Namespace,
		URI:         stateResource.URI,
		LastUpdated: strfmt.DateTime(stateResource.LastUpdated),
		Type:        stateResource.Type,
	}
}
