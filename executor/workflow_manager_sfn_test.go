package executor

import (
	"fmt"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudwatch"
	"github.com/aws/aws-sdk-go/service/sfn"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Clever/workflow-manager/gen-go/models"
	"github.com/Clever/workflow-manager/mocks/mock_cloudwatchiface"
	"github.com/Clever/workflow-manager/mocks/mock_sfniface"
	"github.com/Clever/workflow-manager/resources"
	"github.com/Clever/workflow-manager/store"
	"github.com/Clever/workflow-manager/store/memory"
)

type sfnManagerTestController struct {
	manager            *SFNWorkflowManager
	mockController     *gomock.Controller
	mockSFNAPI         *mock_sfniface.MockSFNAPI
	mockCWAPI          *mock_cloudwatchiface.MockCloudWatchAPI
	store              store.Store
	t                  *testing.T
	workflowDefinition *models.WorkflowDefinition
}

type stateMachineNameInput struct {
	wdName    string
	wdVersion int64
	namespace string
	startAt   string
}

type stateMachineNameTest struct {
	input  stateMachineNameInput
	output string
}

func TestStateMachineName(t *testing.T) {
	tests := []stateMachineNameTest{
		{
			input: stateMachineNameInput{
				wdName:    "cil-reliability-dashboard:sfn",
				wdVersion: 3,
				namespace: "production",
				startAt:   "cil",
			},
			output: "production--cil-reliability-dashboard-sfn--3--cil",
		},
	}
	for _, test := range tests {
		output := stateMachineName(
			test.input.wdName,
			test.input.wdVersion,
			test.input.namespace,
			test.input.startAt,
		)
		require.Equal(t, output, test.output, "input: %#v", test.input)
	}
}

func TestStateMachineWithFullActivityARNs(t *testing.T) {
	sm := models.SLStateMachine{
		States: map[string]models.SLState{
			"foostate": models.SLState{
				Type:     models.SLStateTypeTask,
				Resource: "resource-name",
			},
		},
	}
	smWithFullActivityARNs := stateMachineWithFullActivityARNs(sm, "region", "accountID", "namespace")
	require.Equal(t, map[string]models.SLState{
		"foostate": models.SLState{
			Type:     models.SLStateTypeTask,
			Resource: "arn:aws:states:region:accountID:activity:namespace--resource-name",
		},
	}, smWithFullActivityARNs.States)
}

func TestStateMachineWithDefaultRetriers(t *testing.T) {
	sm := models.SLStateMachine{
		States: map[string]models.SLState{
			"foostate": models.SLState{
				Type: models.SLStateTypeTask,
			},
		},
	}
	smWithRetry := stateMachineWithDefaultRetriers(sm)
	require.Equal(t, map[string]models.SLState{
		"foostate": models.SLState{
			Type:  models.SLStateTypeTask,
			Retry: []*models.SLRetrier{defaultSFNCLICommandTerminatedRetrier},
		},
	}, smWithRetry.States)
}

