package main

import (
	"testing"

	"github.com/Clever/workflow-manager/gen-go/models"
	"github.com/stretchr/testify/assert"
)

// TestNewWorkflowDefinitionFromRequest tests the newWorkflowFromRequest helper
func TestNewWorkflowDefinitionFromRequest(t *testing.T) {
	workflowReq := models.NewWorkflowDefinitionRequest{
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

	wf, err := newWorkflowDefinitionFromRequest(workflowReq)
	t.Log("No error converting from new workflow request to resource")
	assert.Nil(t, err)

	// We currently only support linear workflows (single dependency)
	// and we also require that start-state does not have dependencies.
	// i.e. start states in the middle of the workflow are not allowed.
	for _, s := range wf.States() {
		if s.Name() == wf.StartAt().Name() {
			t.Logf("Starting state has no dependencies")
			assert.Empty(t, s.Dependencies())
		} else {
			t.Logf("State %s has the correct infered dependency", s.Name())
			depName := s.Dependencies()[0]
			assert.Equal(t, wf.States()[depName].Next(), s.Name())
		}
	}
}

// TestStoreTagsFromAPI tests the storeTagsFromAPI helper
func TestStoreTagsFromAPI(t *testing.T) {
	// test with valid (string-string) tags
	apiTags := map[string]interface{}{"team": "infra", "k": "v"}
	expected1 := map[string]string{"team": "infra", "k": "v"}
	tags1, err := storeTagsFromAPI(apiTags)
	assert.Nil(t, err)
	assert.Equal(t, tags1, expected1)
	// test with an invalid tag - value must be string
	apiTags["num"] = 4
	_, err = storeTagsFromAPI(apiTags)
	assert.Error(t, err, "error converting value to string: 4")
}
