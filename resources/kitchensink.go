package resources

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// KitchenSinkWorkflow returns a Workflow resource to use for tests
func KitchenSinkWorkflow(t *testing.T) WorkflowDefinition {

	states := map[string]State{
		"start-state": &WorkerState{
			"start-state",
			"second-state",
			"fake-resource-1",
			[]string{},
			false,
		},
		"second-state": &WorkerState{
			"second-state",
			"end-state",
			"fake-resource-2",
			[]string{"start-state"},
			false,
		},
		"end-state": &WorkerState{
			"end-state",
			"",
			"fake-resource-3",
			[]string{"second-state"},
			true,
		},
	}

	wf, err := NewWorkflowDefinition(fmt.Sprintf("kitchensink-%s", time.Now().Format(time.RFC3339Nano)),
		"description",
		"start-state",
		states)
	assert.Nil(t, err)

	return wf
}