func TestCancelWorkflow(t *testing.T) {
	c := newSFNManagerTestController(t)
	defer c.tearDown()

	workflow := c.newWorkflow()
	initialStatus := models.WorkflowStatusRunning
	workflow.Status = initialStatus
	c.saveWorkflow(t, workflow)

	t.Log("Verify execution is stopped and status reason is updated.")
	reason := "i have my reasons"
	sfnExecutionARN := c.manager.executionARN(workflow, c.workflowDefinition)
	c.mockSFNAPI.EXPECT().
		StopExecution(&sfn.StopExecutionInput{
			ExecutionArn: aws.String(sfnExecutionARN),
			Cause:        aws.String(reason),
		}).
		Return(&sfn.StopExecutionOutput{}, nil)
	c.mockSFNAPI.EXPECT().
		DescribeExecution(&sfn.DescribeExecutionInput{
			ExecutionArn: aws.String(sfnExecutionARN),
		}).
		Return(&sfn.DescribeExecutionOutput{
			Status: aws.String(sfn.ExecutionStatusAborted),
		}, nil)
	require.NoError(t, c.manager.CancelWorkflow(workflow, reason))
	assert.Equal(t, initialStatus, workflow.Status)
	assert.Equal(t, reason, workflow.StatusReason)

	t.Log("Verify both status and status reason are updated if execution has already failed.")
	newReason := "seriously, stop asking"
	c.mockSFNAPI.EXPECT().
		StopExecution(&sfn.StopExecutionInput{
			ExecutionArn: aws.String(sfnExecutionARN),
			Cause:        aws.String(newReason),
		}).
		Return(&sfn.StopExecutionOutput{}, nil)
	c.mockSFNAPI.EXPECT().
		DescribeExecution(&sfn.DescribeExecutionInput{
			ExecutionArn: aws.String(sfnExecutionARN),
		}).
		Return(&sfn.DescribeExecutionOutput{
			Status: aws.String(sfn.ExecutionStatusFailed),
		}, nil)
	require.NoError(t, c.manager.CancelWorkflow(workflow, newReason))
	assert.Equal(t, models.WorkflowStatusCancelled, workflow.Status)
	assert.Equal(t, newReason, workflow.StatusReason)

	t.Log("Verify errors are propagated.")
	cancelError := fmt.Errorf("nope")
	c.mockSFNAPI.EXPECT().
		StopExecution(gomock.Any()).
		Return(&sfn.StopExecutionOutput{}, cancelError)
	require.Error(t, c.manager.CancelWorkflow(workflow, reason))
}

func TestUpdateWorkflowStatusNoop(t *testing.T) {
	c := newSFNManagerTestController(t)
	defer c.tearDown()

	workflow := c.newWorkflow()
	workflow.Status = models.WorkflowStatusCancelled
	c.saveWorkflow(t, workflow)

	require.NoError(t, c.manager.UpdateWorkflowStatus(workflow))
	assert.Equal(t, models.WorkflowStatusCancelled, workflow.Status)
}

var jobCreatedEventTimestamp = time.Now()
var jobCreatedEvent = &sfn.HistoryEvent{
	Id:        aws.Int64(1),
	Timestamp: aws.Time(jobCreatedEventTimestamp),
	Type:      aws.String(sfn.HistoryEventTypeTaskStateEntered),
	StateEnteredEventDetails: &sfn.StateEnteredEventDetails{
		Name:  aws.String("my-first-state"),
		Input: aws.String(`{foo: "bar"}`),
	},
}

func assertBasicJobData(t *testing.T, job *models.Job) {
	assert.Equal(t, "1", job.ID)
	assert.Equal(t, "my-first-state", job.State)
	assert.Equal(t, `{foo: "bar"}`, job.Input)
	assert.WithinDuration(t, jobCreatedEventTimestamp, time.Time(job.CreatedAt), 1*time.Second)
}

func TestUpdateWorkflowStatusJobCreated(t *testing.T) {
	c := newSFNManagerTestController(t)
	defer c.tearDown()

	workflow := c.newWorkflow()
	workflow.Status = models.WorkflowStatusQueued
	c.saveWorkflow(t, workflow)

	sfnExecutionARN := c.manager.executionARN(workflow, c.workflowDefinition)
	c.mockSFNAPI.EXPECT().
		DescribeExecution(&sfn.DescribeExecutionInput{
			ExecutionArn: aws.String(sfnExecutionARN),
		}).
		Return(&sfn.DescribeExecutionOutput{
			Status: aws.String(sfn.ExecutionStatusRunning),
		}, nil)
	c.mockSFNAPI.EXPECT().
		GetExecutionHistoryPages(&sfn.GetExecutionHistoryInput{
			ExecutionArn: aws.String(sfnExecutionARN),
		}, gomock.Any()).
		Do(func(
			input *sfn.GetExecutionHistoryInput,
			cb func(historyOutput *sfn.GetExecutionHistoryOutput, lastPage bool) bool,
		) {
			cb(&sfn.GetExecutionHistoryOutput{Events: []*sfn.HistoryEvent{jobCreatedEvent}}, true)
		})

	require.NoError(t, c.manager.UpdateWorkflowStatus(workflow))
	assert.Equal(t, models.WorkflowStatusRunning, workflow.Status)
	require.Len(t, workflow.Jobs, 1)
	assertBasicJobData(t, workflow.Jobs[0])
	assert.Equal(t, models.JobStatusCreated, workflow.Jobs[0].Status)
}

