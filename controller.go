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

// controller implements the wag Controller
type controller struct {
	store   store.Store
	manager executor.WorkflowManager
}

// HealthCheck returns 200 if workflow-manager can respond to requests
func (c controller) HealthCheck(ctx context.Context) error {
	// TODO: check that dependency clients are initialized and working
	// 1. AWS Batch
	// 2. DB
	return nil
}

// NewWorkflowDefinition creates a new workflow
func (c controller) NewWorkflowDefinition(ctx context.Context, workflowReq *models.NewWorkflowDefinitionRequest) (*models.WorkflowDefinition, error) {
	//TODO: validate states
	if len(workflowReq.States) == 0 {
		return &models.WorkflowDefinition{}, fmt.Errorf("Must define at least one state")
	}
	if workflowReq.Name == "" {
		return &models.WorkflowDefinition{}, fmt.Errorf("WorkflowDefinition `name` is required")
	}

	workflow, err := newWorkflowDefinitionFromRequest(*workflowReq)
	if err != nil {
		return &models.WorkflowDefinition{}, err
	}

	if err := c.store.SaveWorkflowDefinition(workflow); err != nil {
		return &models.WorkflowDefinition{}, err
	}

	return apiWorkflowDefinitionFromStore(workflow), nil
}

// UpdateWorkflowDefinition creates a new revision for an existing workflow
func (c controller) UpdateWorkflowDefinition(ctx context.Context, input *models.UpdateWorkflowDefinitionInput) (*models.WorkflowDefinition, error) {
	workflowReq := input.NewWorkflowDefinitionRequest
	if workflowReq == nil || workflowReq.Name != input.Name {
		return &models.WorkflowDefinition{}, fmt.Errorf("Name in path must match WorkflowDefinition object")
	}
	if len(workflowReq.States) == 0 {
		return &models.WorkflowDefinition{}, fmt.Errorf("Must define at least one state")
	}

	workflow, err := newWorkflowDefinitionFromRequest(*workflowReq)
	if err != nil {
		return &models.WorkflowDefinition{}, err
	}

	workflow, err = c.store.UpdateWorkflowDefinition(workflow)
	if err != nil {
		return &models.WorkflowDefinition{}, err
	}

	return apiWorkflowDefinitionFromStore(workflow), nil
}

// GetWorkflowDefinitions retrieves a list of the latest version of each workflow
func (c controller) GetWorkflowDefinitions(ctx context.Context) ([]models.WorkflowDefinition, error) {
	workflows, err := c.store.GetWorkflowDefinitions()

	if err != nil {
		return []models.WorkflowDefinition{}, err
	}
	apiWorkflowDefinitions := []models.WorkflowDefinition{}
	for _, workflow := range workflows {
		apiWorkflowDefinition := apiWorkflowDefinitionFromStore(workflow)
		apiWorkflowDefinitions = append(apiWorkflowDefinitions, *apiWorkflowDefinition)
	}
	return apiWorkflowDefinitions, nil
}

// GetWorkflowDefinitionVersionsByName fetches either:
//  1. A list of all versions of a workflow by name
//  2. The most recent version of a workflow by name
func (c controller) GetWorkflowDefinitionVersionsByName(ctx context.Context, input *models.GetWorkflowDefinitionVersionsByNameInput) ([]models.WorkflowDefinition, error) {
	if *input.Latest == true {
		workflow, err := c.store.LatestWorkflowDefinition(input.Name)
		if err != nil {
			return []models.WorkflowDefinition{}, err
		}
		return []models.WorkflowDefinition{*(apiWorkflowDefinitionFromStore(workflow))}, nil
	}

	apiWorkflowDefinitions := []models.WorkflowDefinition{}
	workflows, err := c.store.GetWorkflowDefinitionVersions(input.Name)
	if err != nil {
		return []models.WorkflowDefinition{}, err
	}
	for _, workflow := range workflows {
		apiWorkflowDefinition := apiWorkflowDefinitionFromStore(workflow)
		apiWorkflowDefinitions = append(apiWorkflowDefinitions, *apiWorkflowDefinition)
	}

	return apiWorkflowDefinitions, nil
}

// GetWorkflowDefinitionByNameAndVersion allows fetching an existing WorkflowDefinition by providing it's name and version
func (c controller) GetWorkflowDefinitionByNameAndVersion(ctx context.Context, input *models.GetWorkflowDefinitionByNameAndVersionInput) (*models.WorkflowDefinition, error) {
	workflow, err := c.store.GetWorkflowDefinition(input.Name, int(input.Version))
	if err != nil {
		return &models.WorkflowDefinition{}, err
	}
	return apiWorkflowDefinitionFromStore(workflow), nil
}

// PostStateResource creates a new state resource
func (c controller) PostStateResource(ctx context.Context, i *models.NewStateResource) (*models.StateResource, error) {
	stateResource := resources.NewBatchResource(i.Name, i.Namespace, i.URI)
	if err := c.store.SaveStateResource(stateResource); err != nil {
		return &models.StateResource{}, err
	}

	return apiStateResourceFromStore(stateResource), nil
}

