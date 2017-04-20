package memory

import (
	"fmt"
	"time"

	"github.com/Clever/workflow-manager/resources"
	"github.com/Clever/workflow-manager/store"
)

type MemoryStore struct {
	workflows map[string][]resources.WorkflowDefinition
	jobs      map[string]resources.Job
}

func New() MemoryStore {
	return MemoryStore{
		workflows: map[string][]resources.WorkflowDefinition{},
		jobs:      map[string]resources.Job{},
	}
}

func (s MemoryStore) SaveWorkflow(def resources.WorkflowDefinition) error {

	if _, ok := s.workflows[def.Name()]; ok {
		return store.NewConflict(def.Name())
	}
	def.CreatedAt = time.Now()
	s.workflows[def.Name()] = []resources.WorkflowDefinition{def}
	return nil
}

func (s MemoryStore) UpdateWorkflow(def resources.WorkflowDefinition) (resources.WorkflowDefinition, error) {
	last, err := s.LatestWorkflow(def.Name())
	if err != nil {
		return def, err
	}

	newVersion := resources.NewWorkflowDefinitionVersion(def, last.Version()+1)
	newVersion.CreatedAt = time.Now()
	s.workflows[def.Name()] = append(s.workflows[def.Name()], newVersion)

	return newVersion, nil
}

func (s MemoryStore) GetWorkflow(name string, version int) (resources.WorkflowDefinition, error) {
	if _, ok := s.workflows[name]; !ok {
		return resources.WorkflowDefinition{}, store.NewNotFound(name)
	}

	if len(s.workflows[name]) < version {
		return resources.WorkflowDefinition{}, store.NewNotFound(fmt.Sprintf("%s.%d", name, version))
	}

	return s.workflows[name][version], nil
}

func (s MemoryStore) LatestWorkflow(name string) (resources.WorkflowDefinition, error) {
	if _, ok := s.workflows[name]; !ok {
		return resources.WorkflowDefinition{}, store.NewNotFound(name)
	}

	return s.GetWorkflow(name, len(s.workflows[name])-1)
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

func (s MemoryStore) GetJobsForWorkflow(workflowName string) ([]resources.Job, error) {
	jobs := []resources.Job{}
	for _, job := range s.jobs {
		if job.Workflow.Name() == workflowName {
			jobs = append([]resources.Job{job}, jobs...) // newest first
		}
	}

	return jobs, nil
}

func (s MemoryStore) GetJob(id string) (resources.Job, error) {
	if _, ok := s.jobs[id]; !ok {
		return resources.Job{}, store.NewNotFound(id)
	}

	return s.jobs[id], nil
}