var jobFailedEventTimestamp = jobCreatedEventTimestamp.Add(5 * time.Minute)
var jobFailedEvent = &sfn.HistoryEvent{
	Id:        aws.Int64(2),
	Timestamp: aws.Time(jobFailedEventTimestamp),
	Type:      aws.String(sfn.HistoryEventTypeActivityFailed),
	ActivityFailedEventDetails: &sfn.ActivityFailedEventDetails{
		Cause: aws.String("line1\nline2\nline3\nline4\nline5\nline6\n\n"),
	},
}

func TestUpdateWorkflowStatusJobFailed(t *testing.T) {
	c := newSFNManagerTestController(t)
	defer c.tearDown()

	workflow := c.newWorkflow()
	workflow.Status = models.WorkflowStatusQueued
	c.saveWorkflow(t, workflow)

	sfnExecutionARN := c.manager.executionARN(workflow, c.workflowDefinition)
	c.mockSFNAPI.EXPECT().
		DescribeExecution(&sfn.DescribeExecutionInput{
			ExecutionArn: aws.String(sfnExecutionARN),
		}).
		Return(&sfn.DescribeExecutionOutput{
			Status: aws.String(sfn.ExecutionStatusFailed),
		}, nil)

	c.mockSFNAPI.EXPECT().
		GetExecutionHistoryPages(&sfn.GetExecutionHistoryInput{
			ExecutionArn: aws.String(sfnExecutionARN),
		}, gomock.Any()).
		Do(func(
			input *sfn.GetExecutionHistoryInput,
			cb func(historyOutput *sfn.GetExecutionHistoryOutput, lastPage bool) bool,
		) {
			cb(&sfn.GetExecutionHistoryOutput{Events: []*sfn.HistoryEvent{
				jobCreatedEvent,
				jobFailedEvent,
			}}, true)
		})

	require.NoError(t, c.manager.UpdateWorkflowStatus(workflow))
	assert.Equal(t, models.WorkflowStatusFailed, workflow.Status)
	require.Len(t, workflow.Jobs, 1)
	assertBasicJobData(t, workflow.Jobs[0])
	assert.Equal(t, models.JobStatusFailed, workflow.Jobs[0].Status)
	assert.Equal(t, "line4\nline5\nline6", workflow.Jobs[0].StatusReason)
	assert.WithinDuration(
		t, jobFailedEventTimestamp, time.Time(workflow.Jobs[0].StoppedAt), 1*time.Second,
	)
}

