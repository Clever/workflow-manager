package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	elasticsearch "github.com/elastic/go-elasticsearch/v6"
	"github.com/elastic/go-elasticsearch/v6/esapi"
	"gopkg.in/Clever/kayvee-go.v6/logger"

	"github.com/Clever/workflow-manager/executor"
	"github.com/Clever/workflow-manager/gen-go/models"
	"github.com/Clever/workflow-manager/resources"
	"github.com/Clever/workflow-manager/store"
)

const defaultLimit = 10

// Handler implements the wag Controller
type Handler struct {
	store     store.Store
	manager   executor.WorkflowManager
	es        *elasticsearch.Client
	deployEnv string
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

	if err := h.store.SaveWorkflowDefinition(ctx, *workflowDef); err != nil {
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

	updatedWorkflow, err := h.store.UpdateWorkflowDefinition(ctx, *workflow)
	if err != nil {
		return &models.WorkflowDefinition{}, err
	}

	return &updatedWorkflow, nil
}

// GetWorkflowDefinitions retrieves a list of the latest version of each workflow
func (h Handler) GetWorkflowDefinitions(ctx context.Context) ([]models.WorkflowDefinition, error) {
	return h.store.GetWorkflowDefinitions(ctx)
}

// GetWorkflowDefinitionVersionsByName fetches either:
//  1. A list of all versions of a workflow by name
//  2. The most recent version of a workflow by name
func (h Handler) GetWorkflowDefinitionVersionsByName(ctx context.Context, input *models.GetWorkflowDefinitionVersionsByNameInput) ([]models.WorkflowDefinition, error) {
	if *input.Latest == true {
		workflow, err := h.store.LatestWorkflowDefinition(ctx, input.Name)
		if err != nil {
			return []models.WorkflowDefinition{}, err
		}
		return []models.WorkflowDefinition{workflow}, nil
	}
	return h.store.GetWorkflowDefinitionVersions(ctx, input.Name)
}

// GetWorkflowDefinitionByNameAndVersion allows fetching an existing WorkflowDefinition by providing it's name and version
func (h Handler) GetWorkflowDefinitionByNameAndVersion(ctx context.Context, input *models.GetWorkflowDefinitionByNameAndVersionInput) (*models.WorkflowDefinition, error) {
	wfd, err := h.store.GetWorkflowDefinition(ctx, input.Name, int(input.Version))
	if err != nil {
		return nil, err
	}
	return &wfd, nil
}

// PostStateResource creates a new state resource
func (h Handler) PostStateResource(ctx context.Context, i *models.NewStateResource) (*models.StateResource, error) {
	stateResource := resources.NewStateResource(i.Name, i.Namespace, i.URI)
	if err := h.store.SaveStateResource(ctx, *stateResource); err != nil {
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
	if err := h.store.SaveStateResource(ctx, *stateResource); err != nil {
		return &models.StateResource{}, err
	}

	return stateResource, nil
}

// GetStateResource fetches a StateResource given a name and namespace
func (h Handler) GetStateResource(ctx context.Context, i *models.GetStateResourceInput) (*models.StateResource, error) {
	stateResource, err := h.store.GetStateResource(ctx, i.Name, i.Namespace)
	if err != nil {
		return &models.StateResource{}, err
	}
	return &stateResource, nil
}

// DeleteStateResource removes a StateResource given a name and namespace
func (h Handler) DeleteStateResource(ctx context.Context, i *models.DeleteStateResourceInput) error {
	return h.store.DeleteStateResource(ctx, i.Name, i.Namespace)
}

// StartWorkflow starts a new Workflow for the given WorkflowDefinition
func (h Handler) StartWorkflow(ctx context.Context, req *models.StartWorkflowRequest) (*models.Workflow, error) {
	var workflowDefinition models.WorkflowDefinition
	var err error
	if req.WorkflowDefinition.Version < 0 {
		workflowDefinition, err = h.store.LatestWorkflowDefinition(ctx, req.WorkflowDefinition.Name)
	} else {
		workflowDefinition, err = h.store.GetWorkflowDefinition(ctx, req.WorkflowDefinition.Name, int(req.WorkflowDefinition.Version))
	}
	switch err.(type) {
	case nil: // Do nothing
	case models.NotFound:
		logger.FromContext(ctx).WarnD("start-unknown-workflow", logger.M{
			"name":    req.WorkflowDefinition.Name,
			"version": req.WorkflowDefinition.Version,
		})
		return &models.Workflow{}, err
	default:
		return &models.Workflow{}, err
	}

	if req.Queue == "" {
		req.Queue = "default"
	}

	// verify request's tags (map[string]interface{}) are actually map[string]string
	if err := validateTagsMap(req.Tags); err != nil {
		return &models.Workflow{}, err
	}

	// Workflows require an input that is marshallable to map[string]interface{}
	// To simplify submitting workflows that require no specific configuration in their input,
	// allow submitting empty string and convert it to an empty dict for convenience
	if req.Input == "" {
		req.Input = "{}"
	}

	return h.manager.CreateWorkflow(ctx, workflowDefinition, req.Input, req.Namespace, req.Queue, req.Tags)
}

func decodeESResponseToWorkflows(body io.Reader) ([]models.Workflow, error) {
	var r map[string]interface{}
	if err := json.NewDecoder(body).Decode(&r); err != nil {
		return nil, fmt.Errorf("Error parsing the response body: %s", err)
	}
	workflows := []models.Workflow{}
	for _, hit := range r["hits"].(map[string]interface{})["hits"].([]interface{}) {
		workflowMap := hit.(map[string]interface{})["_source"].(map[string]interface{})["Workflow"]
		workflowBs, err := json.Marshal(workflowMap)
		if err != nil {
			return nil, err
		}
		var workflow models.Workflow
		if err := json.Unmarshal(workflowBs, &workflow); err != nil {
			return nil, err
		}
		workflows = append(workflows, workflow)
	}
	return workflows, nil
}

// GetWorkflows returns a summary of all workflows matching the given query.
func (h Handler) GetWorkflows(
	ctx context.Context,
	input *models.GetWorkflowsInput,
) ([]models.Workflow, string, error) {
	indexName := "workflow-manager-prod-v3-workflows"
	if h.deployEnv != "production" {
		indexName = "clever-dev-workflow-manager-dev-v3-workflows"
	}
	req := []func(*esapi.SearchRequest){
		h.es.Search.WithContext(context.Background()),
		h.es.Search.WithIndex(indexName),
		h.es.Search.WithFrom(0),
		h.es.Search.WithSourceIncludes(
			"Workflow.id",
			"Workflow.createdAt",
			"Workflow.stoppedAt",
			"Workflow.lastUpdated",
			"Workflow.workflowDefinition.id",
			"Workflow.workflowDefinition.name",
			"Workflow.workflowDefinition.createdAt",
			"Workflow.workflowDefinition.manager",
			"Workflow.workflowDefinition.defaultTags",
			"Workflow.status",
			"Workflow.namespace",
			"Workflow.queue",
			"Workflow.input",
			"Workflow.resolvedByUser",
			"Workflow.retryFor",
			"Workflow.retries",
		),
	}
	req = append(req, h.es.Search.WithBody(strings.NewReader(h.getWorkflowsInputToESQuery(input))))
	res, err := h.es.Search(req...)
	if err != nil {
		return nil, "", err
	}
	defer res.Body.Close()
	if res.IsError() {
		var e map[string]interface{}
		if err := json.NewDecoder(res.Body).Decode(&e); err != nil {
			return nil, "", fmt.Errorf("error parsing the response body: %s", err)
		}
		return nil, "", fmt.Errorf("[%s] %s: %s",
			res.Status(),
			e["error"].(map[string]interface{})["type"],
			e["error"].(map[string]interface{})["reason"],
		)
	}

	workflows, err := decodeESResponseToWorkflows(res.Body)
	if err != nil {
		return nil, "", err
	}
	var nextPageToken string
	if len(workflows) > 0 {
		last := workflows[len(workflows)-1]
		nextPageToken = fmt.Sprintf("%d", epochMillis(time.Time(last.CreatedAt)))
	}
	return workflows, nextPageToken, nil
}

func (h Handler) getWorkflowsInputToESQuery(input *models.GetWorkflowsInput) string {
	var b strings.Builder
	b.WriteString("{")

	if input.Limit == nil {
		b.WriteString(fmt.Sprintf(`"size": %d,`, defaultLimit))
	} else if *input.Limit != 0 {
		b.WriteString(fmt.Sprintf(`"size": %d,`, *input.Limit))
	}

	if input.OldestFirst == nil || *input.OldestFirst == false {
		b.WriteString(`"sort": [{"Workflow.createdAt": "desc"}],`)
	} else {
		b.WriteString(`"sort": [{"Workflow.createdAt": "asc"}],`)
	}

	if input.PageToken != nil && *input.PageToken != "" {
		b.WriteString(fmt.Sprintf(`"search_after": [%s],`, *input.PageToken))
	}

	// status, resolved by user, workflow definition name must be filters / must nots
	var filters []string
	var mustNots []string
	if input.Status != nil && *input.Status != "" {
		filters = append(filters, fmt.Sprintf(`{"term":{"Workflow.status": "%s"}}`, *input.Status))
	}
	if input.ResolvedByUser != nil {
		if *input.ResolvedByUser {
			filters = append(filters, `{"term":{"Workflow.resolvedByUser": true}}`)
		} else {
			mustNots = append(mustNots, `{"term":{"Workflow.resolvedByUser": true}}`)
		}
	}
	if input.WorkflowDefinitionName != "" {
		filters = append(filters, fmt.Sprintf(`{"term":{"Workflow.workflowDefinition.name.keyword": "%s"}}`, input.WorkflowDefinitionName))
	}

	b.WriteString(fmt.Sprintf(`"query" : { "bool": {"filter": [%s], "must_not": [%s]}}`,
		strings.Join(filters, ","),
		strings.Join(mustNots, ","),
	))

	b.WriteString("}")
	return b.String()
}

// GetWorkflowByID returns current details about a Workflow with the given workflowId
func (h Handler) GetWorkflowByID(ctx context.Context, workflowID string) (*models.Workflow, error) {
	workflow, err := h.store.GetWorkflowByID(ctx, workflowID)
	if err != nil {
		return &models.Workflow{}, err
	}

	if err := h.manager.UpdateWorkflowSummary(ctx, &workflow); err != nil {
		return &models.Workflow{}, err
	}

	if err := h.manager.UpdateWorkflowHistory(ctx, &workflow); err != nil {
		return &models.Workflow{}, err
	}

	return &workflow, nil
}

// CancelWorkflow cancels all the jobs currently running or queued for the Workflow and
// marks the workflow as cancelled
func (h Handler) CancelWorkflow(ctx context.Context, input *models.CancelWorkflowInput) error {
	workflow, err := h.store.GetWorkflowByID(ctx, input.WorkflowID)
	if err != nil {
		return err
	}

	return h.manager.CancelWorkflow(ctx, &workflow, input.Reason.Reason)
}

// ResumeWorkflowByID starts a new Workflow based on an existing completed Workflow
// from the provided position. Uses existing inputs and outputs when required
func (h Handler) ResumeWorkflowByID(ctx context.Context, input *models.ResumeWorkflowByIDInput) (*models.Workflow, error) {
	workflow, err := h.store.GetWorkflowByID(ctx, input.WorkflowID)
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

	return h.manager.RetryWorkflow(ctx, workflow, input.Overrides.StartAt, effectiveInput)
}

// ResolveWorkflowByID sets a workflow's ResolvedByUser to true if it is currently false.
// If the workflow's ResolvedByUser field is already true, it identifies this situation as a conflict.
func (h Handler) ResolveWorkflowByID(ctx context.Context, workflowID string) error {
	workflow, err := h.store.GetWorkflowByID(ctx, workflowID)
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

	return h.store.UpdateWorkflow(ctx, workflow)
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

	// verify request's default_tags (map[string]interface{}) are actually map[string]string
	if err := validateTagsMap(req.DefaultTags); err != nil {
		return nil, err
	}

	return resources.NewWorkflowDefinition(req.Name, req.Manager, req.StateMachine, req.DefaultTags)
}

// validateTagsMap ensures that all tags values are strings
func validateTagsMap(apiTags map[string]interface{}) error {
	for _, val := range apiTags {
		if _, ok := val.(string); !ok {
			return fmt.Errorf("error converting tag value to string: %+v", val)
		}
	}
	return nil
}

func epochMillis(t time.Time) int {
	return int(t.UnixNano() / int64(time.Millisecond))
}
