package resources

import (
	"fmt"

	"github.com/Clever/workflow-manager/toposort"
)

// State refers to the different states in a Workflow
type State interface {
	// State constants
	Name() string
	Type() string
	Next() string
	Resource() string
	IsEnd() bool

	// State metadata
	Dependencies() []string
	AddDependency(State)
}

// Workflow defines an interface for defining a flow of States
type Workflow interface {
	Name() string
	Version() int
	StartAt() State
	States() map[string]State
	OrderedStates() []State
}

// WorkflowDefinition defines a named ordered set of
// states. Currently does NOT define a DAG.
type WorkflowDefinition struct {
	name          string
	version       int
	startAt       string
	states        map[string]State
	orderedStates []State

	Description string
}

// Name returns the name of the workflow
func (wf WorkflowDefinition) Name() string {
	return wf.name
}

// Version returns the revision of the workflow
// i.e. how many times has this definition been updated
func (wf WorkflowDefinition) Version() int {
	return wf.version
}

// States returns all the states in this workflow as a map
func (wf WorkflowDefinition) States() map[string]State {
	return wf.states
}

// OrderedStates returns an ordered list of states in the
// order of execution required
func (wf WorkflowDefinition) OrderedStates() []State {
	return wf.orderedStates
}

// StartAt returns the first state that should be executed for this workflow
func (wf WorkflowDefinition) StartAt() State {
	return wf.states[wf.startAt]
}

// NewWorkflowDefinition creates a new Workflow
func NewWorkflowDefinition(name string, desc string, startAt string, states map[string]State) (WorkflowDefinition, error) {

	orderedStates, err := orderStates(states)
	if err != nil {
		return WorkflowDefinition{}, err
	}

	return WorkflowDefinition{
		name:          name,
		version:       0,
		startAt:       startAt,
		states:        states,
		orderedStates: orderedStates,
		Description:   desc,
	}, nil

}

func NewWorkflowDefinitionVersion(def WorkflowDefinition, version int) WorkflowDefinition {
	return WorkflowDefinition{
		name:          def.Name(),
		version:       version,
		startAt:       def.StartAt().Name(),
		states:        def.States(),
		orderedStates: def.OrderedStates(),
		Description:   def.Description,
	}
}

// currently uses toposort for an ordered list
func orderStates(states map[string]State) ([]State, error) {
	var stateDeps = map[string][]string{}
	for _, s := range states {
		stateDeps[s.Name()] = []string{}
		for _, d := range s.Dependencies() {
			stateDeps[s.Name()] = append(stateDeps[s.Name()], d)
		}
	}

	// get toposorted states
	sortedStates, err := toposort.Sort(stateDeps)
	if err != nil {
		return []State{}, err
	}

	// flatten but keep order
	orderedStates := []State{}
	for _, deps := range sortedStates {
		for _, dep := range deps {
			orderedStates = append(orderedStates, states[dep])
		}
	}

	return orderedStates, nil
}

// WorkerState implements the State interface for workers runnin in containers
type WorkerState struct {
	name         string
	next         string
	resource     string
	dependencies []string
	end          bool
}

// NewWorkerState creates a new struct
func NewWorkerState(name, next, resource string, end bool) (*WorkerState, error) {
	if end && next != "" {
		return &WorkerState{}, fmt.Errorf("End state can not have a next")
	}
	if !end && next == "" {
		return &WorkerState{}, fmt.Errorf("Next must be defined for non-end state")
	}

	return &WorkerState{name, next, resource, []string{}, end}, nil
}

// Type of a WorkerState is WORKER
func (ws *WorkerState) Type() string {
	return "WORKER"
}

// Next returns the name of the state to run after successful execution
// of this one
func (ws *WorkerState) Next() string {
	return ws.next
}

// Dependencies returns the names of States that this state depends on
func (ws *WorkerState) Dependencies() []string {
	return ws.dependencies
}

// AddDependency adds a new dependency for this state
func (ws *WorkerState) AddDependency(s State) {
	ws.dependencies = append(ws.dependencies, s.Name())
}

// IsEnd returns true if this State is the last one for the workflow
func (ws *WorkerState) IsEnd() bool {
	return ws.end
}

// Resource the resource that needs to be executed as
// part of a task for this State
func (ws *WorkerState) Resource() string {
	return ws.resource
}

// Name of this state
func (ws *WorkerState) Name() string {
	return ws.name
}
