package resources

type State interface {
	Name() string
	Type() string
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
	name    string
	version int
	states  map[string]State

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

func NewWorkflowDefinition(name string, desc string, states map[string]State) WorkflowDefinition {
	return WorkflowDefinition{
		name:        name,
		version:     0,
		states:      states,
		Description: desc,
	}
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

func (ws WorkerState) IsEnd() bool {
	return ws.end
}

func (ws WorkerState) Resource() string {
	return ws.resource
}

func (ws WorkerState) Name() string {
	return ws.name
}