// PutStateResource creates or updates a state resource
func (c controller) PutStateResource(ctx context.Context, i *models.PutStateResourceInput) (*models.StateResource, error) {
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
	if err := c.store.SaveStateResource(stateResource); err != nil {
		return &models.StateResource{}, err
	}

	return apiStateResourceFromStore(stateResource), nil
}

// GetStateResource fetches a StateResource given a name and namespace
func (c controller) GetStateResource(ctx context.Context, i *models.GetStateResourceInput) (*models.StateResource, error) {
	stateResource, err := c.store.GetStateResource(i.Name, i.Namespace)
	if err != nil {
		return &models.StateResource{}, err
	}

	return apiStateResourceFromStore(stateResource), nil
}

// DeleteStateResource removes a StateResource given a name and namespace
func (c controller) DeleteStateResource(ctx context.Context, i *models.DeleteStateResourceInput) error {
	return c.store.DeleteStateResource(i.Name, i.Namespace)
}

// StartWorkflow starts a new Workflow for the given WorkflowDefinition
func (c controller) StartWorkflow(ctx context.Context, input *models.WorkflowInput) (*models.Workflow, error) {
	var workflow resources.WorkflowDefinition
	var err error
	if input.WorkflowDefinition.Revision < 0 {
		workflow, err = c.store.LatestWorkflowDefinition(input.WorkflowDefinition.Name)
	} else {
		workflow, err = c.store.GetWorkflowDefinition(input.WorkflowDefinition.Name, int(input.WorkflowDefinition.Revision))
	}
	if err != nil {
		return &models.Workflow{}, err
	}

	var data []string
	if input.Data != nil {
		// convert from []interface{} to []string (i.e. flattened json string array)
		data = jsonToArgs(input.Data)
	}

	job, err := c.manager.CreateWorkflow(workflow, data, input.Namespace, input.Queue)
	if err != nil {
		return &models.Workflow{}, err
	}

	return apiWorkflowFromStore(*job), nil
}

// GetWorkflows returns a summary of all active jobs for the given workflow
func (c controller) GetWorkflows(ctx context.Context, input *models.GetWorkflowsInput) ([]models.Workflow, error) {
	jobs, err := c.store.GetWorkflows(input.WorkflowDefinitionName)
	if err != nil {
		return []models.Workflow{}, err
	}

	results := []models.Workflow{}
	for _, job := range jobs {
		c.manager.UpdateWorkflowStatus(&job)
		results = append(results, *apiWorkflowFromStore(job))
	}
	return results, nil
}

// GetWorkflow returns current details about a Workflow with the given jobId
func (c controller) GetWorkflowByID(ctx context.Context, jobID string) (*models.Workflow, error) {
	job, err := c.store.GetWorkflowByID(jobID)
	if err != nil {
		return &models.Workflow{}, err
	}

	err = c.manager.UpdateWorkflowStatus(&job)
	if err != nil {
		return &models.Workflow{}, err
	}

	return apiWorkflowFromStore(job), nil
}

// CancelWorkflow cancels all the tasks currently running or queued for the Workflow and
// marks the job as cancelled
func (c controller) CancelWorkflow(ctx context.Context, input *models.CancelWorkflowInput) error {
	job, err := c.store.GetWorkflowByID(input.WorkflowId)
	if err != nil {
		return err
	}

	return c.manager.CancelWorkflow(&job, input.Reason.Reason)
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

func newWorkflowDefinitionFromRequest(req models.NewWorkflowDefinitionRequest) (resources.WorkflowDefinition, error) {
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

func apiWorkflowDefinitionFromStore(wf resources.WorkflowDefinition) *models.WorkflowDefinition {
	states := []*models.State{}
	for _, s := range wf.OrderedStates() {
		states = append(states, &models.State{
			Resource: s.Resource(),
			Name:     s.Name(),
			Next:     s.Next(),
			End:      s.IsEnd(),
			Type:     s.Type(),
			Retry:    s.Retry(),
		})
	}

	return &models.WorkflowDefinition{
		Name:      wf.Name(),
		Revision:  int64(wf.Version()),
		StartAt:   wf.StartAt().Name(),
		CreatedAt: strfmt.DateTime(wf.CreatedAt()),
		States:    states,
	}
}

func apiWorkflowFromStore(job resources.Workflow) *models.Workflow {
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
			Attempts:     task.Attempts,
		})
	}

	return &models.Workflow{
		ID:                 job.ID,
		CreatedAt:          strfmt.DateTime(job.CreatedAt),
		LastUpdated:        strfmt.DateTime(job.LastUpdated),
		Tasks:              tasks,
		WorkflowDefinition: apiWorkflowDefinitionFromStore(job.WorkflowDefinition),
		Status:             string(job.Status),
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
