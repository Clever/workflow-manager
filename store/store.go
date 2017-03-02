package store

import "github.com/Clever/workflow-manager/resources"

// WorkflowStore defines the interface for persistence of Workflow defintions
type WorkflowStore interface {
	Create(def resources.WorkflowDefintion) error
	Update(name string, def resources.WorkflowDefinition) error
	Get(name string, version int) (resources.WorkflowDefinition, error)
	Latest(name string) (resources.WorkflowDefinition, error)
}
