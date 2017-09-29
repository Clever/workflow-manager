package memory

import (
	"fmt"
	"sort"
	"time"

	"github.com/go-openapi/strfmt"
	"github.com/satori/go.uuid"

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

func (s MemoryStore) SaveWorkflowDefinition(def models.WorkflowDefinition) error {

	if _, ok := s.workflowDefinitions[def.Name]; ok {
		return store.NewConflict(def.Name)
	}
	def.CreatedAt = strfmt.DateTime(time.Now())
	s.workflowDefinitions[def.Name] = []models.WorkflowDefinition{def}
	return nil
}

func (s MemoryStore) UpdateWorkflowDefinition(def models.WorkflowDefinition) (models.WorkflowDefinition, error) {
	last, err := s.LatestWorkflowDefinition(def.Name)
	if err != nil {
		return def, err
	}

	newVersion := resources.NewWorkflowDefinitionVersion(&def, int(last.Version+1))
	newVersion.CreatedAt = strfmt.DateTime(time.Now())
	s.workflowDefinitions[def.Name] = append(s.workflowDefinitions[def.Name], *newVersion)

	return *newVersion, nil
}

// GetWorkflowDefinitions returns the latest version of all stored workflow definitions
func (s MemoryStore) GetWorkflowDefinitions() ([]models.WorkflowDefinition, error) {
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
func (s MemoryStore) GetWorkflowDefinitionVersions(name string) ([]models.WorkflowDefinition, error) {
	workflowDefinitions, ok := s.workflowDefinitions[name]
	if !ok {
		return []models.WorkflowDefinition{}, store.NewNotFound(name)
	}

	return workflowDefinitions, nil
}

func (s MemoryStore) GetWorkflowDefinition(name string, version int) (models.WorkflowDefinition, error) {
	if _, ok := s.workflowDefinitions[name]; !ok {
		return models.WorkflowDefinition{}, store.NewNotFound(name)
	}

	if len(s.workflowDefinitions[name]) < version {
		return models.WorkflowDefinition{}, store.NewNotFound(fmt.Sprintf("%s.%d", name, version))
	}

	return s.workflowDefinitions[name][version], nil
}

func (s MemoryStore) LatestWorkflowDefinition(name string) (models.WorkflowDefinition, error) {
	if _, ok := s.workflowDefinitions[name]; !ok {
		return models.WorkflowDefinition{}, store.NewNotFound(name)
	}

	return s.GetWorkflowDefinition(name, len(s.workflowDefinitions[name])-1)
}

func (s MemoryStore) SaveStateResource(res models.StateResource) error {
	resourceName := res.Name
	if res.Namespace != "" {
		resourceName = fmt.Sprintf("%s--%s", res.Namespace, res.Name)
	}

	s.stateResources[resourceName] = res
	return nil
}

func (s MemoryStore) GetStateResource(name, namespace string) (models.StateResource, error) {
	resourceName := name
	if namespace != "" {
		resourceName = fmt.Sprintf("%s--%s", namespace, name)
	}

	if _, ok := s.stateResources[resourceName]; !ok {
		return models.StateResource{}, store.NewNotFound(resourceName)
	}

	return s.stateResources[resourceName], nil
}

func (s MemoryStore) DeleteStateResource(name, namespace string) error {
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

func (s MemoryStore) SaveWorkflow(workflow models.Workflow) error {
	if _, ok := s.workflows[workflow.ID]; ok {
		return store.NewConflict(workflow.ID)
	}
	workflow.CreatedAt = strfmt.DateTime(time.Now())
	workflow.LastUpdated = workflow.CreatedAt
	s.workflows[workflow.ID] = workflow
	return nil
}

func (s MemoryStore) UpdateWorkflow(workflow models.Workflow) error {
	if _, ok := s.workflows[workflow.ID]; !ok {
		return store.NewNotFound(workflow.ID)
	}
	workflow.LastUpdated = strfmt.DateTime(time.Now())
	s.workflows[workflow.ID] = workflow
	return nil
}

func (s MemoryStore) GetWorkflows(
	query *store.WorkflowQuery,
) ([]models.Workflow, string, error) {
	workflows := []models.Workflow{}
	for _, workflow := range s.workflows {
		if s.matchesQuery(workflow, query) {
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

	rangeEnd := rangeStart + query.Limit
	if rangeEnd > len(workflows) {
		rangeEnd = len(workflows)
	}
	nextPageToken := ""
	if rangeEnd < len(workflows) {
		nextPageToken = workflows[rangeEnd-1].ID
	}

	return workflows[rangeStart:rangeEnd], nextPageToken, nil
}

func (s MemoryStore) matchesQuery(workflow models.Workflow, query *store.WorkflowQuery) bool {
	if workflow.WorkflowDefinition.Name != query.DefinitionName {
		return false
	}

	if query.Status != "" && string(workflow.Status) != query.Status {
		return false
	}

	return true
}

func (s MemoryStore) GetWorkflowByID(id string) (models.Workflow, error) {
	if _, ok := s.workflows[id]; !ok {
		return models.Workflow{}, store.NewNotFound(id)
	}

	return s.workflows[id], nil
}

type byLastUpdatedTime []models.Workflow

func (b byLastUpdatedTime) Len() int      { return len(b) }
func (b byLastUpdatedTime) Swap(i, j int) { b[i], b[j] = b[j], b[i] }
func (b byLastUpdatedTime) Less(i, j int) bool {
	return time.Time(b[i].LastUpdated).Before(time.Time(b[j].LastUpdated))
}

func (s MemoryStore) GetPendingWorkflowIDs() ([]string, error) {
	var pendingWorkflows []models.Workflow
	for _, wf := range s.workflows {
		if !(wf.Status == models.WorkflowStatusQueued || wf.Status == models.WorkflowStatusRunning) {
			continue
		}
		wfcopy := wf // don't store loop variables
		pendingWorkflows = append(pendingWorkflows, wfcopy)
	}
	sort.Sort(byLastUpdatedTime(pendingWorkflows))
	pendingWorkflowIDs := []string{}
	for _, pendingWorkflow := range pendingWorkflows {
		pendingWorkflowIDs = append(pendingWorkflowIDs, pendingWorkflow.ID)
	}
	return pendingWorkflowIDs, nil

}

func (s MemoryStore) LockWorkflow(id string) error {
	if _, ok := s.workflowsLocked[id]; ok {
		return store.ErrWorkflowLocked
	}
	s.workflowsLocked[id] = struct{}{}
	return nil
}

func (s MemoryStore) UnlockWorkflow(id string) error {
	delete(s.workflowsLocked, id)
	return nil
}
