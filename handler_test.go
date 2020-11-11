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
					Next:     "parallel-state",
					Resource: "test-resource-2",
				},
				"parallel-state": models.SLState{
					Type: models.SLStateTypeParallel,
					Next: "map-state",
					End:  false,
					Branches: []*models.SLStateMachine{
						&models.SLStateMachine{
							StartAt: "branch1",
							States: map[string]models.SLState{
								"branch1": models.SLState{
									Type:     models.SLStateTypeTask,
									Resource: "fake-resource-3",
									End:      true,
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
				"map-state": models.SLState{
					Type: models.SLStateTypeMap,
					Next: "end-state",
					End:  false,
					Iterator: &models.SLStateMachine{
						StartAt: "mapStateStart",
						States: map[string]models.SLState{
							"mapStateStart": models.SLState{
								Type:     models.SLStateTypeTask,
								Resource: "fake-resource-5",
								End:      true,
								Retry:    []*models.SLRetrier{},
							},
						},
					},
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

func TestValidateMapState(t *testing.T) {
	state := models.SLState{
		Type: models.SLStateTypeMap,
		End:  true,
	}

	t.Log("Error if there is no use of the Iterator field")
	err := validateMapState(state)
	assert.Error(t, err)

	state.Iterator = &models.SLStateMachine{
		StartAt: "mapStateStart",
		States: map[string]models.SLState{
			"mapStateStart": models.SLState{
				Type:     models.SLStateTypeTask,
				Resource: "fake-resource-5",
				End:      true,
				Retry:    []*models.SLRetrier{},
			},
		},
	}
	t.Log("No error with correctly formatted map state")
	state.Parameters = map[string]interface{}{
		"_EXECUTION_NAME.$": "$._EXECUTION_NAME",
	}
	err = validateMapState(state)
	assert.Nil(t, err)
}

func TestValidateTaskState(t *testing.T) {
	state := models.SLState{
		Type: models.SLStateTypeTask,
		End:  true,
	}
	err := validateTaskState(state)
	t.Log("Error if there is not use of Resource")
	assert.Error(t, err)

	t.Log("No error with correctly formatted Task state")
	state.Resource = "fake-resource"
	err = validateTaskState(state)
	assert.Nil(t, err)
}

func TestValidateParallelState(t *testing.T) {
	state := models.SLState{
		Type: models.SLStateTypeParallel,
		End:  true,
	}

	t.Log("Error if branches field is not specified")
	err := validateParallelState(state)
	assert.Error(t, err)

	t.Log("Error if branch is malformed")
	state.Branches = []*models.SLStateMachine{
		&models.SLStateMachine{
			StartAt: "branch1",
			States: map[string]models.SLState{
				"branch1": models.SLState{
					Type:  models.SLStateTypeTask,
					End:   true,
					Retry: []*models.SLRetrier{},
				},
			},
		},
	}
	err = validateParallelState(state)
	assert.Error(t, err)

	t.Log("No error with correctly formatted Parallel state")
	state.Branches = []*models.SLStateMachine{
		&models.SLStateMachine{
			StartAt: "branch1",
			States: map[string]models.SLState{
				"branch1": models.SLState{
					Type:     models.SLStateTypeTask,
					Resource: "fake-resource",
					End:      true,
					Retry:    []*models.SLRetrier{},
				},
			},
		},
	}
	err = validateParallelState(state)
	assert.Nil(t, err)

}

func TestValidateChoiceState(t *testing.T) {
	state := models.SLState{
		Type: models.SLStateTypeChoice,
	}

	t.Log("Error if branches field is not specified")
	err := validateChoiceState(state)
	assert.Error(t, err)

	t.Log("Error if choice is missing Next field")
	state.Choices = []*models.SLChoice{&models.SLChoice{}}
	err = validateChoiceState(state)
	assert.Error(t, err)

	t.Log("No error with correctly formatted Choice state")
	state.Choices = []*models.SLChoice{&models.SLChoice{Next: "fakeState"}}
	err = validateChoiceState(state)
	assert.Nil(t, err)
}

func TestValidateWaitState(t *testing.T) {
	state := models.SLState{
		Type: models.SLStateTypeWait,
	}

	t.Log("Error if no wait time field specified")
	err := validateWaitState(state)
	assert.Error(t, err)

	t.Log("Error if more than one wait time field specified")
	state = models.SLState{
		Type:          models.SLStateTypeWait,
		Timestamp:     "timestamp",
		TimestampPath: "timestampPath",
	}
	err = validateWaitState(state)
	assert.Error(t, err)

	t.Log("No error with correctly formatted Wait state")
	state = models.SLState{
		Type:      models.SLStateTypeWait,
		Timestamp: "timestamp",
	}
	err = validateWaitState(state)
	assert.Nil(t, err)
}

func TestValidateEndState(t *testing.T) {
	state := models.SLState{
		Type: models.SLStateTypeFail,
		Next: "fakeStat",
	}
	t.Log("Error if End state specifies next state")
	err := validateEndState(state)
	assert.Error(t, err)

	t.Log("No error for correctly formatted End state")
	state = models.SLState{
		Type: models.SLStateTypeSucceed,
	}
	err = validateEndState(state)
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

func TestResumeWorkflowByID(t *testing.T) {
	workflowDefinition := &models.WorkflowDefinition{
		StateMachine: &models.SLStateMachine{
			StartAt: "monkey-state",
			States: map[string]models.SLState{
				"monkey-state": models.SLState{
					Type:     models.SLStateTypeTask,
					Next:     "gorilla-state",
					Resource: "resource-name",
				},
				"gorilla-state": models.SLState{
					Type:     models.SLStateTypeTask,
					Resource: "lambda:resource-name",
					End:      true,
				},
			},
		},
	}

	specs := []struct {
		desc    string
		isError bool
		startAt string
		wf      *models.Workflow
		newWF   *models.Workflow
	}{
		{
			desc: "uses the input from lastJob if StartAt == lastJob.State" +
				" regardless of the existance of the jobs array",
			isError: false,
			startAt: "gorilla-state",
			wf: &models.Workflow{
				WorkflowSummary: models.WorkflowSummary{
					ID:                 "workflow-id",
					Status:             models.WorkflowStatusFailed,
					WorkflowDefinition: workflowDefinition,
					LastJob: &models.Job{
						State:  "gorilla-state",
						Input:  `{"snack":"plum"}`,
						Status: models.JobStatusFailed,
					},
				},
				Jobs: nil,
			},
			newWF: &models.Workflow{
				WorkflowSummary: models.WorkflowSummary{
					ID:                 "new-workflow-id",
					Status:             models.WorkflowStatusQueued,
					WorkflowDefinition: workflowDefinition,
					Input:              `{"snack":"plum"}`,
				},
			},
		},
		{
			desc:    "finds the input from jobs array when StartAt != lastJob.State",
			isError: false,
			startAt: "monkey-state",
			wf: &models.Workflow{
				WorkflowSummary: models.WorkflowSummary{
					ID:                 "workflow-id",
					Status:             models.WorkflowStatusFailed,
					WorkflowDefinition: workflowDefinition,
					LastJob: &models.Job{
						State:  "gorilla-state",
						Input:  `{"snack":"plum"}`,
						Status: models.JobStatusFailed,
					},
				},
				Jobs: []*models.Job{
					{
						State:  "monkey-state",
						Input:  `{"snack":"banana"}`,
						Status: models.JobStatusSucceeded,
					},
					{
						State:  "gorilla-state",
						Input:  `{"snack":"plum"}`,
						Status: models.JobStatusFailed,
					},
				},
			},
			newWF: &models.Workflow{
				WorkflowSummary: models.WorkflowSummary{
					ID:                 "new-workflow-id",
					Status:             models.WorkflowStatusQueued,
					WorkflowDefinition: workflowDefinition,
					Input:              `{"snack":"banana"}`,
				},
			},
		},
		{
			desc:    "fails if there are no previous attempted jobs at StartAt state",
			isError: true,
			startAt: "gorilla-state",
			wf: &models.Workflow{
				WorkflowSummary: models.WorkflowSummary{
					ID:                 "workflow-id",
					Status:             models.WorkflowStatusFailed,
					WorkflowDefinition: workflowDefinition,
					LastJob: &models.Job{
						State:  "monkey-state",
						Input:  `{"snack":"banana"}`,
						Status: models.JobStatusFailed,
					},
				},
				Jobs: []*models.Job{
					{
						State:  "monkey-state",
						Input:  `{"snack":"banana"}`,
						Status: models.JobStatusFailed,
					},
				},
			},
			newWF: &models.Workflow{},
		},
		{
			desc:    "fails if StartAt state doesn't exist in the workflow definition",
			isError: true,
			startAt: "invalid-state",
			wf: &models.Workflow{
				WorkflowSummary: models.WorkflowSummary{
					ID:                 "workflow-id",
					Status:             models.WorkflowStatusFailed,
					WorkflowDefinition: workflowDefinition,
					LastJob: &models.Job{
						State:  "monkey-state",
						Input:  `{"snack":"banana"}`,
						Status: models.JobStatusFailed,
					},
				},
				Jobs: []*models.Job{
					{
						State:  "monkey-state",
						Input:  `{"snack":"banana"}`,
						Status: models.JobStatusFailed,
					},
				},
			},
			newWF: &models.Workflow{},
		},
	}

	for _, spec := range specs {
		t.Run(spec.desc, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			c := newSFNManagerTestController(t)

			c.mockStore.EXPECT().
				GetWorkflowByID(ctx, gomock.Eq(spec.wf.ID)).
				Return(*spec.wf, nil).
				Times(1)

			if !spec.isError {
				c.mockWFM.EXPECT().
					RetryWorkflow(ctx, *spec.wf, spec.startAt, spec.newWF.Input).
					Return(spec.newWF, nil).
					Times(1)
			}

			resumedWorkflow, err := c.handler.ResumeWorkflowByID(
				ctx,
				&models.ResumeWorkflowByIDInput{
					WorkflowID: spec.wf.ID,
					Overrides:  &models.WorkflowDefinitionOverrides{StartAt: spec.startAt},
				},
			)

			if spec.isError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, spec.newWF, resumedWorkflow)
		})
	}

	t.Run("loads the jobs array when it's missing and StartAt != lastJob.State", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		c := newSFNManagerTestController(t)

		startAt := "monkey-state"

		wf := &models.Workflow{
			WorkflowSummary: models.WorkflowSummary{
				ID:                 "workflow-id",
				Status:             models.WorkflowStatusFailed,
				WorkflowDefinition: workflowDefinition,
				LastJob: &models.Job{
					State:  "gorilla-state",
					Input:  `{"snack":"plum"}`,
					Status: models.JobStatusFailed,
				},
			},
			Jobs: nil,
		}

		jobsToLoad := []*models.Job{
			{
				State:  "monkey-state",
				Input:  `{"snack":"banana"}`,
				Status: models.JobStatusSucceeded,
			},
			{
				State:  "gorilla-state",
				Input:  `{"snack":"plum"}`,
				Status: models.JobStatusFailed,
			},
		}

		newWF := &models.Workflow{
			WorkflowSummary: models.WorkflowSummary{
				ID:                 "new-workflow-id",
				Status:             models.WorkflowStatusQueued,
				WorkflowDefinition: workflowDefinition,
				Input:              `{"snack":"banana"}`,
			},
		}

		c.mockStore.EXPECT().
			GetWorkflowByID(ctx, gomock.Eq(wf.ID)).
			Return(*wf, nil).
			Times(1)

		wfWithoutJobs := *wf
		c.mockWFM.EXPECT().
			UpdateWorkflowHistory(ctx, &wfWithoutJobs).
			Return(nil).
			Times(1)

		wf.Jobs = jobsToLoad
		c.mockStore.EXPECT().
			GetWorkflowByID(ctx, gomock.Eq(wf.ID)).
			Return(*wf, nil).
			Times(1)

		c.mockWFM.EXPECT().
			RetryWorkflow(ctx, *wf, startAt, newWF.Input).
			Return(newWF, nil).
			Times(1)

		resumedWorkflow, err := c.handler.ResumeWorkflowByID(
			ctx,
			&models.ResumeWorkflowByIDInput{
				WorkflowID: wf.ID,
				Overrides:  &models.WorkflowDefinitionOverrides{StartAt: startAt},
			},
		)

		require.NoError(t, err)
		assert.Equal(t, newWF, resumedWorkflow)
	})

	t.Run("fails if ResumeWorkflowByID is called on a running workflow", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		c := newSFNManagerTestController(t)

		wf := &models.Workflow{
			WorkflowSummary: models.WorkflowSummary{
				ID:                 "workflow-id",
				Status:             models.WorkflowStatusRunning,
				WorkflowDefinition: workflowDefinition,
			},
			Jobs: []*models.Job{
				{
					State:  "monkey-state",
					Input:  `{"snack":"banana"}`,
					Status: models.JobStatusRunning,
				},
			},
		}

		c.mockStore.EXPECT().
			GetWorkflowByID(ctx, gomock.Eq(wf.ID)).
			Return(*wf, nil).
			Times(1)

		_, err := c.handler.ResumeWorkflowByID(
			ctx,
			&models.ResumeWorkflowByIDInput{
				WorkflowID: wf.ID,
				Overrides:  &models.WorkflowDefinitionOverrides{StartAt: "monkey-state"},
			},
		)
		require.Error(t, err)
	})

	t.Run(
		"fails if ResumeWorkflowByID is called on a workflow with active retries",
		func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			c := newSFNManagerTestController(t)

			wfRetry := &models.Workflow{
				WorkflowSummary: models.WorkflowSummary{
					ID:                 "retry-workflow-id",
					Status:             models.WorkflowStatusRunning,
					WorkflowDefinition: workflowDefinition,
				},
				Jobs: []*models.Job{
					{
						State:  "monkey-state",
						Input:  `{"snack":"banana"}`,
						Status: models.JobStatusRunning,
					},
				},
			}

			wf := &models.Workflow{
				WorkflowSummary: models.WorkflowSummary{
					ID:                 "workflow-id",
					Status:             models.WorkflowStatusFailed,
					WorkflowDefinition: workflowDefinition,
					Retries:            []string{wfRetry.ID},
				},
				Jobs: []*models.Job{
					{
						State:  "monkey-state",
						Input:  `{"snack":"banana"}`,
						Status: models.JobStatusFailed,
					},
				},
			}

			c.mockStore.EXPECT().
				GetWorkflowByID(ctx, gomock.Eq(wf.ID)).
				Return(*wf, nil).
				Times(1)

			c.mockStore.EXPECT().
				GetWorkflowByID(ctx, gomock.Eq(wfRetry.ID)).
				Return(*wfRetry, nil).
				Times(1)

			_, err := c.handler.ResumeWorkflowByID(
				ctx,
				&models.ResumeWorkflowByIDInput{
					WorkflowID: wf.ID,
					Overrides:  &models.WorkflowDefinitionOverrides{StartAt: "monkey-state"},
				},
			)
			require.Error(t, err)
		})
}
