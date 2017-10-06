package resources

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOrderedStates(t *testing.T) {
	// run multiple times since map ordering
	for i := 0; i < 10; i++ {
		wf := KitchenSinkWorkflowDefinition(t)
		orderedStates, err := OrderedStates(wf.StateMachine.States)
		assert.Nil(t, err)
		assert.Equal(t, "start-state", orderedStates[0].StateName)
	}
}

func TestActiveStates(t *testing.T) {
	wf := KitchenSinkWorkflowDefinition(t)
	// save original state machine
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
}