func TestUpdateWorkflowStatusJobFailedNotDeployed(t *testing.T) {
	c := newSFNManagerTestController(t)
	defer c.tearDown()

	workflow := c.newWorkflow()
	workflow.Status = models.WorkflowStatusQueued
	c.saveWorkflow(t, workflow)

	sfnExecutionARN := c.manager.executionARN(workflow, c.workflowDefinition)
	c.mockSFNAPI.EXPECT().
		DescribeExecution(&sfn.DescribeExecutionInput{
			ExecutionArn: aws.String(sfnExecutionARN),
		}).
		Return(&sfn.DescribeExecutionOutput{
			Status: aws.String(sfn.ExecutionStatusFailed),
		}, nil)

	c.mockSFNAPI.EXPECT().
		GetExecutionHistoryPages(&sfn.GetExecutionHistoryInput{
			ExecutionArn: aws.String(sfnExecutionARN),
		}, gomock.Any()).
		Do(func(
			input *sfn.GetExecutionHistoryInput,
			cb func(historyOutput *sfn.GetExecutionHistoryOutput, lastPage bool) bool,
		) {
			cb(&sfn.GetExecutionHistoryOutput{Events: []*sfn.HistoryEvent{
				jobCreatedEvent,
				&sfn.HistoryEvent{
					Id:        aws.Int64(2),
					Timestamp: aws.Time(jobFailedEventTimestamp),
					Type:      aws.String(sfn.HistoryEventTypeExecutionFailed),
					ExecutionFailedEventDetails: &sfn.ExecutionFailedEventDetails{
						Cause: aws.String("Internal Error (49b863bd-3367-4035-a76d-bfb2e777ece3)"),
						Error: aws.String("States.Runtime"),
					},
				},
			}}, true)
		})

	require.NoError(t, c.manager.UpdateWorkflowStatus(workflow))
	assert.Equal(t, models.WorkflowStatusFailed, workflow.Status)
	require.Len(t, workflow.Jobs, 1)
	assert.Equal(t, models.JobStatusFailed, workflow.Jobs[0].Status)
	assert.Equal(t, "State resource does not exist", workflow.Jobs[0].StatusReason)
	assert.WithinDuration(
		t, jobFailedEventTimestamp, time.Time(workflow.Jobs[0].StoppedAt), 1*time.Second,
	)
}

var jobSucceededEventTimestamp = jobCreatedEventTimestamp.Add(5 * time.Minute)
var jobSucceededEvent = &sfn.HistoryEvent{
	Id:        aws.Int64(3),
	Timestamp: aws.Time(jobSucceededEventTimestamp),
	Type:      aws.String(sfn.HistoryEventTypeActivitySucceeded),
}
var jobExitedEventTimestamp = jobSucceededEventTimestamp.Add(5 * time.Second)
var jobExitedEvent = &sfn.HistoryEvent{
	Id:        aws.Int64(4),
	Timestamp: aws.Time(jobExitedEventTimestamp),
	Type:      aws.String(sfn.HistoryEventTypeTaskStateExited),
	StateExitedEventDetails: &sfn.StateExitedEventDetails{
		Name:   jobCreatedEvent.StateEnteredEventDetails.Name,
		Output: aws.String(`{out: "put"}`),
	},
}

func assertSucceededJobData(t *testing.T, job *models.Job) {
	assert.Equal(t, models.JobStatusSucceeded, job.Status)
	assert.Equal(t, `{out: "put"}`, job.Output)
	assert.WithinDuration(t, jobExitedEventTimestamp, time.Time(job.StoppedAt), 1*time.Second)
}

func TestUpdateWorkflowStatusWorkflowJobSucceeded(t *testing.T) {
	c := newSFNManagerTestController(t)
	defer c.tearDown()

	workflow := c.newWorkflow()
	workflow.Status = models.WorkflowStatusQueued
	c.saveWorkflow(t, workflow)

	sfnExecutionARN := c.manager.executionARN(workflow, c.workflowDefinition)
	c.mockSFNAPI.EXPECT().
		DescribeExecution(&sfn.DescribeExecutionInput{
			ExecutionArn: aws.String(sfnExecutionARN),
		}).
		Return(&sfn.DescribeExecutionOutput{
			Status: aws.String(sfn.ExecutionStatusSucceeded),
		}, nil)

	c.mockSFNAPI.EXPECT().
		GetExecutionHistoryPages(&sfn.GetExecutionHistoryInput{
			ExecutionArn: aws.String(sfnExecutionARN),
		}, gomock.Any()).
		Do(func(
			input *sfn.GetExecutionHistoryInput,
			cb func(historyOutput *sfn.GetExecutionHistoryOutput, lastPage bool) bool,
		) {
			cb(&sfn.GetExecutionHistoryOutput{Events: []*sfn.HistoryEvent{
				jobCreatedEvent,
				jobSucceededEvent,
				jobExitedEvent,
			}}, true)
		})

	require.NoError(t, c.manager.UpdateWorkflowStatus(workflow))
	assert.Equal(t, models.WorkflowStatusSucceeded, workflow.Status)
	require.Len(t, workflow.Jobs, 1)
	assertBasicJobData(t, workflow.Jobs[0])
	assertSucceededJobData(t, workflow.Jobs[0])
}

