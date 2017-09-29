package main

import (
	"testing"

	"github.com/Clever/workflow-manager/gen-go/models"
	"github.com/stretchr/testify/assert"
)

// TestNewWorkflowDefinitionFromRequest tests the newWorkflowFromRequest helper
func TestNewWorkflowDefinitionFromRequest(t *testing.T) {
	workflowReq := models.NewWorkflowDefinitionRequest{
		Name: "test-workflow",
		StateMachine: &models.SLStateMachine{
			StartAt: "start-state",
			States: map[string]models.SLState{
				"start-state": models.SLState{
					Type:     models.SLStateTypeTask,
					Next:     "second-state",
					Resource: "test-resource",
				},
				"second-state": models.SLState{
					Type:     models.SLStateTypeTask,
					Next:     "end-state",
					Resource: "test-resource-2",
				},
				"end-state": models.SLState{
					Type:     models.SLStateTypeTask,
					Resource: "test-resource-3",
					End:      true,
				},
			},
		},
	}

	_, err := newWorkflowDefinitionFromRequest(workflowReq)
	t.Log("No error converting from new workflow request to resource")
	assert.Nil(t, err)
}

func TestValidateTagsMap(t *testing.T) {
	apiTags := map[string]interface{}{"team": "infra", "k": "v"}
	assert.Nil(t, validateTagsMap(apiTags))
	apiTags["num"] = 4
	assert.Error(t, validateTagsMap(apiTags))
}
