package main

import (
	"context"
	"fmt"

	"github.com/go-openapi/swag"

	"github.com/Clever/workflow-manager/executor"
	"github.com/Clever/workflow-manager/gen-go/models"
	"github.com/Clever/workflow-manager/resources"
	"github.com/Clever/workflow-manager/store"
)

const (
	// WorkflowsPageSizeMax defines the default page size for workflow queries.
	// TODO: This can be bumped up a bit once ark is updated to use the `limit` query param.
	WorkflowsPageSizeDefault int = 10

	// WorkflowsPageSizeMax defines the maximum allowed page size limit for workflow queries.
	WorkflowsPageSizeMax int = 10000
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
	if len(workflowReq.StateMachine.States) == 0 {
		return &models.WorkflowDefinition{}, fmt.Errorf("Must define at least one state")
	}
	if workflowReq.Name == "" {
		return &models.WorkflowDefinition{}, fmt.Errorf("WorkflowDefinition `name` is required")
	}

	workflow, err := newWorkflowDefinitionFromRequest(*workflowReq)
	if err != nil {
		return &models.WorkflowDefinition{}, err
	}

	if err := h.store.SaveWorkflowDefinition(*workflow); err != nil {
		return &models.WorkflowDefinition{}, err
	}

	return workflow, nil
}

// UpdateWorkflowDefinition creates a new version for an existing workflow
func (h Handler) UpdateWorkflowDefinition(ctx context.Context, input *models.UpdateWorkflowDefinitionInput) (*models.WorkflowDefinition, error) {
	workflowReq := input.NewWorkflowDefinitionRequest
	if workflowReq == nil || workflowReq.Name != input.Name {
		return &models.WorkflowDefinition{}, fmt.Errorf("Name in path must match WorkflowDefinition object")
	}
	if len(workflowReq.StateMachine.States) == 0 {
		return &models.WorkflowDefinition{}, fmt.Errorf("Must define at least one state")
	}

	workflow, err := newWorkflowDefinitionFromRequest(*workflowReq)
	if err != nil {
		return &models.WorkflowDefinition{}, err
	}

	updatedWorkflow, err := h.store.UpdateWorkflowDefinition(*workflow)
	if err != nil {
		return &models.WorkflowDefinition{}, err
	}

	return &updatedWorkflow, nil
}

// GetWorkflowDefinitions retrieves a list of the latest version of each workflow
func (h Handler) GetWorkflowDefinitions(ctx context.Context) ([]models.WorkflowDefinition, error) {
	return h.store.GetWorkflowDefinitions()
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
		return []models.WorkflowDefinition{workflow}, nil
	}
	return h.store.GetWorkflowDefinitionVersions(input.Name)
}

// GetWorkflowDefinitionByNameAndVersion allows fetching an existing WorkflowDefinition by providing it's name and version
func (h Handler) GetWorkflowDefinitionByNameAndVersion(ctx context.Context, input *models.GetWorkflowDefinitionByNameAndVersionInput) (*models.WorkflowDefinition, error) {
	wfd, err := h.store.GetWorkflowDefinition(input.Name, int(input.Version))
	if err != nil {
		return nil, err
	}
	return &wfd, nil
}

// PostStateResource creates a new state resource
func (h Handler) PostStateResource(ctx context.Context, i *models.NewStateResource) (*models.StateResource, error) {
	stateResource := resources.NewBatchResource(i.Name, i.Namespace, i.URI)
	if err := h.store.SaveStateResource(*stateResource); err != nil {
		return &models.StateResource{}, err
	}
	return stateResource, nil
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
	if err := h.store.SaveStateResource(*stateResource); err != nil {
		return &models.StateResource{}, err
	}

	return stateResource, nil
}

// GetStateResource fetches a StateResource given a name and namespace
func (h Handler) GetStateResource(ctx context.Context, i *models.GetStateResourceInput) (*models.StateResource, error) {
	stateResource, err := h.store.GetStateResource(i.Name, i.Namespace)
	if err != nil {
		return &models.StateResource{}, err
	}
	return &stateResource, nil
}