var jobAbortedEventTimestamp = jobSucceededEventTimestamp.Add(5 * time.Minute)
var jobAbortedEvent = &sfn.HistoryEvent{
	Id:        aws.Int64(5),
	Timestamp: aws.Time(jobAbortedEventTimestamp),
	Type:      aws.String(sfn.HistoryEventTypeExecutionAborted),
	ExecutionAbortedEventDetails: &sfn.ExecutionAbortedEventDetails{
		Cause: aws.String("sfn abort reason"),
	},
}

func assertCancelledJobData(t *testing.T, job *models.Job) {
	assert.Equal(t, models.JobStatusAbortedByUser, job.Status)
	assert.Equal(t, "sfn abort reason", job.StatusReason)
	assert.WithinDuration(t, jobAbortedEventTimestamp, time.Time(job.StoppedAt), 1*time.Second)
}

func TestUpdateWorkflowStatusJobCancelled(t *testing.T) {
	c := newSFNManagerTestController(t)
	defer c.tearDown()

	workflow := c.newWorkflow()
	workflow.Status = models.WorkflowStatusQueued
	workflow.StatusReason = "cancelled by user"
	c.saveWorkflow(t, workflow)

	sfnExecutionARN := c.manager.executionARN(workflow, c.workflowDefinition)
	c.mockSFNAPI.EXPECT().
		DescribeExecution(&sfn.DescribeExecutionInput{
			ExecutionArn: aws.String(sfnExecutionARN),
		}).
		Return(&sfn.DescribeExecutionOutput{
			Status: aws.String(sfn.ExecutionStatusAborted),
		}, nil)

	c.mockSFNAPI.EXPECT().
		GetExecutionHistoryPages(&sfn.GetExecutionHistoryInput{
			ExecutionArn: aws.String(sfnExecutionARN),
		}, gomock.Any()).
		Do(func(
			input *sfn.GetExecutionHistoryInput,
			cb func(historyOutput *sfn.GetExecutionHistoryOutput, lastPage bool) bool,
		) {
			cb(&sfn.GetExecutionHistoryOutput{Events: []*sfn.HistoryEvent{
				jobCreatedEvent,
				jobAbortedEvent,
			}}, true)
		})

	require.NoError(t, c.manager.UpdateWorkflowStatus(workflow))
	assert.Equal(t, models.WorkflowStatusCancelled, workflow.Status)
	assert.Equal(t, "cancelled by user", workflow.StatusReason)
	require.Len(t, workflow.Jobs, 1)
	assertBasicJobData(t, workflow.Jobs[0])
	assertCancelledJobData(t, workflow.Jobs[0])
}

func TestUpdateWorkflowStatusWorkflowCancelledAfterJobSucceeded(t *testing.T) {
	c := newSFNManagerTestController(t)
	defer c.tearDown()

	workflow := c.newWorkflow()
	workflow.Status = models.WorkflowStatusQueued
	workflow.StatusReason = "cancelled by user"
	c.saveWorkflow(t, workflow)

	sfnExecutionARN := c.manager.executionARN(workflow, c.workflowDefinition)
	c.mockSFNAPI.EXPECT().
		DescribeExecution(&sfn.DescribeExecutionInput{
			ExecutionArn: aws.String(sfnExecutionARN),
		}).
		Return(&sfn.DescribeExecutionOutput{
			Status: aws.String(sfn.ExecutionStatusAborted),
		}, nil)

	c.mockSFNAPI.EXPECT().
		GetExecutionHistoryPages(&sfn.GetExecutionHistoryInput{
			ExecutionArn: aws.String(sfnExecutionARN),
		}, gomock.Any()).
		Do(func(
			input *sfn.GetExecutionHistoryInput,
			cb func(historyOutput *sfn.GetExecutionHistoryOutput, lastPage bool) bool,
		) {
			cb(&sfn.GetExecutionHistoryOutput{Events: []*sfn.HistoryEvent{
				jobCreatedEvent,
				jobSucceededEvent,
				jobAbortedEvent,
			}}, true)
		})

	require.NoError(t, c.manager.UpdateWorkflowStatus(workflow))
	assert.Equal(t, models.WorkflowStatusCancelled, workflow.Status)
	assert.Equal(t, "cancelled by user", workflow.StatusReason)
	require.Len(t, workflow.Jobs, 1)
	assertBasicJobData(t, workflow.Jobs[0])
	assertCancelledJobData(t, workflow.Jobs[0])
}

