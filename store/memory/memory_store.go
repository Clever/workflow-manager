package memory

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/go-openapi/strfmt"
	uuid "github.com/satori/go.uuid"

	"github.com/Clever/workflow-manager/gen-go/models"
	"github.com/Clever/workflow-manager/resources"
	"github.com/Clever/workflow-manager/store"
)

type MemoryStore struct {
	workflowDefinitions map[string][]models.WorkflowDefinition
	workflows           map[string]models.Workflow
	workflowsLocked     map[string]struct{}
	stateResources      map[string]models.StateResource
}

type ByCreatedAt []models.Workflow

func (a ByCreatedAt) Len() int      { return len(a) }
func (a ByCreatedAt) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a ByCreatedAt) Less(i, j int) bool {
	return time.Time(a[i].CreatedAt).Before(time.Time(a[j].CreatedAt))
}

func New() MemoryStore {
	return MemoryStore{
		workflowDefinitions: map[string][]models.WorkflowDefinition{},
		workflows:           map[string]models.Workflow{},
		workflowsLocked:     map[string]struct{}{},
		stateResources:      map[string]models.StateResource{},
	}
}

func (s MemoryStore) SaveWorkflowDefinition(ctx context.Context, def models.WorkflowDefinition) error {

	if _, ok := s.workflowDefinitions[def.Name]; ok {
		return store.NewConflict(def.Name)
	}
	def.CreatedAt = strfmt.DateTime(time.Now())
	s.workflowDefinitions[def.Name] = []models.WorkflowDefinition{def}
	return nil
}

func (s MemoryStore) UpdateWorkflowDefinition(ctx context.Context, def models.WorkflowDefinition) (models.WorkflowDefinition, error) {
	last, err := s.LatestWorkflowDefinition(ctx, def.Name)
	if err != nil {
		return def, err
	}

	newVersion := resources.NewWorkflowDefinitionVersion(&def, int(last.Version+1))
	newVersion.CreatedAt = strfmt.DateTime(time.Now())
	s.workflowDefinitions[def.Name] = append(s.workflowDefinitions[def.Name], *newVersion)

	return *newVersion, nil
}

// GetWorkflowDefinitions returns the latest version of all stored workflow definitions
func (s MemoryStore) GetWorkflowDefinitions(ctx context.Context) ([]models.WorkflowDefinition, error) {
	workflowDefinitions := []models.WorkflowDefinition{}
	// for each workflow definition
	for _, versionedWorkflowDefinitions := range s.workflowDefinitions {
		// for each version of a workflow definition
		for _, workflow := range versionedWorkflowDefinitions {
			workflowDefinitions = append(workflowDefinitions, workflow)
		}
	}

	return workflowDefinitions, nil
}

// GetWorkflowDefinitionVersions gets all versions of a workflow definition
func (s MemoryStore) GetWorkflowDefinitionVersions(ctx context.Context, name string) ([]models.WorkflowDefinition, error) {
	workflowDefinitions, ok := s.workflowDefinitions[name]
	if !ok {
		return []models.WorkflowDefinition{}, store.NewNotFound(name)
	}

	return workflowDefinitions, nil
}

func (s MemoryStore) GetWorkflowDefinition(ctx context.Context, name string, version int) (models.WorkflowDefinition, error) {
	if _, ok := s.workflowDefinitions[name]; !ok {
		return models.WorkflowDefinition{}, store.NewNotFound(fmt.Sprintf("%s@%d", name, version))
	}

	if len(s.workflowDefinitions[name]) < version {
		return models.WorkflowDefinition{}, store.NewNotFound(fmt.Sprintf("%s@%d", name, version))
	}

	return s.workflowDefinitions[name][version], nil
}

func (s MemoryStore) LatestWorkflowDefinition(ctx context.Context, name string) (models.WorkflowDefinition, error) {
	if _, ok := s.workflowDefinitions[name]; !ok {
		return models.WorkflowDefinition{}, store.NewNotFound(name)
	}

	return s.GetWorkflowDefinition(ctx, name, len(s.workflowDefinitions[name])-1)
}

func (s MemoryStore) SaveStateResource(ctx context.Context, res models.StateResource) error {
	resourceName := res.Name
	if res.Namespace != "" {
		resourceName = fmt.Sprintf("%s--%s", res.Namespace, res.Name)
	}

	s.stateResources[resourceName] = res
	return nil
}

func (s MemoryStore) GetStateResource(ctx context.Context, name, namespace string) (models.StateResource, error) {
	resourceName := name
	if namespace != "" {
		resourceName = fmt.Sprintf("%s--%s", namespace, name)
	}

	if _, ok := s.stateResources[resourceName]; !ok {
		return models.StateResource{}, store.NewNotFound(resourceName)
	}

	return s.stateResources[resourceName], nil
}

func (s MemoryStore) DeleteStateResource(ctx context.Context, name, namespace string) error {
	resourceName := name
	if namespace != "" {
		resourceName = fmt.Sprintf("%s--%s", namespace, name)
	}

	if _, ok := s.stateResources[resourceName]; !ok {
		return store.NewNotFound(resourceName)
	}
	delete(s.stateResources, resourceName)

	return nil
}

