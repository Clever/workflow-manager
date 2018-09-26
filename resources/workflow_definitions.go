package resources

import (
	"fmt"
	"time"

	"github.com/Clever/workflow-manager/gen-go/models"
	"github.com/go-openapi/strfmt"
	"github.com/mohae/deepcopy"
	uuid "github.com/satori/go.uuid"
)

// NewWorkflowDefinition creates a new Workflow
func NewWorkflowDefinition(
	name string, manager models.Manager,
	stateMachine *models.SLStateMachine,
	defaultTags map[string]interface{}) (*models.WorkflowDefinition, error) {
	return &models.WorkflowDefinition{
		ID:           uuid.NewV4().String(),
		Name:         name,
		Version:      0,
		CreatedAt:    strfmt.DateTime(time.Now()),
		Manager:      manager,
		StateMachine: stateMachine,
		DefaultTags:  defaultTags,
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
		DefaultTags:  def.DefaultTags,
	}
}

// CopyWorkflowDefinition creates a copy of an existing WorflowDefinition with the same
// name and version, but a new ID.
//
// This is used to create modifed versions of WorkflowDefinitions that are variations
// on the saved WorflowDefinition. e.g. for Retries
func CopyWorkflowDefinition(def models.WorkflowDefinition) models.WorkflowDefinition {
	newDef := deepcopy.Copy(def).(models.WorkflowDefinition)
	return newDef
}

func stateExists(stateName string, stateMachine *models.SLStateMachine) bool {
	_, ok := stateMachine.States[stateName]
	return ok
}

func reachable(stateName string, stateMachine *models.SLStateMachine) bool {
	// A state is reachable if
	// (1) it exists in the state machine definition
	// (2) it is either (a) the StartAt state or (b) another state contains a transition into it

	if !stateExists(stateName, stateMachine) {
		return false
	}

	if stateMachine.StartAt == stateName {
		return true
	}

	for _, state := range stateMachine.States {
		switch state.Type {
		case models.SLStateTypePass, models.SLStateTypeTask, models.SLStateTypeWait:
			if state.Next == stateName {
				return true
			}
			for _, catcher := range state.Catch {
				if catcher.Next == stateName {
					return true
				}
			}
		case models.SLStateTypeChoice:
			for _, choice := range state.Choices {
				if choice.Next == stateName {
					return true
				}
			}
			if state.Default == stateName {
				return true
			}
		case models.SLStateTypeSucceed, models.SLStateTypeFail:
			// these states don't contain transitions
		case models.SLStateTypeParallel:
			panic("parallel states not supported yet")
		default:
			panic(fmt.Sprintf("%s states not supported yet", state.Type))
		}
	}
	return false
}

func hasValidTransitions(stateName string, state models.SLState, stateMachine *models.SLStateMachine) error {
	// A state has a valid transition if it is an End state or all of its "Next"s are defined
	if state.End {
		return nil
	}
	switch state.Type {
	case models.SLStateTypePass, models.SLStateTypeTask, models.SLStateTypeWait:
		if !stateExists(state.Next, stateMachine) {
			return fmt.Errorf("invalid transition in '%s': '%s'", stateName, state.Next)
		}
	case models.SLStateTypeChoice:
		for _, choice := range state.Choices {
			if !stateExists(choice.Next, stateMachine) {
				return fmt.Errorf("invalid transition in '%s': '%s'", stateName, choice.Next)
			}
		}
		if state.Default != "" && !stateExists(state.Default, stateMachine) {
			return fmt.Errorf("invalid transition in '%s': '%s'", stateName, state.Default)
		}
		return nil
	case models.SLStateTypeSucceed, models.SLStateTypeFail:
		return nil
	case models.SLStateTypeParallel:
		panic("Parallel states not supported yet")
	default:
		panic(fmt.Sprintf("%s states not supported yet", state.Type))
	}
	return nil
}

// RemoveInactiveStates discards all states not reachable in the graph after the StartAt state
// Assumes that startAt and the states are valid
func RemoveInactiveStates(stateMachine *models.SLStateMachine) error {
	if !stateExists(stateMachine.StartAt, stateMachine) {
		return fmt.Errorf("State %s not found in StateMachine", stateMachine.StartAt)
	}

	// loop through all states in the state machine, discarding any unreachable states.
	// repeat until every state is reachable
	for {
		everyStateReachable := true
		for stateName := range stateMachine.States {
			if !reachable(stateName, stateMachine) {
				everyStateReachable = false
				delete(stateMachine.States, stateName)
			}
		}
		if everyStateReachable {
			break
		}
	}

	// loop through and check that all states have valid transitions
	for stateName, state := range stateMachine.States {
		if err := hasValidTransitions(stateName, state, stateMachine); err != nil {
			return err
		}
	}
	return nil
}
