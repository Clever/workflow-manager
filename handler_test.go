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

type handlerTestController struct {
	mockController *gomock.Controller
	mockWFM        *mocks.MockWorkflowManager
	mockStore      *mocks.MockStore
	handler        Handler
	t              *testing.T
}

func newSFNManagerTestController(t *testing.T) *handlerTestController {
	mockController := gomock.NewController(t)
	mockWFM := mocks.NewMockWorkflowManager(mockController)
	mockStore := mocks.NewMockStore(mockController)
	handler := Handler{
		manager: mockWFM,
		store:   mockStore,
	}

	return &handlerTestController{
		mockController: mockController,
		mockWFM:        mockWFM,
		mockStore:      mockStore,
		handler:        handler,
		t:              t,
	}
}

func (c *handlerTestController) tearDown() {
	c.mockController.Finish()
}

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

func TestGetWorkflowByID(t *testing.T) {
	falsePtr := boolPtr(false)
	truePtr := boolPtr(true)
	specs := []struct {
		desc                 string
		omitExecutionHistory *bool
	}{
		{
			desc:                 "updates execution history when omitExecutionHistory is nil",
			omitExecutionHistory: nil,
		},
		{
			desc:                 "updates execution history when omitExecutionHistory is false",
			omitExecutionHistory: falsePtr,
		},
		{
			desc:                 "skips updating execution history when omitExecutionHistory is true",
			omitExecutionHistory: truePtr,
		},
	}

	for _, spec := range specs {
		t.Run(spec.desc, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			c := newSFNManagerTestController(t)

			wf := &models.Workflow{
				WorkflowSummary: models.WorkflowSummary{
					ID:     "workflow-id",
					Status: models.WorkflowStatusRunning,
					WorkflowDefinition: &models.WorkflowDefinition{
						StateMachine: &models.SLStateMachine{
							StartAt: "start",
							States: map[string]models.SLState{
								"start": models.SLState{
									Type:     models.SLStateTypeTask,
									End:      true,
									Resource: "resource-name",
								},
							},
						},
					},
				},
				Jobs: []*models.Job{
					{
						State:  "start",
						Input:  `{"snack":"grapes"}`,
						Status: models.JobStatusFailed,
					},
				},
			}

			c.mockStore.EXPECT().
				GetWorkflowByID(ctx, gomock.Eq(wf.ID)).
				Return(*wf, nil).
				Times(1)
			c.mockWFM.EXPECT().
				UpdateWorkflowSummary(ctx, gomock.Any()).
				Return(nil).
				Times(1)

			if spec.omitExecutionHistory != truePtr {
				c.mockWFM.EXPECT().
					UpdateWorkflowHistory(ctx, gomock.Any()).
					Return(nil).
					Times(1)
			}

			updatedWorkflow, err := c.handler.GetWorkflowByID(
				ctx,
				&models.GetWorkflowByIDInput{
					WorkflowID:           wf.ID,
					OmitExecutionHistory: spec.omitExecutionHistory,
				},
			)

			require.NoError(t, err, err)
			assert.Equal(t, wf.ID, updatedWorkflow.ID)
			assert.Equal(t, wf.WorkflowDefinition, updatedWorkflow.WorkflowDefinition)

			if spec.omitExecutionHistory == truePtr {
				assert.Nil(t, updatedWorkflow.Jobs)
			} else {
				assert.NotNil(t, updatedWorkflow.Jobs)
			}
		})
	}
}

func boolPtr(b bool) *bool {
	return &b
}
