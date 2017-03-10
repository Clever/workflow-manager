package store

import (
	"fmt"

	"github.com/Clever/workflow-manager/resources"
)

type MemoryStore struct {
	workflows map[string][]resources.WorkflowDefinition
	jobs      map[string]resources.Job
}

func NewMemoryStore() MemoryStore {
	return MemoryStore{
		workflows: map[string][]resources.WorkflowDefinition{},
		jobs:      map[string]resources.Job{},
	}
}

func (s MemoryStore) CreateWorkflow(def resources.WorkflowDefinition) error {

	if _, ok := s.workflows[def.Name()]; ok {
		return NewConflict(def.Name())
	}

	s.workflows[def.Name()] = []resources.WorkflowDefinition{def}
	return nil
}

func (s MemoryStore) UpdateWorkflow(name string, def resources.WorkflowDefinition) error {
	return nil
}

func (s MemoryStore) GetWorkflow(name string, version int) (resources.WorkflowDefinition, error) {
	if _, ok := s.workflows[name]; !ok {
		return resources.WorkflowDefinition{}, NewNotFound(name)
	}

	if len(s.workflows[name]) < version {
		return resources.WorkflowDefinition{}, NewNotFound(fmt.Sprintf("%s.%d", name, version))
	}

	return s.workflows[name][version], nil
}

func (s MemoryStore) LatestWorkflow(name string) (resources.WorkflowDefinition, error) {
	if _, ok := s.workflows[name]; !ok {
		return resources.WorkflowDefinition{}, NewNotFound(name)
	}

	return s.GetWorkflow(name, len(s.workflows[name])-1)
}

func (s MemoryStore) CreateJob(job resources.Job) error {
	if _, ok := s.jobs[job.ID]; ok {
		return NewConflict(job.ID)
	}

	s.jobs[job.ID] = job
	return nil
}

func (s MemoryStore) GetJob(id string) (resources.Job, error) {
	if _, ok := s.jobs[id]; !ok {
		return resources.Job{}, NewNotFound(id)
	}

	return s.jobs[id], nil
}
