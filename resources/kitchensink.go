package resources

import (
	"fmt"
	"testing"
	"time"

	"github.com/Clever/workflow-manager/gen-go/models"
	"github.com/go-openapi/swag"
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
			[]*models.Retrier{{
				ErrorEquals: []models.ErrorEquals{models.ErrorEqualsStatesALL},
				MaxAttempts: swag.Int64(2),
			}},
		},
		"second-state": &WorkerState{
			"second-state",
			"end-state",
			"fake-resource-2",
			[]string{"start-state"},
			false,
			[]*models.Retrier{},
		},
		"end-state": &WorkerState{
			"end-state",
			"",
			"fake-resource-3",
			[]string{"second-state"},
			true,
			[]*models.Retrier{},
		},
	}

	wf, err := NewWorkflowDefinition(fmt.Sprintf("kitchensink-%s", time.Now().Format(time.RFC3339Nano)),
		"description",
		"start-state",
		states)
	assert.Nil(t, err)

	return wf
}
