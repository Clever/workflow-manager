package resources

import (
	uuid "github.com/satori/go.uuid"
)

type Job struct {
	Id       string
	Workflow WorkflowDefinition
	Input    map[string]interface{}
	Tasks    []Task // list of states submitted as tasks
}

func NewJob(wf WorkflowDefinition, input map[string]interface{}) Job {
	return Job{
		Id:       uuid.NewV4().String(),
		Workflow: wf,
		Input:    input,
	}
}
