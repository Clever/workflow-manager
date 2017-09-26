package memory

import (
	"fmt"
	"sort"
	"time"

	"github.com/satori/go.uuid"

	"github.com/Clever/workflow-manager/resources"
	"github.com/Clever/workflow-manager/store"
)

type MemoryStore struct {
	workflowDefinitions map[string][]resources.WorkflowDefinition
	workflows           map[string]resources.Workflow
	workflowsLocked     map[string]struct{}
	stateResources      map[string]resources.StateResource
}

type ByCreatedAt []resources.Workflow

func (a ByCreatedAt) Len() int           { return len(a) }
func (a ByCreatedAt) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByCreatedAt) Less(i, j int) bool { return a[i].CreatedAt.Before(a[j].CreatedAt) }

func New() MemoryStore {
	return MemoryStore{
		workflowDefinitions: map[string][]resources.WorkflowDefinition{},
		workflows:           map[string]resources.Workflow{},
		workflowsLocked:     map[string]struct{}{},
		stateResources:      map[string]resources.StateResource{},
	}
}

func (s MemoryStore) SaveWorkflowDefinition(def resources.WorkflowDefinition) error {

	if _, ok := s.workflowDefinitions[def.Name()]; ok {
		return store.NewConflict(def.Name())
	}
	def.CreatedAtTime = time.Now()
	s.workflowDefinitions[def.Name()] = []resources.WorkflowDefinition{def}
	return nil
}

func (s MemoryStore) UpdateWorkflowDefinition(def resources.WorkflowDefinition) (resources.WorkflowDefinition, error) {
	last, err := s.LatestWorkflowDefinition(def.Name())
	if err != nil {
		return def, err
	}

	newVersion := resources.NewWorkflowDefinitionVersion(def, last.Version()+1)
	newVersion.CreatedAtTime = time.Now()
	s.workflowDefinitions[def.Name()] = append(s.workflowDefinitions[def.Name()], newVersion)

	return newVersion, nil
}

// GetWorkflowDefinitions returns the latest version of all stored workflow definitions
func (s MemoryStore) GetWorkflowDefinitions() ([]resources.WorkflowDefinition, error) {
	workflowDefinitions := []resources.WorkflowDefinition{}
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
func (s MemoryStore) GetWorkflowDefinitionVersions(name string) ([]resources.WorkflowDefinition, error) {
	workflowDefinitions, ok := s.workflowDefinitions[name]
	if !ok {
		return []resources.WorkflowDefinition{}, store.NewNotFound(name)
	}

	return workflowDefinitions, nil
}

func (s MemoryStore) GetWorkflowDefinition(name string, version int) (resources.WorkflowDefinition, error) {
	if _, ok := s.workflowDefinitions[name]; !ok {
		return resources.WorkflowDefinition{}, store.NewNotFound(name)
	}

	if len(s.workflowDefinitions[name]) < version {
		return resources.WorkflowDefinition{}, store.NewNotFound(fmt.Sprintf("%s.%d", name, version))
	}

	return s.workflowDefinitions[name][version], nil
}

func (s MemoryStore) LatestWorkflowDefinition(name string) (resources.WorkflowDefinition, error) {
	if _, ok := s.workflowDefinitions[name]; !ok {
		return resources.WorkflowDefinition{}, store.NewNotFound(name)
	}

	return s.GetWorkflowDefinition(name, len(s.workflowDefinitions[name])-1)
}

func (s MemoryStore) SaveStateResource(res resources.StateResource) error {
	resourceName := res.Name
	if res.Namespace != "" {
		resourceName = fmt.Sprintf("%s--%s", res.Namespace, res.Name)
	}

	s.stateResources[resourceName] = res
	return nil
}

func (s MemoryStore) GetStateResource(name, namespace string) (resources.StateResource, error) {
	resourceName := name
	if namespace != "" {
		resourceName = fmt.Sprintf("%s--%s", namespace, name)
	}

	if _, ok := s.stateResources[resourceName]; !ok {
		return resources.StateResource{}, store.NewNotFound(resourceName)
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

func (s MemoryStore) SaveWorkflow(workflow resources.Workflow) error {
	if _, ok := s.workflows[workflow.ID]; ok {
		return store.NewConflict(workflow.ID)
	}
	workflow.CreatedAt = time.Now()
	workflow.LastUpdated = workflow.CreatedAt
	s.workflows[workflow.ID] = workflow
	return nil
}

func (s MemoryStore) UpdateWorkflow(workflow resources.Workflow) error {
	if _, ok := s.workflows[workflow.ID]; !ok {
		return store.NewNotFound(workflow.ID)
	}
	workflow.LastUpdated = time.Now()
	s.workflows[workflow.ID] = workflow
	return nil
}

func (s MemoryStore) GetWorkflows(
	query *store.WorkflowQuery,
) ([]resources.Workflow, string, error) {
	workflows := []resources.Workflow{}
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
			return []resources.Workflow{}, "", store.NewInvalidPageTokenError(err)
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

func (s MemoryStore) matchesQuery(workflow resources.Workflow, query *store.WorkflowQuery) bool {
	if workflow.WorkflowDefinition.Name() != query.DefinitionName {
		return false
	}

	if query.Status != "" && string(workflow.Status) != query.Status {
		return false
	}

	return true
}

func (s MemoryStore) GetWorkflowByID(id string) (resources.Workflow, error) {
	if _, ok := s.workflows[id]; !ok {
		return resources.Workflow{}, store.NewNotFound(id)
	}

	return s.workflows[id], nil
}

type byLastUpdatedTime []resources.Workflow

func (b byLastUpdatedTime) Len() int           { return len(b) }
func (b byLastUpdatedTime) Swap(i, j int)      { b[i], b[j] = b[j], b[i] }
func (b byLastUpdatedTime) Less(i, j int) bool { return b[i].LastUpdated.Before(b[j].LastUpdated) }

func (s MemoryStore) GetPendingWorkflowIDs() ([]string, error) {
	var pendingWorkflows []resources.Workflow
	for _, wf := range s.workflows {
		if !(wf.Status == resources.Queued || wf.Status == resources.Running) {
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