// DeleteStateResource removes a StateResource given a name and namespace
func (h Handler) DeleteStateResource(ctx context.Context, i *models.DeleteStateResourceInput) error {
	return h.store.DeleteStateResource(i.Name, i.Namespace)
}

// StartWorkflow starts a new Workflow for the given WorkflowDefinition
func (h Handler) StartWorkflow(ctx context.Context, req *models.StartWorkflowRequest) (*models.Workflow, error) {
	var workflowDefinition models.WorkflowDefinition
	var err error
	if req.WorkflowDefinition.Version < 0 {
		workflowDefinition, err = h.store.LatestWorkflowDefinition(req.WorkflowDefinition.Name)
	} else {
		workflowDefinition, err = h.store.GetWorkflowDefinition(req.WorkflowDefinition.Name, int(req.WorkflowDefinition.Version))
	}
	if err != nil {
		return &models.Workflow{}, err
	}

	if req.Queue == nil {
		return &models.Workflow{}, fmt.Errorf("workflow queue cannot be nil")
	}
	// verify request's tags (map[string]interface{}) are actually map[string]string
	if err := validateTagsMap(req.Tags); err != nil {
		return &models.Workflow{}, err
	}

	workflow, err := h.manager.CreateWorkflow(workflowDefinition, req.Input, req.Namespace, *req.Queue, req.Tags)
	if err != nil {
		return &models.Workflow{}, err
	}

	return workflow, nil
}

// GetWorkflows returns a summary of all workflows matching the given query.
func (h Handler) GetWorkflows(
	ctx context.Context,
	input *models.GetWorkflowsInput,
) ([]models.Workflow, string, error) {
	limit := WorkflowsPageSizeDefault
	if input.Limit != nil && *input.Limit > 0 {
		limit = int(*input.Limit)
	}
	if limit > WorkflowsPageSizeMax {
		limit = WorkflowsPageSizeMax
	}

	workflows, nextPageToken, err := h.store.GetWorkflows(&store.WorkflowQuery{
		DefinitionName: input.WorkflowDefinitionName,
		Limit:          limit,
		OldestFirst:    swag.BoolValue(input.OldestFirst),
		PageToken:      swag.StringValue(input.PageToken),
		Status:         swag.StringValue(input.Status),
	})
	if err != nil {
		if _, ok := err.(store.InvalidPageTokenError); ok {
			return []models.Workflow{}, "", models.BadRequest{
				Message: err.Error(),
			}
		}

		return []models.Workflow{}, "", err
	}

	results := []models.Workflow{}
	for _, workflow := range workflows {
		h.manager.UpdateWorkflowStatus(&workflow)
		results = append(results, workflow)
	}
	return results, nextPageToken, nil
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

	return &workflow, nil
}

// CancelWorkflow cancels all the jobs currently running or queued for the Workflow and
// marks the workflow as cancelled
func (h Handler) CancelWorkflow(ctx context.Context, input *models.CancelWorkflowInput) error {
	workflow, err := h.store.GetWorkflowByID(input.WorkflowID)
	if err != nil {
		return err
	}

	return h.manager.CancelWorkflow(&workflow, input.Reason.Reason)
}

// TODO: the functions below should probably just be functions on the respective resources.<Struct>

func newWorkflowDefinitionFromRequest(req models.NewWorkflowDefinitionRequest) (*models.WorkflowDefinition, error) {
	if req.StateMachine.StartAt == "" {
		return nil, fmt.Errorf("StartAt is a required field")
	}

	for _, s := range req.StateMachine.States {
		if s.Type != "" && s.Type != models.SLStateTypeTask {
			return nil, fmt.Errorf("Only States of `type=Task` are supported")
		}
	}

	if _, ok := req.StateMachine.States[req.StateMachine.StartAt]; !ok {
		return nil, fmt.Errorf("StartAt state %s not defined", req.StateMachine.StartAt)
	}
	return resources.NewWorkflowDefinition(req.Name, req.StateMachine)
}

func validateTagsMap(apiTags map[string]interface{}) error {
	for _, val := range apiTags {
		if _, ok := val.(string); !ok {
			return fmt.Errorf("error converting tag value to string: %+v", val)
		}
	}
	return nil
}
