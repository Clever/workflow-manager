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

// Handler implements the wag Controller
type Handler struct {
	store   store.Store
	manager executor.WorkflowManager
}

// HealthCheck returns 200 if workflow-manager can respond to requests
func (h Handler) HealthCheck(ctx context.Context) error {
	// TODO: check that dependency clients are initialized and working
	// 1. AWS Batch
	// 2. DB
	return nil
}

// NewWorkflowDefinition creates a new workflow
func (h Handler) NewWorkflowDefinition(ctx context.Context, workflowReq *models.NewWorkflowDefinitionRequest) (*models.WorkflowDefinition, error) {
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

	if err := h.store.SaveWorkflowDefinition(workflow); err != nil {
		return &models.WorkflowDefinition{}, err
	}

	return apiWorkflowDefinitionFromStore(workflow), nil
}

// UpdateWorkflowDefinition creates a new version for an existing workflow
func (h Handler) UpdateWorkflowDefinition(ctx context.Context, input *models.UpdateWorkflowDefinitionInput) (*models.WorkflowDefinition, error) {
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

	workflow, err = h.store.UpdateWorkflowDefinition(workflow)
	if err != nil {
		return &models.WorkflowDefinition{}, err
	}

	return apiWorkflowDefinitionFromStore(workflow), nil
}

