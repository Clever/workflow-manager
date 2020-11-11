package resources

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/Clever/workflow-manager/gen-go/models"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/stretchr/testify/assert"
)

func TestRemoveInactiveStates(t *testing.T) {
	wf := KitchenSinkWorkflowDefinition(t)
	numStates := len(wf.StateMachine.States)

	t.Log("Removes states with no path to")
	wf.StateMachine.StartAt = "second-state"
	assert.Nil(t, RemoveInactiveStates(wf.StateMachine))
	assert.Equal(t, numStates-1, len(wf.StateMachine.States))
	assert.Contains(t, wf.StateMachine.States, "second-state")
	assert.Contains(t, wf.StateMachine.States, "end-state")
	assert.NotContains(t, wf.StateMachine.States, "start-state")

	t.Log("Removes states even if one is left")
	wf = KitchenSinkWorkflowDefinition(t)
	wf.StateMachine.StartAt = "end-state"
	assert.Nil(t, RemoveInactiveStates(wf.StateMachine))
	assert.Equal(t, 1, len(wf.StateMachine.States))
	assert.Contains(t, wf.StateMachine.States, "end-state")
	assert.NotContains(t, wf.StateMachine.States, "second-state")
	assert.NotContains(t, wf.StateMachine.States, "start-state")

	t.Log("Fails if links are missing")
	wf = KitchenSinkWorkflowDefinition(t)
	delete(wf.StateMachine.States, "second-state")
	assert.Error(t, RemoveInactiveStates(wf.StateMachine))

	t.Log("Fails if StartAt is missing")
	wf = KitchenSinkWorkflowDefinition(t)
	wf.StateMachine.StartAt = ""
	assert.Error(t, RemoveInactiveStates(wf.StateMachine))

	t.Log("Keeps all states if startAt does not change")
	wf = KitchenSinkWorkflowDefinition(t)
	assert.Nil(t, RemoveInactiveStates(wf.StateMachine))
	assert.Equal(t, numStates, len(wf.StateMachine.States))

	t.Log("Works with choice and success states")
	var sm models.SLStateMachine
	assert.Nil(t, json.Unmarshal([]byte(awsExampleChoiceStateMachine), &sm))
	assert.Nil(t, RemoveInactiveStates(&sm))
	var smInvalid models.SLStateMachine
	assert.Nil(t, json.Unmarshal([]byte(awsExampleChoiceStateMachineInvalid), &smInvalid))
	assert.Error(t, RemoveInactiveStates(&smInvalid))

	t.Log("Fails if parallel state machine is malformed")
	sm = models.SLStateMachine{
		Comment: "description",
		StartAt: "parallel-state",
		States: map[string]models.SLState{
			"parallel-state": models.SLState{
				Type: models.SLStateTypeParallel,
				End:  true,
				Branches: []*models.SLStateMachine{
					&models.SLStateMachine{
						StartAt: "branch1",
						States: map[string]models.SLState{
							"branch1": models.SLState{
								Type:     models.SLStateTypeTask,
								Resource: "fake-resource-3",
								End:      false,
								Retry:    []*models.SLRetrier{},
							},
						},
					},
					&models.SLStateMachine{
						StartAt: "branch2",
						States: map[string]models.SLState{
							"branch2": models.SLState{
								Type:     models.SLStateTypeTask,
								Resource: "fake-resource-4",
								End:      true,
								Retry:    []*models.SLRetrier{},
							},
						},
					},
				},
			},
		},
	}
	assert.Error(t, RemoveInactiveStates(&sm))
	t.Log("Fails if map state machine is malformed")
	sm = models.SLStateMachine{
		Comment: "description",
		StartAt: "map-state",
		States: map[string]models.SLState{
			"map-state": models.SLState{
				Type: models.SLStateTypeMap,
				End:  true,
				Iterator: &models.SLStateMachine{
					StartAt: "mapStateStart",
					States: map[string]models.SLState{
						"mapStateStart": models.SLState{
							Type:     models.SLStateTypeTask,
							Resource: "fake-resource-3",
							End:      false,
							Next:     "mapStateBad",
							Retry:    []*models.SLRetrier{},
						},
						"mapStateEnd": models.SLState{
							Type:     models.SLStateTypeTask,
							Resource: "fake-resource-4",
							End:      true,
							Retry:    []*models.SLRetrier{},
						},
					},
				},
			},
		},
	}
	assert.Error(t, RemoveInactiveStates(&sm))
}

func TestCopyWorflowDefinition(t *testing.T) {
	wf := KitchenSinkWorkflowDefinition(t)
	copy := CopyWorkflowDefinition(*wf)

	t.Log("Name, Version, Manager are equal")
	assert.Equal(t, wf.Name, copy.Name)
	assert.Equal(t, wf.Version, copy.Version)
	assert.Equal(t, wf.Version, copy.Version)
	assert.Equal(t, wf.Manager, copy.Manager)
	assert.Equal(t, wf.DefaultTags, copy.DefaultTags)

	t.Log("StateMachines are equal but pointers are different")
	assert.Equal(t, wf.StateMachine, copy.StateMachine)
	assert.True(t, reflect.DeepEqual(wf.StateMachine, copy.StateMachine))
	assert.False(t, wf.StateMachine == copy.StateMachine)

	t.Log("Changing States in copy does not affect original")
	for name, state := range copy.StateMachine.States {
		state.Resource = "testing"
		state.Retry = []*models.SLRetrier{&models.SLRetrier{
			MaxAttempts: aws.Int64(1),
		}}
		copy.StateMachine.States[name] = state
	}
	for name, _ := range wf.StateMachine.States {
		assert.NotEqual(t, wf.StateMachine.States[name].Resource, copy.StateMachine.States[name].Resource)
		assert.NotEqual(t, wf.StateMachine.States[name].Retry, copy.StateMachine.States[name].Retry)
	}
}
