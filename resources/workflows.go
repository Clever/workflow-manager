package resources

import "github.com/Clever/catapult/toposort"

type State interface {
	Name() string
	Type() string
	Next() string
	Resource() string
	IsEnd() bool
}

type Workflow interface {
	Name() string
	Version() string
	States() map[string]State
	NextState(stateName string) State
}

// WorkflowDefinition defines a named ordered set of
// states. Currently does NOT define a DAG.
type WorkflowDefinition struct {
	name          string
	version       int
	states        map[string]State
	orderedStates []State

	Description string
}

func (wf WorkflowDefinition) Name() string {
	return wf.name
}

func (wf WorkflowDefinition) Version() int {
	return wf.version
}

func (wf WorkflowDefinition) States() map[string]State {
	return wf.states
}

func NewWorkflowDefinition(name string, desc string, states map[string]State) (WorkflowDefinition, error) {

	orderedStates, err := orderStates(states)
	if err != nil {
		return WorkflowDefinition{}, err
	}

	return WorkflowDefinition{
		name:          name,
		version:       0,
		states:        states,
		orderedStates: orderedStates,
		Description:   desc,
	}, nil

}

// TODO: this is where to insert DAG ordering and traversal code
func orderStates(states map[string]State) ([]State, error) {
	// currently uses catapult/toposort for an ordered list
	var stateDeps = map[string][]string{}
	for _, s := range states {
		// add state to the map if it does not exist
		if _, ok := stateDeps[s.Name()]; !ok {
			stateDeps[s.Name()] = []string{}
		}

		// add state as a dependency of s.Next
		if _, ok := stateDeps[s.Next()]; ok {
			stateDeps[s.Next()] = append(stateDeps[s.Next()], s.Name())
		} else {
			stateDeps[s.Next()] = []string{s.Name()}
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

type WorkerState struct {
	name     string
	next     string
	resource string
	end      bool
}

func NewWorkerState(name, next, resource string, end bool) WorkerState {
	return WorkerState{name, next, resource, end}
}

func (ws WorkerState) Type() string {
	return "WORKER"
}

func (ws WorkerState) Next() string {
	return ws.next
}

func (ws WorkerState) IsEnd() bool {
	return ws.end
}

func (ws WorkerState) Resource() string {
	return ws.resource
}

func (ws WorkerState) Name() string {
	return ws.name
}
