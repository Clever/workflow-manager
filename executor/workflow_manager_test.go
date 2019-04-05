package executor

import (
	"context"
	"errors"
	"github.com/Clever/workflow-manager/gen-go/models"
	"github.com/Clever/workflow-manager/mocks"
	"github.com/Clever/workflow-manager/resources"
	"github.com/aws/aws-sdk-go/service/sfn"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/go-openapi/strfmt"
	"github.com/golang/mock/gomock"
	uuid "github.com/satori/go.uuid"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

type wfmTestController struct {
	manager            *SFNWorkflowManager
	mockController     *gomock.Controller
	mockSFNAPI         *mocks.MockSFNAPI
	mockSQSAPI         *mocks.MockSQSAPI
	store              *mocks.MockStore
	t                  *testing.T
	workflowDefinition *models.WorkflowDefinition
}

func TestUpdatePendingWorkflowStoreWorkflowSucceeds(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c := newWfmTestController(t)
	defer c.mockController.Finish()

	id := uuid.NewV4().String()
	wf := models.Workflow{
		WorkflowSummary: models.WorkflowSummary{
			ID:          id,
			LastUpdated: strfmt.DateTime(time.Now()),
			Status:      models.WorkflowStatusRunning,
			WorkflowDefinition: &models.WorkflowDefinition{
				StateMachine: &models.SLStateMachine{
					StartAt: "foo",
				},
			},
		},
		Jobs: []*models.Job{},
	}

	c.store.EXPECT().
		GetWorkflowByID(gomock.Any(), gomock.Eq(id)).
		Return(wf, nil)

	succeeded := string(sfn.ExecutionStatusSucceeded)
	descExecOutput := sfn.DescribeExecutionOutput{
		Status: &succeeded,
	}

	c.mockSFNAPI.EXPECT().
		DescribeExecutionWithContext(gomock.Any(), gomock.Any()).
		Return(&descExecOutput, nil)

	c.store.EXPECT().
		UpdateWorkflow(gomock.Any(), gomock.Any()).
		Return(nil).
		Times(1)

	c.mockSQSAPI.EXPECT().
		DeleteMessageWithContext(gomock.Any(), gomock.Any()).
		Return(nil, nil).
		Times(1)

	// If store succeeds and job finished, should not requeue
	c.mockSQSAPI.EXPECT().
		SendMessageWithContext(gomock.Any(), gomock.Any()).
		Return(nil, nil).
		Times(0)

	wfID, err := updatePendingWorkflow(ctx, &sqs.Message{Body: &id}, c.manager, c.store, c.mockSQSAPI, "urlQueue")
	require.NoError(t, err)
	require.Equal(t, id, wfID)
}

func TestUpdatePendingWorkflowStoreWorkflowStillRunning(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c := newWfmTestController(t)
	defer c.mockController.Finish()

	id := uuid.NewV4().String()
	wf := models.Workflow{
		WorkflowSummary: models.WorkflowSummary{
			ID:          id,
			LastUpdated: strfmt.DateTime(time.Now()),
			Status:      models.WorkflowStatusRunning,
			WorkflowDefinition: &models.WorkflowDefinition{
				StateMachine: &models.SLStateMachine{
					StartAt: "foo",
				},
			},
		},
		Jobs: []*models.Job{},
	}

	c.store.EXPECT().
		GetWorkflowByID(gomock.Any(), gomock.Eq(id)).
		Return(wf, nil)

	running := string(sfn.ExecutionStatusRunning)
	descExecOutput := sfn.DescribeExecutionOutput{
		Status: &running,
	}

	c.mockSFNAPI.EXPECT().
		DescribeExecutionWithContext(gomock.Any(), gomock.Any()).
		Return(&descExecOutput, nil)

	c.store.EXPECT().
		UpdateWorkflow(gomock.Any(), gomock.Any()).
		Return(nil).
		Times(1)

	c.mockSQSAPI.EXPECT().
		DeleteMessageWithContext(gomock.Any(), gomock.Any()).
		Return(nil, nil).
		Times(1)

	// If store succeeds, but job is still running, should requeue
	c.mockSQSAPI.EXPECT().
		SendMessageWithContext(gomock.Any(), gomock.Any()).
		Return(nil, nil).
		Times(1)

	wfID, err := updatePendingWorkflow(ctx, &sqs.Message{Body: &id}, c.manager, c.store, c.mockSQSAPI, "urlQueue")
	require.NoError(t, err)
	require.Equal(t, id, wfID)
}

func TestUpdatePendingWorkflowStoreWorkflowFails(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c := newWfmTestController(t)
	defer c.mockController.Finish()

	id := uuid.NewV4().String()
	wf := models.Workflow{
		WorkflowSummary: models.WorkflowSummary{
			ID:          id,
			LastUpdated: strfmt.DateTime(time.Now()),
			Status:      models.WorkflowStatusRunning,
			WorkflowDefinition: &models.WorkflowDefinition{
				StateMachine: &models.SLStateMachine{
					StartAt: "foo",
				},
			},
		},
		Jobs: []*models.Job{},
	}

	c.store.EXPECT().
		GetWorkflowByID(gomock.Any(), gomock.Eq(id)).
		Return(wf, nil)

	succeeded := string(sfn.ExecutionStatusSucceeded)
	descExecOutput := sfn.DescribeExecutionOutput{
		Status: &succeeded,
	}

	c.mockSFNAPI.EXPECT().
		DescribeExecutionWithContext(gomock.Any(), gomock.Any()).
		Return(&descExecOutput, nil)

	c.store.EXPECT().
		UpdateWorkflow(gomock.Any(), gomock.Any()).
		Return(errors.New("planned failure")).
		Times(1)

	c.mockSQSAPI.EXPECT().
		DeleteMessageWithContext(gomock.Any(), gomock.Any()).
		Return(nil, nil).
		Times(1)

	// If store fails, should requeue 1 time
	c.mockSQSAPI.EXPECT().
		SendMessageWithContext(gomock.Any(), gomock.Any()).
		Return(nil, nil).
		Times(1)

	wfID, err := updatePendingWorkflow(ctx, &sqs.Message{Body: &id}, c.manager, c.store, c.mockSQSAPI, "urlQueue")
	require.Error(t, err)
	require.Equal(t, "", wfID)
}

func newWfmTestController(t *testing.T) *wfmTestController {
	mockController := gomock.NewController(t)
	mockSFNAPI := mocks.NewMockSFNAPI(mockController)
	mockSQSAPI := mocks.NewMockSQSAPI(mockController)
	mockStore := mocks.NewMockStore(mockController)

	workflowDefinition := resources.KitchenSinkWorkflowDefinition(t)

	return &wfmTestController{
		manager:            NewSFNWorkflowManager(mockSFNAPI, mockSQSAPI, mockStore, "", "", "", ""),
		mockController:     mockController,
		mockSFNAPI:         mockSFNAPI,
		mockSQSAPI:         mockSQSAPI,
		store:              mockStore,
		t:                  t,
		workflowDefinition: workflowDefinition,
	}
}
