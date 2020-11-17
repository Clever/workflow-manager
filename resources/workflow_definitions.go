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

func reachable(stateName string, stateMachine *models.SLStateMachine) (bool, error) {
	// A state is reachable if
	// (1) it exists in the state machine definition
	// (2) it is either (a) the StartAt state or (b) another state contains a transition into it

	if !stateExists(stateName, stateMachine) {
		return false, nil
	}

	if stateMachine.StartAt == stateName {
		return true, nil
	}

	for _, state := range stateMachine.States {
		switch state.Type {
		case models.SLStateTypePass, models.SLStateTypeTask, models.SLStateTypeWait, models.SLStateTypeParallel, models.SLStateTypeMap:
			if state.Next == stateName {
				return true, nil
			}
			for _, catcher := range state.Catch {
				if catcher.Next == stateName {
					return true, nil
				}
			}
		case models.SLStateTypeChoice:
			for _, choice := range state.Choices {
				if choice.Next == stateName {
					return true, nil
				}
			}
			if state.Default == stateName {
				return true, nil
			}
		case models.SLStateTypeSucceed, models.SLStateTypeFail:
			// these states don't contain transitions
		default:
			return false, fmt.Errorf("%s states not supported yet", state.Type)
		}
	}
	return false, nil
}

func hasValidTransitions(stateName string, state models.SLState, stateMachine *models.SLStateMachine) error {
	// A state has a valid transition if it is an End state or all of its "Next"s are defined
	if state.End && len(state.Next) > 0 {
		return fmt.Errorf("Can only be an End state or have a Next state not both")
	}
	if (!state.End && len(state.Next) == 0) &&
		state.Type != models.SLStateTypeFail &&
		state.Type != models.SLStateTypeSucceed &&
		state.Type != models.SLStateTypeChoice {
		return fmt.Errorf("Must either be an End state, Choice state, or have a valid Next state")
	}

	if state.End {
		return nil
	}

	switch state.Type {
	case models.SLStateTypePass, models.SLStateTypeTask, models.SLStateTypeWait, models.SLStateTypeParallel, models.SLStateTypeMap:
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
	default:
		return fmt.Errorf("%s states not supported yet", state.Type)
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
			canReach, err := reachable(stateName, stateMachine)
			if err != nil {
				return err
			} else if !canReach {
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
	for _, state := range stateMachine.States {
		if state.Type == models.SLStateTypeParallel {
			for _, branch := range state.Branches {
				if err := RemoveInactiveStates(branch); err != nil {
					return err
				}
			}
		} else if state.Type == models.SLStateTypeMap {
			if err := RemoveInactiveStates(state.Iterator); err != nil {
				return err
			}
		}
	}
	return nil
}
