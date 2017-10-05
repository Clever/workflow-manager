package resources

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOrderedStates(t *testing.T) {

	for i := 0; i < 10; i++ {
		wf := KitchenSinkWorkflowDefinition(t)
		orderedStates, err := OrderedStates(wf.StateMachine.States)
		assert.Nil(t, err)
		assert.Equal(t, "start-state", orderedStates[0].StateName)
	}

}