func TestStateResourcesStatus(t *testing.T) {
	c := newSFNManagerTestController(t)
	defer c.tearDown()

	c.mockCWAPI.EXPECT().
		GetMetricStatistics(gomock.Any()).
		Return(&cloudwatch.GetMetricStatisticsOutput{
			Datapoints: []*cloudwatch.Datapoint{&cloudwatch.Datapoint{
				Sum:       aws.Float64(2),
				Timestamp: aws.Time(time.Now()),
			}},
		}, nil).
		Times(len(c.workflowDefinition.StateMachine.States) * 2)

	stateResources, err := c.manager.StateResourcesStatus(*c.workflowDefinition, "namespace")
	assert.Nil(t, err)
	assert.Equal(t, len(c.workflowDefinition.StateMachine.States), len(stateResources))

	t.Log("Queued and Running metrics are computed for all resources")
	for _, stateResource := range stateResources {
		assert.WithinDuration(t, time.Now(), time.Time(stateResource.Status.LastUpdated), 2*time.Second)
		assert.Equal(t, stateResource.Status.Queued, int64(0))
		assert.Equal(t, stateResource.Status.Running, int64(2))
		assert.Equal(t, stateResource.Namespace, "namespace")
		assert.Contains(t, stateResource.Name, "fake-resource")
		assert.Contains(t, stateResource.URI, ":activity:namespace--fake-resource")
	}

	t.Log("getStatus fetches Cloudwatch metrics for a StateResource")

	c.mockCWAPI.EXPECT().
		GetMetricStatistics(gomock.Any()).
		Do(func(input *cloudwatch.GetMetricStatisticsInput) {
			assert.Contains(t, []string{"ActivitiesScheduled", "ActivitiesStarted"},
				*input.MetricName)
		}).
		Return(&cloudwatch.GetMetricStatisticsOutput{
			Datapoints: []*cloudwatch.Datapoint{&cloudwatch.Datapoint{
				Sum:       aws.Float64(2),
				Timestamp: aws.Time(time.Now()),
			}},
		}, nil).Times(2)
	c.manager.getStatus("aws:..:states:namespace--fake-resource-1")
}

func newSFNManagerTestController(t *testing.T) *sfnManagerTestController {
	mockController := gomock.NewController(t)
	mockSFNAPI := mock_sfniface.NewMockSFNAPI(mockController)
	mockCWAPI := mock_cloudwatchiface.NewMockCloudWatchAPI(mockController)
	store := memory.New()

	workflowDefinition := resources.KitchenSinkWorkflowDefinition(t)
	require.NoError(t, store.SaveWorkflowDefinition(*workflowDefinition))

	return &sfnManagerTestController{
		manager:            NewSFNWorkflowManager(mockSFNAPI, mockCWAPI, store, "", "", ""),
		mockController:     mockController,
		mockSFNAPI:         mockSFNAPI,
		mockCWAPI:          mockCWAPI,
		store:              &store,
		t:                  t,
		workflowDefinition: workflowDefinition,
	}
}

func (c *sfnManagerTestController) newWorkflow() *models.Workflow {
	return resources.NewWorkflow(
		c.workflowDefinition, `["input"]`, "namespace", "queue", map[string]interface{}{},
	)
}

func (c *sfnManagerTestController) saveWorkflow(t *testing.T, workflow *models.Workflow) {
	require.NoError(t, c.store.SaveWorkflow(*workflow))
}

func (c *sfnManagerTestController) tearDown() {
	c.mockController.Finish()
}