func (s MemoryStore) SaveWorkflow(ctx context.Context, workflow models.Workflow) error {
	if _, ok := s.workflows[workflow.ID]; ok {
		return store.NewConflict(workflow.ID)
	}
	workflow.CreatedAt = strfmt.DateTime(time.Now())
	workflow.LastUpdated = workflow.CreatedAt
	s.workflows[workflow.ID] = workflow
	return nil
}

func (s MemoryStore) UpdateWorkflow(ctx context.Context, workflow models.Workflow) error {
	if _, ok := s.workflows[workflow.ID]; !ok {
		return store.NewNotFound(workflow.ID)
	}
	workflow.LastUpdated = strfmt.DateTime(time.Now())
	s.workflows[workflow.ID] = workflow
	return nil
}

func (s MemoryStore) DeleteWorkflowByID(ctx context.Context, workflowID string) error {
	if _, ok := s.workflows[workflowID]; !ok {
		return store.NewNotFound(workflowID)
	}
	delete(s.workflows, workflowID)
	return nil
}

func (s MemoryStore) GetWorkflows(ctx context.Context,
	query *models.WorkflowQuery,
) ([]models.Workflow, string, error) {
	workflows := []models.Workflow{}

	for _, workflow := range s.workflows {
		if s.matchesQuery(workflow, query) {
			if aws.BoolValue(query.SummaryOnly) {
				// remove everything but WorkflowSummary
				workflow = models.Workflow{
					WorkflowSummary: workflow.WorkflowSummary,
				}
				// we need to minimize the WorkflowDefinition
				workflow.WorkflowDefinition = &models.WorkflowDefinition{
					Name:    workflow.WorkflowDefinition.Name,
					Version: workflow.WorkflowDefinition.Version,
				}
			}

			workflows = append(workflows, workflow)
		}
	}

	if query.OldestFirst {
		sort.Sort(ByCreatedAt(workflows))
	} else {
		sort.Sort(sort.Reverse(ByCreatedAt(workflows)))
	}

	rangeStart := 0
	if query.PageToken != "" {
		lastWorkflowID, err := uuid.FromString(query.PageToken)
		if err != nil {
			return []models.Workflow{}, "", store.NewInvalidPageTokenError(err)
		}

		for i, workflow := range workflows {
			if workflow.ID == lastWorkflowID.String() {
				rangeStart = i + 1
			}
		}
	}

	rangeEnd := rangeStart + int(query.Limit)
	if rangeEnd > len(workflows) {
		rangeEnd = len(workflows)
	}
	nextPageToken := ""
	if rangeEnd < len(workflows) {
		nextPageToken = workflows[rangeEnd-1].ID
	}

	return workflows[rangeStart:rangeEnd], nextPageToken, nil
}

func (s MemoryStore) matchesQuery(workflow models.Workflow, query *models.WorkflowQuery) bool {
	if workflow.WorkflowDefinition.Name != aws.StringValue(query.WorkflowDefinitionName) {
		return false
	}

	if query.Status != "" && workflow.Status != query.Status {
		return false
	}

	if query.ResolvedByUserWrapper != nil && query.ResolvedByUserWrapper.IsSet && workflow.ResolvedByUser != query.ResolvedByUserWrapper.Value {
		return false
	}

	return true
}

func (s MemoryStore) GetWorkflowByID(ctx context.Context, id string) (models.Workflow, error) {
	if _, ok := s.workflows[id]; !ok {
		return models.Workflow{}, store.NewNotFound(id)
	}

	return s.workflows[id], nil
}

func (s *MemoryStore) UpdateWorkflowAttributes(ctx context.Context, id string, update store.UpdateWorkflowAttributesInput) error {
	wf, ok := s.workflows[id]
	if !ok {
		return store.NewNotFound(id)
	}
	if update.LastUpdated != nil {
		wf.LastUpdated = *update.LastUpdated
	}
	if update.Status != nil {
		wf.Status = *update.Status
	}
	if update.StatusReason != nil {
		wf.StatusReason = *update.StatusReason
	}
	if update.StoppedAt != nil {
		wf.StoppedAt = strfmt.DateTime(*update.StoppedAt)
	}
	if update.ResolvedByUser != nil {
		wf.ResolvedByUser = *update.ResolvedByUser
	}
	if update.Output != nil {
		wf.Output = *update.Output
	}
	if update.LastJob != nil {
		wf.LastJob = update.LastJob
	}
	s.workflows[id] = wf
	return nil
}

type byLastUpdatedTime []models.Workflow

func (b byLastUpdatedTime) Len() int      { return len(b) }
func (b byLastUpdatedTime) Swap(i, j int) { b[i], b[j] = b[j], b[i] }
func (b byLastUpdatedTime) Less(i, j int) bool {
	return time.Time(b[i].LastUpdated).Before(time.Time(b[j].LastUpdated))
}
