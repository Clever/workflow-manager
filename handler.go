package main

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go/aws"

	"github.com/Clever/workflow-manager/executor"
	"github.com/Clever/workflow-manager/gen-go/models"
	"github.com/Clever/workflow-manager/resources"
	"github.com/Clever/workflow-manager/store"
)

// Handler implements the wag Controller
type Handler struct {
	store   store.Store
	manager executor.WorkflowManager
}

// HealthCheck returns 200 if workflow-manager can respond to requests
func (h Handler) HealthCheck(ctx context.Context) error {
	return nil
}

// NewWorkflowDefinition creates a new workflow definition
func (h Handler) NewWorkflowDefinition(ctx context.Context, workflowDefReq *models.NewWorkflowDefinitionRequest) (*models.WorkflowDefinition, error) {
	//TODO: validate states
	if len(workflowDefReq.StateMachine.States) == 0 {
		return nil, fmt.Errorf("Must define at least one state")
	}
	if workflowDefReq.Name == "" {
		return nil, fmt.Errorf("WorkflowDefinition `name` is required")
	}

	workflowDef, err := newWorkflowDefinitionFromRequest(*workflowDefReq)
	if err != nil {
		return nil, err
	}

	if err := h.store.SaveWorkflowDefinition(*workflowDef); err != nil {
		return nil, err
	}

	return workflowDef, nil
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
	stateResource := resources.NewStateResource(i.Name, i.Namespace, i.URI)
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

	stateResource := resources.NewStateResource(i.NewStateResource.Name, i.NewStateResource.Namespace, i.NewStateResource.URI)
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

	if req.Queue == "" {
		req.Queue = "default"
	}

	// verify request's tags (map[string]interface{}) are actually map[string]string
	if err := validateTagsMap(req.Tags); err != nil {
		return &models.Workflow{}, err
	}

	return h.manager.CreateWorkflow(workflowDefinition, req.Input, req.Namespace, req.Queue, req.Tags)
}

// GetWorkflows returns a summary of all workflows matching the given query.
func (h Handler) GetWorkflows(
	ctx context.Context,
	input *models.GetWorkflowsInput,
) ([]models.Workflow, string, error) {

	workflowQuery, err := paramsToWorkflowsQuery(input)
	if err != nil {
		return []models.Workflow{}, "", err
	}

	workflows, nextPageToken, err := h.store.GetWorkflows(workflowQuery)
	if err != nil {
		if _, ok := err.(store.InvalidPageTokenError); ok {
			return workflows, "", models.BadRequest{
				Message: err.Error(),
			}
		}

		return []models.Workflow{}, "", err
	}

	return workflows, nextPageToken, nil
}

func paramsToWorkflowsQuery(input *models.GetWorkflowsInput) (*models.WorkflowQuery, error) {
	query := &models.WorkflowQuery{
		WorkflowDefinitionName: aws.String(input.WorkflowDefinitionName),
		Limit:       aws.Int64Value(input.Limit),
		OldestFirst: aws.BoolValue(input.OldestFirst),
		PageToken:   aws.StringValue(input.PageToken),
		Status:      models.WorkflowStatus(aws.StringValue(input.Status)),
		SummaryOnly: input.SummaryOnly,
	}

	if err := query.Validate(nil); err != nil {
		return nil, err
	}
	return query, nil
}

// GetWorkflowByID returns current details about a Workflow with the given workflowId
func (h Handler) GetWorkflowByID(ctx context.Context, workflowID string) (*models.Workflow, error) {
	workflow, err := h.store.GetWorkflowByID(workflowID)
	if err != nil {
		return &models.Workflow{}, err
	}

	if err := h.manager.UpdateWorkflowSummary(&workflow); err != nil {
		return &models.Workflow{}, err
	}

	if err := h.manager.UpdateWorkflowHistory(&workflow); err != nil {
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

// ResumeWorkflowByID starts a new Workflow based on an existing completed Workflow
// from the provided position. Uses existing inputs and outputs when required
func (h Handler) ResumeWorkflowByID(ctx context.Context, input *models.ResumeWorkflowByIDInput) (*models.Workflow, error) {
	workflow, err := h.store.GetWorkflowByID(input.WorkflowID)
	if err != nil {
		return &models.Workflow{}, err
	}

	// don't allow resume if workflow is still active
	if !resources.WorkflowIsDone(&workflow) {
		return &models.Workflow{}, fmt.Errorf("Workflow %s active: %s", workflow.ID, workflow.Status)
	}
	if _, ok := workflow.WorkflowDefinition.StateMachine.States[input.Overrides.StartAt]; !ok {
		return &models.Workflow{}, fmt.Errorf("Invalid StartAt state %s", input.Overrides.StartAt)
	}

	// find the input to the StartAt state
	effectiveInput := ""
	for _, job := range workflow.Jobs {
		if job.State == input.Overrides.StartAt {
			// if job was never started then we should probably not trust the input
			if job.Status == models.JobStatusAbortedDepsFailed ||
				job.Status == models.JobStatusQueued ||
				job.Status == models.JobStatusWaitingForDeps ||
				job.Status == models.JobStatusCreated {

				return &models.Workflow{},
					fmt.Errorf("Job %s for StartAt %s was not started for Workflow: %s. Could not infer input",
						job.ID, job.State, workflow.ID)
			}

			effectiveInput = job.Input
			break
		}
	}

	return h.manager.RetryWorkflow(workflow, input.Overrides.StartAt, effectiveInput)
}

// ResolveWorkflowByID sets a workflow's ResolvedByUser to true if it is currently false.
// If the workflow's ResolvedByUser field is already true, it identifies this situation as a conflict.
func (h Handler) ResolveWorkflowByID(ctx context.Context, workflowID string) error {
	workflow, err := h.store.GetWorkflowByID(workflowID)
	if err != nil {
		return err
	}

	// if workflow is already resolved by user, error
	if workflow.ResolvedByUser {
		return models.Conflict{
			Message: fmt.Sprintf("workflow %s already resolved", workflow.ID),
		}
	}
	// set the ResolvedByUser value to true
	workflow.ResolvedByUser = true

	return h.store.UpdateWorkflow(workflow)
}

func newWorkflowDefinitionFromRequest(req models.NewWorkflowDefinitionRequest) (*models.WorkflowDefinition, error) {
	if req.StateMachine.StartAt == "" {
		return nil, fmt.Errorf("StartAt is a required field")
	}

	// ensure all states are defined and have a transition path
	numStates := len(req.StateMachine.States)
	if err := resources.RemoveInactiveStates(req.StateMachine); err != nil {
		return nil, err
	}
	if len(req.StateMachine.States) != numStates {
		return nil, fmt.Errorf("Invalid WorkflowDefinition: %d states have no transition path",
			numStates-len(req.StateMachine.States))
	}

	return resources.NewWorkflowDefinition(req.Name, req.Manager, req.StateMachine)
}

func validateTagsMap(apiTags map[string]interface{}) error {
	for _, val := range apiTags {
		if _, ok := val.(string); !ok {
			return fmt.Errorf("error converting tag value to string: %+v", val)
		}
	}
	return nil
}
