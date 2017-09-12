package resources

import (
	"encoding/gob"
	"fmt"
	"time"

	"github.com/Clever/workflow-manager/gen-go/models"
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
	Retry() []*models.Retrier

	// State metadata
	Dependencies() []string
	AddDependency(State)
}

// WorkflowIface defines an interface for defining a flow of States
type WorkflowIface interface {
	Name() string
	Version() int
	CreatedAt() time.Time
	StartAt() State
	States() map[string]State
	OrderedStates() []State
}

// WorkflowDefinition defines a named ordered set of
// states. Currently does NOT define a DAG.
type WorkflowDefinition struct {
	NameStr          string
	VersionInt       int
	CreatedAtTime    time.Time
	StartAtStr       string
	StatesMap        map[string]State
	OrderedStatesArr []State

	Description string
}

// Name returns the name of the workflow
func (wf WorkflowDefinition) Name() string {
	return wf.NameStr
}

// CreatedAt returns the time the workflow was created
func (wf WorkflowDefinition) CreatedAt() time.Time {
	return wf.CreatedAtTime
}

// Version returns the revision of the workflow
// i.e. how many times has this definition been updated
func (wf WorkflowDefinition) Version() int {
	return wf.VersionInt
}

// States returns all the states in this workflow as a map
func (wf WorkflowDefinition) States() map[string]State {
	return wf.StatesMap
}

// OrderedStates returns an ordered list of states in the
// order of execution required
func (wf WorkflowDefinition) OrderedStates() []State {
	return wf.OrderedStatesArr
}

// StartAt returns the first state that should be executed for this workflow
func (wf WorkflowDefinition) StartAt() State {
	return wf.StatesMap[wf.StartAtStr]
}

// NewWorkflowDefinition creates a new Workflow
func NewWorkflowDefinition(name string, desc string, startAt string, states map[string]State) (WorkflowDefinition, error) {
	orderedStates, err := orderStates(states)
	if err != nil {
		return WorkflowDefinition{}, err
	}

	return WorkflowDefinition{
		NameStr:          name,
		VersionInt:       0,
		CreatedAtTime:    time.Now(),
		StartAtStr:       startAt,
		StatesMap:        states,
		OrderedStatesArr: orderedStates,
		Description:      desc,
	}, nil

}

func NewWorkflowDefinitionVersion(def WorkflowDefinition, version int) WorkflowDefinition {
	return WorkflowDefinition{
		NameStr:          def.Name(),
		VersionInt:       version,
		CreatedAtTime:    time.Now(),
		StartAtStr:       def.StartAt().Name(),
		StatesMap:        def.States(),
		OrderedStatesArr: def.OrderedStates(),
		Description:      def.Description,
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

// WorkerState implements the State interface for workers running in containers
type WorkerState struct {
	NameStr         string
	NextStr         string
	ResourceStr     string
	DependenciesArr []string
	End             bool
	Retriers        []*models.Retrier
}

// Currently the way we persist worker state is via GOB encoding, since it is the
// only encoding that can encode interface types (e.g. State).
// In order for GOB encoding to work, we have to Register types that implement encoded interfaces, e.g. State.
func init() {
	gob.Register(&WorkerState{})
}

// NewWorkerState creates a new struct
func NewWorkerState(name, next, resource string, end bool, retriers []*models.Retrier) (*WorkerState, error) {
	if end && next != "" {
		return &WorkerState{}, fmt.Errorf("End state can not have a next")
	}
	if !end && next == "" {
		return &WorkerState{}, fmt.Errorf("Next must be defined for non-end state")
	}

	return &WorkerState{
		NameStr:         name,
		NextStr:         next,
		ResourceStr:     resource,
		DependenciesArr: []string{},
		End:             end,
		Retriers:        retriers,
	}, nil
}

// Type of a WorkerState is WORKER
func (ws *WorkerState) Type() string {
	return "WORKER"
}

// Next returns the name of the state to run after successful execution
// of this one
func (ws *WorkerState) Next() string {
	return ws.NextStr
}

// Dependencies returns the names of States that this state depends on
func (ws *WorkerState) Dependencies() []string {
	return ws.DependenciesArr
}

// AddDependency adds a new dependency for this state
func (ws *WorkerState) AddDependency(s State) {
	ws.DependenciesArr = append(ws.DependenciesArr, s.Name())
}

// IsEnd returns true if this State is the last one for the workflow
func (ws *WorkerState) IsEnd() bool {
	return ws.End
}

// Retry returns the retry policy for the state.
func (ws *WorkerState) Retry() []*models.Retrier {
	return ws.Retriers
}

// Resource is the name of the resource that needs to be executed as
// part of a task for this State
func (ws *WorkerState) Resource() string {
	return ws.ResourceStr
}

// Name of this state
func (ws *WorkerState) Name() string {
	return ws.NameStr
}
