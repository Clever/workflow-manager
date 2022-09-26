package executor

import (
	"context"
	"testing"
	"time"

	"github.com/Clever/workflow-manager/gen-go/models"
	"github.com/Clever/workflow-manager/mocks"
	"github.com/Clever/workflow-manager/resources"
	"github.com/aws/aws-sdk-go/service/sfn"
	"github.com/go-openapi/strfmt"
	"github.com/golang/mock/gomock"
	uuid "github.com/satori/go.uuid"
	"github.com/stretchr/testify/require"
)

type wfmTestController struct {
	manager            *SFNWorkflowManager
	mockController     *gomock.Controller
	mockSFNAPI         *mocks.MockSFNAPI
	store              *mocks.MockStore
	t                  *testing.T
	workflowDefinition *models.WorkflowDefinition
}

func TestUpdateCancelledWorkflowWhereResolvedByUserNotSet(t *testing.T) {
	// checks that cancelled workflow has ResolvedByUser set to true when Updated
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c := newWfmTestController(t)
	defer c.mockController.Finish()

	id := uuid.NewV4().String()
	wf := models.Workflow{
		WorkflowSummary: models.WorkflowSummary{
			ID:          id,
			LastUpdated: strfmt.DateTime(time.Now()),
			Status:      models.WorkflowStatusCancelled,
			WorkflowDefinition: &models.WorkflowDefinition{
				StateMachine: &models.SLStateMachine{
					StartAt: "foo",
				},
			},
		},
		Jobs: []*models.Job{},
	}

	aborted := sfn.ExecutionStatusAborted
	descExecOutput := sfn.DescribeExecutionOutput{
		Status: &aborted,
	}

	c.mockSFNAPI.EXPECT().
		DescribeExecutionWithContext(gomock.Any(), gomock.Any()).
		Return(&descExecOutput, nil)

	c.store.EXPECT().
		UpdateWorkflow(gomock.Any(), resolvedByUserMatcher()).
		Return(nil).
		Times(1)

	require.NoError(t, c.manager.UpdateWorkflowSummary(ctx, &wf))
}

type resolvedByUser struct{}

func resolvedByUserMatcher() gomock.Matcher {
	return &resolvedByUser{}
}

func (m *resolvedByUser) Matches(x interface{}) bool {
	return x.(models.Workflow).ResolvedByUser
}

func (m *resolvedByUser) String() string {
	return " has ResolvedByUser set to false"
}

func newWfmTestController(t *testing.T) *wfmTestController {
	mockController := gomock.NewController(t)
	mockSFNAPI := mocks.NewMockSFNAPI(mockController)
	mockCWLogsAPI := mocks.NewMockCloudWatchLogsAPI(mockController)
	mockStore := mocks.NewMockStore(mockController)

	workflowDefinition := resources.KitchenSinkWorkflowDefinition(t)

	return &wfmTestController{
		manager:            NewSFNWorkflowManager(mockSFNAPI, mockCWLogsAPI, mockStore, "", "", "", "", ""),
		mockController:     mockController,
		mockSFNAPI:         mockSFNAPI,
		store:              mockStore,
		t:                  t,
		workflowDefinition: workflowDefinition,
	}
}
