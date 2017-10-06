package resources

import (
	"fmt"
	"time"

	"github.com/Clever/workflow-manager/gen-go/models"
	"github.com/Clever/workflow-manager/toposort"
	"github.com/go-openapi/strfmt"
	uuid "github.com/satori/go.uuid"
)

// NewWorkflowDefinition creates a new Workflow
func NewWorkflowDefinition(name string, manager models.Manager, stateMachine *models.SLStateMachine) (*models.WorkflowDefinition, error) {
	return &models.WorkflowDefinition{
		ID:           uuid.NewV4().String(),
		Name:         name,
		Version:      0,
		CreatedAt:    strfmt.DateTime(time.Now()),
		Manager:      manager,
		StateMachine: stateMachine,
	}, nil
}

func NewWorkflowDefinitionVersion(def *models.WorkflowDefinition, version int) *models.WorkflowDefinition {
	return &models.WorkflowDefinition{
		ID:           uuid.NewV4().String(),
		Name:         def.Name,
		Version:      int64(version),
		CreatedAt:    strfmt.DateTime(time.Now()),
		Manager:      def.Manager,
		StateMachine: def.StateMachine,
	}
}

type StateAndDeps struct {
	StateName string
	State     models.SLState
	Deps      []string
}

func OrderedStates(states map[string]models.SLState) ([]StateAndDeps, error) {
	var stateDeps = make(map[string][]string)
	for stateName, state := range states {
		if _, ok := stateDeps[stateName]; !ok {
			stateDeps[stateName] = []string{}
		}

		if !state.End {
			if _, ok := states[state.Next]; !ok {
				return nil, fmt.Errorf("%s.Next=%s, but %s not defined",
					stateName, state.Next, state.Next)
			}

			if _, ok := stateDeps[state.Next]; !ok {
				stateDeps[state.Next] = []string{stateName}
			} else {
				stateDeps[state.Next] = append(stateDeps[state.Next], stateName)
			}
		}
	}

	// get toposorted states
	sortedStates, err := toposort.Sort(stateDeps)
	if err != nil {
		return []StateAndDeps{}, err
	}

	// flatten but keep order
	orderedStates := []StateAndDeps{}
	for _, deps := range sortedStates {
		for _, dep := range deps {
			orderedStates = append(orderedStates, StateAndDeps{
				StateName: dep,
				State:     states[dep],
				Deps:      stateDeps[dep],
			})
		}
	}

	return orderedStates, nil
}

// RemoveInactiveStates discards all states not reachable in the graph after the StartAt state
// Assumes that startAt and the states are valid
func RemoveInactiveStates(stateMachine *models.SLStateMachine) error {
	// TODO: curently assuming only models.SLState.Type == Task
	activeStates := map[string]models.SLState{}
	currentStateName := stateMachine.StartAt
	for true {

		if _, ok := stateMachine.States[currentStateName]; !ok {
			return fmt.Errorf("State %s not found in StateMachine", currentStateName)
		}

		activeStates[currentStateName] = stateMachine.States[currentStateName]
		if activeStates[currentStateName].End {
			break
		}
		currentStateName = activeStates[currentStateName].Next
	}

	stateMachine.States = activeStates
	return nil
}
