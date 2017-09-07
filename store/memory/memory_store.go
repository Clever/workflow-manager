package memory

import (
	"fmt"
	"sort"
	"time"

	"github.com/Clever/workflow-manager/resources"
	"github.com/Clever/workflow-manager/store"
)

type MemoryStore struct {
	workflows      map[string][]resources.WorkflowDefinition
	jobs           map[string]resources.Job
	stateResources map[string]resources.StateResource
}

type ByCreatedAt []resources.Job

func (a ByCreatedAt) Len() int           { return len(a) }
func (a ByCreatedAt) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByCreatedAt) Less(i, j int) bool { return a[i].CreatedAt.Before(a[j].CreatedAt) }

func New() MemoryStore {
	return MemoryStore{
		workflows:      map[string][]resources.WorkflowDefinition{},
		jobs:           map[string]resources.Job{},
		stateResources: map[string]resources.StateResource{},
	}
}

func (s MemoryStore) SaveWorkflowDefinition(def resources.WorkflowDefinition) error {

	if _, ok := s.workflows[def.Name()]; ok {
		return store.NewConflict(def.Name())
	}
	def.CreatedAtTime = time.Now()
	s.workflows[def.Name()] = []resources.WorkflowDefinition{def}
	return nil
}

func (s MemoryStore) UpdateWorkflowDefinition(def resources.WorkflowDefinition) (resources.WorkflowDefinition, error) {
	last, err := s.LatestWorkflowDefinition(def.Name())
	if err != nil {
		return def, err
	}

	newVersion := resources.NewWorkflowDefinitionVersion(def, last.Version()+1)
	newVersion.CreatedAtTime = time.Now()
	s.workflows[def.Name()] = append(s.workflows[def.Name()], newVersion)

	return newVersion, nil
}

// GetWorkflowDefinitions returns the latest version of all stored workflow definitions
func (s MemoryStore) GetWorkflowDefinitions() ([]resources.WorkflowDefinition, error) {
	workflows := []resources.WorkflowDefinition{}
	// for each workflow definition
	for _, versionedWorkflowDefinitions := range s.workflows {
		// for each version of a workflow definition
		for _, workflow := range versionedWorkflowDefinitions {
			workflows = append(workflows, workflow)
		}
	}

	return workflows, nil
}

// GetWorkflowDefinitionVersions gets all versions of a workflow definition
func (s MemoryStore) GetWorkflowDefinitionVersions(name string) ([]resources.WorkflowDefinition, error) {
	workflows, ok := s.workflows[name]
	if !ok {
		return []resources.WorkflowDefinition{}, store.NewNotFound(name)
	}

	return workflows, nil
}

func (s MemoryStore) GetWorkflowDefinition(name string, version int) (resources.WorkflowDefinition, error) {
	if _, ok := s.workflows[name]; !ok {
		return resources.WorkflowDefinition{}, store.NewNotFound(name)
	}

	if len(s.workflows[name]) < version {
		return resources.WorkflowDefinition{}, store.NewNotFound(fmt.Sprintf("%s.%d", name, version))
	}

	return s.workflows[name][version], nil
}

func (s MemoryStore) LatestWorkflowDefinition(name string) (resources.WorkflowDefinition, error) {
	if _, ok := s.workflows[name]; !ok {
		return resources.WorkflowDefinition{}, store.NewNotFound(name)
	}

	return s.GetWorkflowDefinition(name, len(s.workflows[name])-1)
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

func (s MemoryStore) SaveJob(job resources.Job) error {
	if _, ok := s.jobs[job.ID]; ok {
		return store.NewConflict(job.ID)
	}
	job.CreatedAt = time.Now()
	job.LastUpdated = job.CreatedAt
	s.jobs[job.ID] = job
	return nil
}

func (s MemoryStore) UpdateJob(job resources.Job) error {
	if _, ok := s.jobs[job.ID]; !ok {
		return store.NewNotFound(job.ID)
	}
	job.LastUpdated = time.Now()
	s.jobs[job.ID] = job
	return nil
}

func (s MemoryStore) GetJobsForWorkflowDefinition(workflowName string) ([]resources.Job, error) {
	jobs := []resources.Job{}
	for _, job := range s.jobs {
		if job.WorkflowDefinition.Name() == workflowName {
			jobs = append(jobs, job)
		}
	}
	sort.Sort(sort.Reverse(ByCreatedAt(jobs))) // reverse to get newest first
	return jobs, nil
}

func (s MemoryStore) GetJob(id string) (resources.Job, error) {
	if _, ok := s.jobs[id]; !ok {
		return resources.Job{}, store.NewNotFound(id)
	}

	return s.jobs[id], nil
}