// GetWorkflowDefinitions retrieves a list of the latest version of each workflow
func (h Handler) GetWorkflowDefinitions(ctx context.Context) ([]models.WorkflowDefinition, error) {
	workflows, err := h.store.GetWorkflowDefinitions()

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
func (h Handler) GetWorkflowDefinitionVersionsByName(ctx context.Context, input *models.GetWorkflowDefinitionVersionsByNameInput) ([]models.WorkflowDefinition, error) {
	if *input.Latest == true {
		workflow, err := h.store.LatestWorkflowDefinition(input.Name)
		if err != nil {
			return []models.WorkflowDefinition{}, err
		}
		return []models.WorkflowDefinition{*(apiWorkflowDefinitionFromStore(workflow))}, nil
	}

	apiWorkflowDefinitions := []models.WorkflowDefinition{}
	workflows, err := h.store.GetWorkflowDefinitionVersions(input.Name)
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
func (h Handler) GetWorkflowDefinitionByNameAndVersion(ctx context.Context, input *models.GetWorkflowDefinitionByNameAndVersionInput) (*models.WorkflowDefinition, error) {
	workflow, err := h.store.GetWorkflowDefinition(input.Name, int(input.Version))
	if err != nil {
		return &models.WorkflowDefinition{}, err
	}
	return apiWorkflowDefinitionFromStore(workflow), nil
}

// PostStateResource creates a new state resource
func (h Handler) PostStateResource(ctx context.Context, i *models.NewStateResource) (*models.StateResource, error) {
	stateResource := resources.NewBatchResource(i.Name, i.Namespace, i.URI)
	if err := h.store.SaveStateResource(stateResource); err != nil {
		return &models.StateResource{}, err
	}

	return apiStateResourceFromStore(stateResource), nil
}

// PutStateResource creates or updates a state resource
func (h Handler) PutStateResource(ctx context.Context, i *models.PutStateResourceInput) (*models.StateResource, error) {
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
	if err := h.store.SaveStateResource(stateResource); err != nil {
		return &models.StateResource{}, err
	}

	return apiStateResourceFromStore(stateResource), nil
}

// GetStateResource fetches a StateResource given a name and namespace
func (h Handler) GetStateResource(ctx context.Context, i *models.GetStateResourceInput) (*models.StateResource, error) {
	stateResource, err := h.store.GetStateResource(i.Name, i.Namespace)
	if err != nil {
		return &models.StateResource{}, err
	}

	return apiStateResourceFromStore(stateResource), nil
}

// DeleteStateResource removes a StateResource given a name and namespace
func (h Handler) DeleteStateResource(ctx context.Context, i *models.DeleteStateResourceInput) error {
	return h.store.DeleteStateResource(i.Name, i.Namespace)
}

// StartWorkflow starts a new Workflow for the given WorkflowDefinition
func (h Handler) StartWorkflow(ctx context.Context, params *models.StartWorkflowParams) (*models.Workflow, error) {
	var workflowDefinition resources.WorkflowDefinition
	var err error
	if params.WorkflowDefinition.Version < 0 {
		workflowDefinition, err = h.store.LatestWorkflowDefinition(params.WorkflowDefinition.Name)
	} else {
		workflowDefinition, err = h.store.GetWorkflowDefinition(params.WorkflowDefinition.Name, int(params.WorkflowDefinition.Version))
	}
	if err != nil {
		return &models.Workflow{}, err
	}

	workflow, err := h.manager.CreateWorkflow(workflowDefinition, params.Input, params.Namespace, params.Queue)
	if err != nil {
		return &models.Workflow{}, err
	}

	return apiWorkflowFromStore(*workflow), nil
}

// GetWorkflows returns a summary of all active workflows for the given workflow
func (h Handler) GetWorkflows(ctx context.Context, input *models.GetWorkflowsInput) ([]models.Workflow, error) {
	workflows, err := h.store.GetWorkflows(input.WorkflowDefinitionName)
	if err != nil {
		return []models.Workflow{}, err
	}

	results := []models.Workflow{}
	for _, workflow := range workflows {
		h.manager.UpdateWorkflowStatus(&workflow)
		results = append(results, *apiWorkflowFromStore(workflow))
	}
	return results, nil
}

// GetWorkflowByID returns current details about a Workflow with the given workflowId
func (h Handler) GetWorkflowByID(ctx context.Context, workflowID string) (*models.Workflow, error) {
	workflow, err := h.store.GetWorkflowByID(workflowID)
	if err != nil {
		return &models.Workflow{}, err
	}

	err = h.manager.UpdateWorkflowStatus(&workflow)
	if err != nil {
		return &models.Workflow{}, err
	}

	return apiWorkflowFromStore(workflow), nil
}

// CancelWorkflow cancels all the jobs currently running or queued for the Workflow and
// marks the workflow as cancelled
func (h Handler) CancelWorkflow(ctx context.Context, input *models.CancelWorkflowInput) error {
	workflow, err := h.store.GetWorkflowByID(input.WorkflowId)
	if err != nil {
		return err
	}

	return h.manager.CancelWorkflow(&workflow, input.Reason.Reason)
}

// TODO: the functions below should probably just be functions on the respective resources.<Struct>

func newWorkflowDefinitionFromRequest(req models.NewWorkflowDefinitionRequest) (resources.WorkflowDefinition, error) {
	if req.StartAt == "" {
		return resources.WorkflowDefinition{}, fmt.Errorf("startAt is a required field")
	}

	states := map[string]resources.State{}
	for _, s := range req.States {
		// TODO: Task=>Job? (INFRA-2483)
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
		Version:   int64(wf.Version()),
		StartAt:   wf.StartAt().Name(),
		CreatedAt: strfmt.DateTime(wf.CreatedAt()),
		States:    states,
	}
}

func apiWorkflowFromStore(workflow resources.Workflow) *models.Workflow {
	jobs := []*models.Job{}
	for _, job := range workflow.Jobs {
		jobs = append(jobs, &models.Job{
			ID:           job.ID,
			Attempts:     job.Attempts,
			Container:    job.ContainerId,
			CreatedAt:    strfmt.DateTime(job.CreatedAt),
			Input:        job.Input,
			Output:       job.Output,
			StartedAt:    strfmt.DateTime(job.StartedAt),
			State:        job.State,
			Status:       string(job.Status),
			StatusReason: job.StatusReason,
			StoppedAt:    strfmt.DateTime(job.StoppedAt),
		})
	}

	return &models.Workflow{
		ID:                 workflow.ID,
		CreatedAt:          strfmt.DateTime(workflow.CreatedAt),
		LastUpdated:        strfmt.DateTime(workflow.LastUpdated),
		Jobs:               jobs,
		WorkflowDefinition: apiWorkflowDefinitionFromStore(workflow.WorkflowDefinition),
		Status:             string(workflow.Status),
		Namespace:          workflow.Namespace,
		Queue:              workflow.Queue,
		Input:              workflow.Input,
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
