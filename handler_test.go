package main

import (
	"context"
	"testing"

	"github.com/Clever/workflow-manager/gen-go/models"
	"github.com/Clever/workflow-manager/mocks"
	"github.com/Clever/workflow-manager/resources"
	"github.com/Clever/workflow-manager/store/memory"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewWorkflowDefinitionFromRequest tests the newWorkflowFromRequest helper
func TestNewWorkflowDefinitionFromRequest(t *testing.T) {
	workflowReq := models.NewWorkflowDefinitionRequest{
		Name:    "test-workflow",
		Manager: models.ManagerStepFunctions,
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

func TestParamsToWorkflowsQuery(t *testing.T) {
	boolTrue := true
	boolFalse := false
	failedString := "failed"
	definitionName := "defName"
	// error if status and resolvedByUser are both sent
	inputWithStatusAndResolvedTrue := &models.GetWorkflowsInput{
		ResolvedByUser:         &boolTrue,
		Status:                 &failedString,
		WorkflowDefinitionName: definitionName,
	}

	workflowQuery, err := paramsToWorkflowsQuery(inputWithStatusAndResolvedTrue)
	assert.Error(t, err)
	assert.IsType(t, models.BadRequest{}, err)

	// if status and resolvedByUser are sent, verify error
	inputWithStatusAndResolvedFalse := &models.GetWorkflowsInput{
		ResolvedByUser:         &boolFalse,
		Status:                 &failedString,
		WorkflowDefinitionName: definitionName,
	}
	workflowQuery, err = paramsToWorkflowsQuery(inputWithStatusAndResolvedFalse)
	assert.Error(t, err)

	// if resolvedByUser is sent, verify that the wrapper is created correctly
	inputWithResolvedTrue := &models.GetWorkflowsInput{
		ResolvedByUser:         &boolTrue,
		WorkflowDefinitionName: definitionName,
	}
	workflowQuery, err = paramsToWorkflowsQuery(inputWithResolvedTrue)
	assert.NoError(t, err)
	assert.Equal(t, true, workflowQuery.ResolvedByUserWrapper.IsSet)
	assert.Equal(t, true, workflowQuery.ResolvedByUserWrapper.Value)

	inputWithResolvedFalse := &models.GetWorkflowsInput{
		ResolvedByUser:         &boolFalse,
		WorkflowDefinitionName: definitionName,
	}
	workflowQuery, err = paramsToWorkflowsQuery(inputWithResolvedFalse)
	assert.NoError(t, err)
	assert.Equal(t, true, workflowQuery.ResolvedByUserWrapper.IsSet)
	assert.Equal(t, false, workflowQuery.ResolvedByUserWrapper.Value)

	// if resolvedByUser is NOT sent, verify that the wrapper is created correctly
	inputWithNameOnly := &models.GetWorkflowsInput{
		WorkflowDefinitionName: definitionName,
	}
	workflowQuery, err = paramsToWorkflowsQuery(inputWithNameOnly)
	assert.NoError(t, err)
	assert.Equal(t, false, workflowQuery.ResolvedByUserWrapper.IsSet)
}

func TestStartWorkflow(t *testing.T) {
	mockController := gomock.NewController(t)
	defer mockController.Finish()

	store := memory.New()
	mockWFM := mocks.NewMockWorkflowManager(mockController)

	workflowDefinition := resources.KitchenSinkWorkflowDefinition(t)
	require.NoError(t, store.SaveWorkflowDefinition(context.Background(), *workflowDefinition))

	h := Handler{
		manager: mockWFM,
		store:   store,
	}

	t.Log("Verify that StartWorkflow handler converts empty string to empty dictionary")
	for _, input := range []string{"", "{}"} {
		mockWFM.EXPECT().
			CreateWorkflow(gomock.Any(), gomock.Any(), "{}", gomock.Any(), gomock.Any(), gomock.Any()).
			Return(&models.Workflow{}, nil)

		_, err := h.StartWorkflow(context.Background(), &models.StartWorkflowRequest{
			Input: input,
			WorkflowDefinition: &models.WorkflowDefinitionRef{
				Name:    workflowDefinition.Name,
				Version: -1,
			},
		})
		assert.NoError(t, err)
	}
}
