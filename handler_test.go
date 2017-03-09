package main

import (
	"testing"

	"github.com/Clever/workflow-manager/gen-go/models"
	"github.com/Clever/workflow-manager/resources"
	"github.com/stretchr/testify/assert"
)

// TestNewWorkflowFromRequest tests the newWorkflowFromRequest helper
func TestNewWorkflowFromRequest(t *testing.T) {

	workflowReq := models.NewWorkflowRequest{
		Name:    "test-workflow",
		StartAt: "start-state",
		States: []*models.State{
			&models.State{
				Name:     "start-state",
				Type:     "Task",
				Next:     "second-state",
				Resource: "test-resource",
			},
			&models.State{
				Name:     "second-state",
				Type:     "Task",
				Next:     "end-state",
				Resource: "test-resource-2",
			},
			&models.State{
				Name:     "end-state",
				Type:     "Task",
				Resource: "test-resource-3",
				End:      true,
			},
		},
	}

	wf, err := newWorkflowFromRequest(workflowReq)
	t.Log("No error converting from new workflow request to resource")
	assert.Nil(t, err)

	var prevState resources.State
	for _, s := range wf.States() {
		if prevState == nil {
			// ignore first state
			prevState = s
			continue
		}
		t.Logf("State %s have inferred depdendencies", s.Name())
		assert.Equal(t, prevState.Name(), s.Dependencies()[0])
		prevState = s
	}
}
