package executor

import (
	"errors"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/sfn"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/go-openapi/swag"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Clever/workflow-manager/gen-go/models"
	"github.com/Clever/workflow-manager/mocks"
	"github.com/Clever/workflow-manager/resources"
	"github.com/Clever/workflow-manager/store"
	"github.com/Clever/workflow-manager/store/memory"
)

type sfnManagerTestController struct {
	manager            *SFNWorkflowManager
	mockController     *gomock.Controller
	mockSFNAPI         *mocks.MockSFNAPI
	mockSQSAPI         *mocks.MockSQSAPI
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
			"foostatelambda": models.SLState{
				Type:     models.SLStateTypeTask,
				Resource: "lambda:resource-name",
			},
		},
	}
	smWithFullActivityARNs := stateMachineWithFullActivityARNs(sm, "region", "accountID", "namespace")
	require.Equal(t, map[string]models.SLState{
		"foostate": models.SLState{
			Type:     models.SLStateTypeTask,
			Resource: "arn:aws:states:region:accountID:activity:namespace--resource-name",
		},
		"foostatelambda": models.SLState{
			Type:     models.SLStateTypeTask,
			Resource: "arn:aws:lambda:region:accountID:function:namespace--resource-name",
		},
	}, smWithFullActivityARNs.States)
}

func TestStateMachineWithDefaultRetriers(t *testing.T) {
	t.Log("Default Retry is prepended to State.Retry")
	userRetry := &models.SLRetrier{
		MaxAttempts: swag.Int64(1),
		ErrorEquals: []models.SLErrorEquals{"States.ALL"},
	}
	sm := models.SLStateMachine{
		States: map[string]models.SLState{
			"foostate": models.SLState{
				Type:  models.SLStateTypeTask,
				Retry: []*models.SLRetrier{userRetry},
			},
		},
	}
	smWithRetry := stateMachineWithDefaultRetriers(sm)
	require.Equal(t, map[string]models.SLState{
		"foostate": models.SLState{
			Type:  models.SLStateTypeTask,
			Retry: []*models.SLRetrier{defaultSFNCLICommandTerminatedRetrier, userRetry},
		},
	}, smWithRetry.States)

	t.Log("Ignore Default retry if custom sfncli.CommandTerminated is set")
	customRetry := &models.SLRetrier{
		MaxAttempts: swag.Int64(2),
		ErrorEquals: []models.SLErrorEquals{sfncliCommandTerminated},
	}
	sm = models.SLStateMachine{
		States: map[string]models.SLState{
			"foostate": models.SLState{
				Type:  models.SLStateTypeTask,
				Retry: []*models.SLRetrier{customRetry},
			},
		},
	}
	smWithRetry = stateMachineWithDefaultRetriers(sm)
	require.Equal(t, map[string]models.SLState{
		"foostate": models.SLState{
			Type:  models.SLStateTypeTask,
			Retry: []*models.SLRetrier{customRetry},
		},
	}, smWithRetry.States)
}

func TestCreateWorkflow(t *testing.T) {
	input := "{\"json\": true}"

	t.Run("CreateWorkflow for existing StateMachines", func(t *testing.T) {
		c := newSFNManagerTestController(t)
		defer c.tearDown()
		stateMachineArn := stateMachineARN(c.manager.region, c.manager.accountID,
			c.workflowDefinition.Name,
			c.workflowDefinition.Version,
			"namespace",
			c.workflowDefinition.StateMachine.StartAt,
		)
		c.mockSFNAPI.EXPECT().
			DescribeStateMachine(&sfn.DescribeStateMachineInput{
				StateMachineArn: aws.String(stateMachineArn),
			}).
			Return(&sfn.DescribeStateMachineOutput{
				StateMachineArn: aws.String(stateMachineArn),
			}, nil)
		c.mockSFNAPI.EXPECT().
			StartExecution(gomock.Any()).
			Return(&sfn.StartExecutionOutput{}, nil)
		c.mockSQSAPI.EXPECT().
			SendMessage(gomock.Any()).
			Return(&sqs.SendMessageOutput{}, nil)

		workflow, err := c.manager.CreateWorkflow(*c.workflowDefinition,
			input,
			"namespace",
			"queue",
			map[string]interface{}{},
		)
		assert.Nil(t, err)
		assert.NotNil(t, workflow)
		assert.Equal(t, workflow.Namespace, "namespace")
		assert.Equal(t, workflow.Input, input)

		savedWorkflow, err := c.store.GetWorkflowByID(workflow.ID)
		assert.Nil(t, err)
		assert.Equal(t, workflow.CreatedAt.String(), savedWorkflow.CreatedAt.String())
		assert.Equal(t, workflow.ID, savedWorkflow.ID)
		pendingIDs, err := c.store.GetPendingWorkflowIDs()
		assert.Nil(t, err)
		assert.Equal(t, 1, len(pendingIDs))
		assert.Equal(t, workflow.ID, pendingIDs[0]) // current workflow is pending
	})

	t.Run("CreateWorkflow deletes workflow on StartExecution failure", func(t *testing.T) {
		c := newSFNManagerTestController(t)
		defer c.tearDown()
		stateMachineArn := stateMachineARN(c.manager.region, c.manager.accountID,
			c.workflowDefinition.Name,
			c.workflowDefinition.Version,
			"namespace",
			c.workflowDefinition.StateMachine.StartAt,
		)
		awsError := awserr.New("test", "test", errors.New(""))
		c.mockSFNAPI.EXPECT().
			DescribeStateMachine(&sfn.DescribeStateMachineInput{
				StateMachineArn: aws.String(stateMachineArn),
			}).
			Return(&sfn.DescribeStateMachineOutput{
				StateMachineArn: aws.String(stateMachineArn),
			}, nil)
		c.mockSFNAPI.EXPECT().
			StartExecution(gomock.Any()).
			Return(nil, awsError)

		workflow, err := c.manager.CreateWorkflow(*c.workflowDefinition,
			input,
			"namespace",
			"queue",
			map[string]interface{}{},
		)
		assert.NotNil(t, err)
		assert.Nil(t, workflow)
		assert.IsType(t, awsError, err)
		assert.Equal(t, "test", err.(awserr.Error).Code()) // ensure this error came from sfn api

		pendingIDs, err := c.store.GetPendingWorkflowIDs()
		assert.Equal(t, 0, len(pendingIDs)) // new workflow not created

	})
}

func TestCancelWorkflow(t *testing.T) {
	c := newSFNManagerTestController(t)
	defer c.tearDown()

	workflow := c.newWorkflow()
	initialStatus := models.WorkflowStatusRunning
	workflow.Status = initialStatus
	c.saveWorkflow(t, workflow)
	assert.Equal(t, false, workflow.ResolvedByUser)

	t.Log("Verify execution is stopped and status reason is updated.")
	reason := "i have my reasons"
	sfnExecutionARN := c.manager.executionARN(workflow, c.workflowDefinition)
	c.mockSFNAPI.EXPECT().
		StopExecution(&sfn.StopExecutionInput{
			ExecutionArn: aws.String(sfnExecutionARN),
			Cause:        aws.String(reason),
		}).
		Return(&sfn.StopExecutionOutput{}, nil)
	require.NoError(t, c.manager.CancelWorkflow(workflow, reason))
	assert.Equal(t, initialStatus, workflow.Status)
	assert.Equal(t, true, workflow.ResolvedByUser)
	assert.Equal(t, reason, workflow.StatusReason)

	t.Log("Failed Workflows cannot be cancelled.")
	workflow.Status = models.WorkflowStatusFailed
	c.updateWorkflow(t, workflow)
	require.Error(t, c.manager.CancelWorkflow(workflow, reason))

	t.Log("Successful Workflows cannot be cancelled.")
	workflow.Status = models.WorkflowStatusSucceeded
	c.updateWorkflow(t, workflow)
	require.Error(t, c.manager.CancelWorkflow(workflow, reason))
}

func TestUpdateWorkflowStatusNoop(t *testing.T) {
	c := newSFNManagerTestController(t)
	defer c.tearDown()

	workflow := c.newWorkflow()
	workflow.Status = models.WorkflowStatusCancelled
	c.saveWorkflow(t, workflow)

	require.NoError(t, c.manager.UpdateWorkflowSummary(workflow))
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
		DescribeExecutionWithContext(gomock.Any(), &sfn.DescribeExecutionInput{
			ExecutionArn: aws.String(sfnExecutionARN),
		}).
		Return(&sfn.DescribeExecutionOutput{
			Status: aws.String(sfn.ExecutionStatusRunning),
		}, nil)
	c.mockSFNAPI.EXPECT().
		GetExecutionHistoryPagesWithContext(gomock.Any(), &sfn.GetExecutionHistoryInput{
			ExecutionArn: aws.String(sfnExecutionARN),
		}, gomock.Any()).
		Do(func(
			ctx aws.Context,
			input *sfn.GetExecutionHistoryInput,
			cb func(historyOutput *sfn.GetExecutionHistoryOutput, lastPage bool) bool,
		) {
			cb(&sfn.GetExecutionHistoryOutput{Events: []*sfn.HistoryEvent{jobCreatedEvent}}, true)
		})

	require.NoError(t, c.manager.UpdateWorkflowSummary(workflow))
	require.NoError(t, c.manager.UpdateWorkflowHistory(workflow))
	assert.Equal(t, models.WorkflowStatusRunning, workflow.Status)
	require.Len(t, workflow.Jobs, 1)
	assertBasicJobData(t, workflow.Jobs[0])
	assert.Equal(t, models.JobStatusCreated, workflow.Jobs[0].Status)
}

var jobFailedEventTimestamp = jobCreatedEventTimestamp.Add(5 * time.Minute)
var jobFailedEvent = &sfn.HistoryEvent{
	Id:              aws.Int64(2),
	PreviousEventId: aws.Int64(1),
	Timestamp:       aws.Time(jobFailedEventTimestamp),
	Type:            aws.String(sfn.HistoryEventTypeActivityFailed),
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
		DescribeExecutionWithContext(gomock.Any(), &sfn.DescribeExecutionInput{
			ExecutionArn: aws.String(sfnExecutionARN),
		}).
		Return(&sfn.DescribeExecutionOutput{
			Status: aws.String(sfn.ExecutionStatusFailed),
		}, nil)

	c.mockSFNAPI.EXPECT().
		GetExecutionHistoryPagesWithContext(gomock.Any(), &sfn.GetExecutionHistoryInput{
			ExecutionArn: aws.String(sfnExecutionARN),
		}, gomock.Any()).
		Do(func(
			ctx aws.Context,
			input *sfn.GetExecutionHistoryInput,
			cb func(historyOutput *sfn.GetExecutionHistoryOutput, lastPage bool) bool,
		) {
			cb(&sfn.GetExecutionHistoryOutput{Events: []*sfn.HistoryEvent{
				jobCreatedEvent,
				jobFailedEvent,
			}}, true)
		})

	require.NoError(t, c.manager.UpdateWorkflowSummary(workflow))
	require.NoError(t, c.manager.UpdateWorkflowHistory(workflow))
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
		DescribeExecutionWithContext(gomock.Any(), &sfn.DescribeExecutionInput{
			ExecutionArn: aws.String(sfnExecutionARN),
		}).
		Return(&sfn.DescribeExecutionOutput{
			Status: aws.String(sfn.ExecutionStatusFailed),
		}, nil)

	c.mockSFNAPI.EXPECT().
		GetExecutionHistoryPagesWithContext(gomock.Any(), &sfn.GetExecutionHistoryInput{
			ExecutionArn: aws.String(sfnExecutionARN),
		}, gomock.Any()).
		Do(func(
			ctx aws.Context,
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

	require.NoError(t, c.manager.UpdateWorkflowSummary(workflow))
	require.NoError(t, c.manager.UpdateWorkflowHistory(workflow))
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
	Id:              aws.Int64(2),
	PreviousEventId: aws.Int64(1),
	Timestamp:       aws.Time(jobSucceededEventTimestamp),
	Type:            aws.String(sfn.HistoryEventTypeActivitySucceeded),
}
var jobExitedEventTimestamp = jobSucceededEventTimestamp.Add(5 * time.Second)
var jobExitedEvent = &sfn.HistoryEvent{
	Id:              aws.Int64(3),
	PreviousEventId: aws.Int64(2),
	Timestamp:       aws.Time(jobExitedEventTimestamp),
	Type:            aws.String(sfn.HistoryEventTypeTaskStateExited),
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

	executionOutput := `{"output": true}`
	sfnExecutionARN := c.manager.executionARN(workflow, c.workflowDefinition)
	c.mockSFNAPI.EXPECT().
		DescribeExecutionWithContext(gomock.Any(), &sfn.DescribeExecutionInput{
			ExecutionArn: aws.String(sfnExecutionARN),
		}).
		Return(&sfn.DescribeExecutionOutput{
			Status: aws.String(sfn.ExecutionStatusSucceeded),
			Output: aws.String(executionOutput),
		}, nil)

	c.mockSFNAPI.EXPECT().
		GetExecutionHistoryPagesWithContext(gomock.Any(), &sfn.GetExecutionHistoryInput{
			ExecutionArn: aws.String(sfnExecutionARN),
		}, gomock.Any()).
		Do(func(
			ctx aws.Context,
			input *sfn.GetExecutionHistoryInput,
			cb func(historyOutput *sfn.GetExecutionHistoryOutput, lastPage bool) bool,
		) {
			cb(&sfn.GetExecutionHistoryOutput{Events: []*sfn.HistoryEvent{
				jobCreatedEvent,
				jobSucceededEvent,
				jobExitedEvent,
			}}, true)
		})

	require.NoError(t, c.manager.UpdateWorkflowSummary(workflow))
	require.NoError(t, c.manager.UpdateWorkflowHistory(workflow))
	assert.Equal(t, models.WorkflowStatusSucceeded, workflow.Status)
	assert.Equal(t, executionOutput, workflow.Output)
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
		DescribeExecutionWithContext(gomock.Any(), &sfn.DescribeExecutionInput{
			ExecutionArn: aws.String(sfnExecutionARN),
		}).
		Return(&sfn.DescribeExecutionOutput{
			Status: aws.String(sfn.ExecutionStatusAborted),
		}, nil)

	c.mockSFNAPI.EXPECT().
		GetExecutionHistoryPagesWithContext(gomock.Any(), &sfn.GetExecutionHistoryInput{
			ExecutionArn: aws.String(sfnExecutionARN),
		}, gomock.Any()).
		Do(func(
			ctx aws.Context,
			input *sfn.GetExecutionHistoryInput,
			cb func(historyOutput *sfn.GetExecutionHistoryOutput, lastPage bool) bool,
		) {
			cb(&sfn.GetExecutionHistoryOutput{Events: []*sfn.HistoryEvent{
				jobCreatedEvent,
				jobAbortedEvent,
			}}, true)
		})

	require.NoError(t, c.manager.UpdateWorkflowSummary(workflow))
	require.NoError(t, c.manager.UpdateWorkflowHistory(workflow))
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
		DescribeExecutionWithContext(gomock.Any(), &sfn.DescribeExecutionInput{
			ExecutionArn: aws.String(sfnExecutionARN),
		}).
		Return(&sfn.DescribeExecutionOutput{
			Status: aws.String(sfn.ExecutionStatusAborted),
		}, nil)

	c.mockSFNAPI.EXPECT().
		GetExecutionHistoryPagesWithContext(gomock.Any(), &sfn.GetExecutionHistoryInput{
			ExecutionArn: aws.String(sfnExecutionARN),
		}, gomock.Any()).
		Do(func(
			ctx aws.Context,
			input *sfn.GetExecutionHistoryInput,
			cb func(historyOutput *sfn.GetExecutionHistoryOutput, lastPage bool) bool,
		) {
			cb(&sfn.GetExecutionHistoryOutput{Events: []*sfn.HistoryEvent{
				jobCreatedEvent,
				jobSucceededEvent,
				jobAbortedEvent,
			}}, true)
		})

	require.NoError(t, c.manager.UpdateWorkflowSummary(workflow))
	require.NoError(t, c.manager.UpdateWorkflowHistory(workflow))
	assert.Equal(t, models.WorkflowStatusCancelled, workflow.Status)
	assert.Equal(t, "cancelled by user", workflow.StatusReason)
	require.Len(t, workflow.Jobs, 1)
	assertBasicJobData(t, workflow.Jobs[0])
	assertCancelledJobData(t, workflow.Jobs[0])
}

func TestUpdateWorkflowStatusExecutionNotFoundRetry(t *testing.T) {

	c := newSFNManagerTestController(t)
	defer c.tearDown()

	workflow := c.newWorkflow()
	workflow.Status = models.WorkflowStatusQueued
	c.saveWorkflow(t, workflow)

	executionOutput := `{"output": true}`
	sfnExecutionARN := c.manager.executionARN(workflow, c.workflowDefinition)
	// fail the first time
	c.mockSFNAPI.EXPECT().
		DescribeExecutionWithContext(gomock.Any(), &sfn.DescribeExecutionInput{
			ExecutionArn: aws.String(sfnExecutionARN),
		}).
		Return(nil, awserr.New(sfn.ErrCodeExecutionDoesNotExist, "test", errors.New("")))
	// then success
	c.mockSFNAPI.EXPECT().
		DescribeExecutionWithContext(gomock.Any(), &sfn.DescribeExecutionInput{
			ExecutionArn: aws.String(sfnExecutionARN),
		}).
		Return(&sfn.DescribeExecutionOutput{
			Status: aws.String(sfn.ExecutionStatusSucceeded),
			Output: aws.String(executionOutput),
		}, nil)

	c.mockSFNAPI.EXPECT().
		GetExecutionHistoryPagesWithContext(gomock.Any(), &sfn.GetExecutionHistoryInput{
			ExecutionArn: aws.String(sfnExecutionARN),
		}, gomock.Any()).
		Do(func(
			ctx aws.Context,
			input *sfn.GetExecutionHistoryInput,
			cb func(historyOutput *sfn.GetExecutionHistoryOutput, lastPage bool) bool,
		) {
			cb(&sfn.GetExecutionHistoryOutput{Events: []*sfn.HistoryEvent{
				jobCreatedEvent,
				jobSucceededEvent,
				jobExitedEvent,
			}}, true)
		})

	require.NoError(t, c.manager.UpdateWorkflowSummary(workflow))
	assert.Equal(t, models.WorkflowStatusQueued, workflow.Status)
	require.Len(t, workflow.Jobs, 0)

	require.NoError(t, c.manager.UpdateWorkflowSummary(workflow))
	assert.Equal(t, models.WorkflowStatusSucceeded, workflow.Status)
	require.NoError(t, c.manager.UpdateWorkflowHistory(workflow))
	require.Len(t, workflow.Jobs, 1)
	assertBasicJobData(t, workflow.Jobs[0])
	assertSucceededJobData(t, workflow.Jobs[0])
}

func TestUpdateWorkflowStatusExecutionNotFoundStopRetry(t *testing.T) {
	durationToRetryDescribeExecutions = 1 * time.Second
	c := newSFNManagerTestController(t)
	defer c.tearDown()

	workflow := c.newWorkflow()
	workflow.Status = models.WorkflowStatusQueued
	c.saveWorkflow(t, workflow)

	sfnExecutionARN := c.manager.executionARN(workflow, c.workflowDefinition)
	c.mockSFNAPI.EXPECT().
		DescribeExecutionWithContext(gomock.Any(), &sfn.DescribeExecutionInput{
			ExecutionArn: aws.String(sfnExecutionARN),
		}).
		Return(nil, awserr.New(sfn.ErrCodeExecutionDoesNotExist,
			"test",
			errors.New("")),
		).
		Times(2)

	require.NoError(t, c.manager.UpdateWorkflowSummary(workflow))
	assert.Equal(t, models.WorkflowStatusQueued, workflow.Status)
	require.Len(t, workflow.Jobs, 0)
	time.Sleep(durationToRetryDescribeExecutions)

	require.NoError(t, c.manager.UpdateWorkflowSummary(workflow))
	assert.Equal(t, models.WorkflowStatusFailed, workflow.Status)
	require.Len(t, workflow.Jobs, 0)
}

var jobScheduledEventTimestamp = jobCreatedEventTimestamp.Add(1 * time.Minute)
var jobScheduledEvent = &sfn.HistoryEvent{
	Id:              aws.Int64(6),
	PreviousEventId: aws.Int64(1), // job created
	Timestamp:       aws.Time(jobScheduledEventTimestamp),
	Type:            aws.String(sfn.HistoryEventTypeActivityScheduled),
}
var jobStartedEventTimestamp = jobScheduledEventTimestamp.Add(1 * time.Minute)
var jobStartedEvent = &sfn.HistoryEvent{
	Id:              aws.Int64(7),
	PreviousEventId: aws.Int64(6),
	Timestamp:       aws.Time(jobStartedEventTimestamp),
	Type:            aws.String(sfn.HistoryEventTypeActivityStarted),
}
var jobTimedOutEventTimestamp = jobStartedEventTimestamp.Add(5 * time.Minute)
var jobTimedOutEvent = &sfn.HistoryEvent{
	Id:              aws.Int64(8),
	PreviousEventId: aws.Int64(7),
	Timestamp:       aws.Time(jobTimedOutEventTimestamp),
	Type:            aws.String(sfn.HistoryEventTypeActivityTimedOut),
	ActivityTimedOutEventDetails: &sfn.ActivityTimedOutEventDetails{
		Error: aws.String("States.Timeout"), // this string will actually be States.Timeout
		Cause: aws.String("(sfn cause for activity that timed out)"),
	},
}
var jobTimedOutWorkflowFailedEventTimestamp = jobTimedOutEventTimestamp
var jobTimedOutWorkflowFailedEvent = &sfn.HistoryEvent{
	Id:              aws.Int64(9),
	PreviousEventId: aws.Int64(8),
	Timestamp:       aws.Time(jobTimedOutWorkflowFailedEventTimestamp),
	Type:            aws.String(sfn.HistoryEventTypeExecutionFailed),
	ExecutionFailedEventDetails: &sfn.ExecutionFailedEventDetails{
		Error: aws.String("States.Timeout"),
	},
}

func assertTimedOutJobData(t *testing.T, job *models.Job) {
	assert.Equal(t, models.JobStatusFailed, job.Status)
	// not currently using the details in TestUpdateWorkflowStatusJobTimedOut test
	// also this format does not match the output in workflow_manager_sfn
	assert.Equal(t, "Job timed out", resources.StatusReasonJobTimedOut)
	assert.Contains(t, job.StatusReason, resources.StatusReasonJobTimedOut)
	assert.WithinDuration(t, jobTimedOutEventTimestamp, time.Time(job.StoppedAt), 1*time.Second)
}

func TestUpdateWorkflowStatusJobTimedOut(t *testing.T) {
	c := newSFNManagerTestController(t)
	defer c.tearDown()

	workflow := c.newWorkflow()
	workflow.Status = models.WorkflowStatusRunning
	c.saveWorkflow(t, workflow)

	sfnExecutionARN := c.manager.executionARN(workflow, c.workflowDefinition)
	c.mockSFNAPI.EXPECT().
		DescribeExecutionWithContext(gomock.Any(), &sfn.DescribeExecutionInput{
			ExecutionArn: aws.String(sfnExecutionARN),
		}).
		Return(&sfn.DescribeExecutionOutput{
			Status: aws.String(sfn.ExecutionStatusFailed), // when activity times out, execution immediately fails
		}, nil)

	c.mockSFNAPI.EXPECT().
		GetExecutionHistoryPagesWithContext(gomock.Any(), &sfn.GetExecutionHistoryInput{
			ExecutionArn: aws.String(sfnExecutionARN),
		}, gomock.Any()).
		Do(func(
			ctx aws.Context,
			input *sfn.GetExecutionHistoryInput,
			cb func(historyOutput *sfn.GetExecutionHistoryOutput, lastPage bool) bool,
		) {
			cb(&sfn.GetExecutionHistoryOutput{Events: []*sfn.HistoryEvent{
				jobCreatedEvent,
				jobScheduledEvent,
				jobStartedEvent, // ActivityStarted - starts timer for timeouts & heartbeat checks
				jobTimedOutEvent,
				jobTimedOutWorkflowFailedEvent,
			}}, true)
		})

	require.NoError(t, c.manager.UpdateWorkflowSummary(workflow))
	require.NoError(t, c.manager.UpdateWorkflowHistory(workflow))
	assert.Equal(t, models.WorkflowStatusFailed, workflow.Status)
	require.Len(t, workflow.Jobs, 1)
	assertBasicJobData(t, workflow.Jobs[0])
	assertTimedOutJobData(t, workflow.Jobs[0])
}

var workflowTimedOutEventTimestamp = jobCreatedEventTimestamp.Add(10 * time.Minute)
var workflowTimedOutEvent = &sfn.HistoryEvent{
	Id:        aws.Int64(8),
	Timestamp: aws.Time(workflowTimedOutEventTimestamp),
	Type:      aws.String(sfn.HistoryEventTypeExecutionTimedOut),
}

func assertWorkflowTimedOutJobData(t *testing.T, job *models.Job) {
	assert.Equal(t, models.JobStatusFailed, job.Status)
	assert.Equal(t, "Workflow timed out", resources.StatusReasonWorkflowTimedOut)
	assert.Contains(t, job.StatusReason, resources.StatusReasonWorkflowTimedOut)
	assert.WithinDuration(t, workflowTimedOutEventTimestamp, time.Time(job.StoppedAt), 1*time.Second)
}

func TestUpdateWorkflowStatusWorkflowTimedOut(t *testing.T) {
	c := newSFNManagerTestController(t)
	defer c.tearDown()

	workflow := c.newWorkflow()
	workflow.Status = models.WorkflowStatusRunning
	c.saveWorkflow(t, workflow)

	sfnExecutionARN := c.manager.executionARN(workflow, c.workflowDefinition)
	c.mockSFNAPI.EXPECT().
		DescribeExecutionWithContext(gomock.Any(), &sfn.DescribeExecutionInput{
			ExecutionArn: aws.String(sfnExecutionARN),
		}).
		Return(&sfn.DescribeExecutionOutput{
			Status: aws.String(sfn.ExecutionStatusTimedOut),
		}, nil)

	c.mockSFNAPI.EXPECT().
		GetExecutionHistoryPagesWithContext(gomock.Any(), &sfn.GetExecutionHistoryInput{
			ExecutionArn: aws.String(sfnExecutionARN),
		}, gomock.Any()).
		Do(func(
			ctx aws.Context,
			input *sfn.GetExecutionHistoryInput,
			cb func(historyOutput *sfn.GetExecutionHistoryOutput, lastPage bool) bool,
		) {
			cb(&sfn.GetExecutionHistoryOutput{Events: []*sfn.HistoryEvent{
				jobCreatedEvent,
				workflowTimedOutEvent,
			}}, true)
		})

	require.NoError(t, c.manager.UpdateWorkflowSummary(workflow))
	require.NoError(t, c.manager.UpdateWorkflowHistory(workflow))
	assert.Equal(t, models.WorkflowStatusFailed, workflow.Status)
	assert.Equal(t, "Workflow timed out", resources.StatusReasonWorkflowTimedOut)
	assert.Equal(t, resources.StatusReasonWorkflowTimedOut, workflow.StatusReason)
	require.Len(t, workflow.Jobs, 1)
	assertBasicJobData(t, workflow.Jobs[0])
	assertWorkflowTimedOutJobData(t, workflow.Jobs[0])
}

func newSFNManagerTestController(t *testing.T) *sfnManagerTestController {
	mockController := gomock.NewController(t)
	mockSFNAPI := mocks.NewMockSFNAPI(mockController)
	mockSQSAPI := mocks.NewMockSQSAPI(mockController)
	store := memory.New()

	workflowDefinition := resources.KitchenSinkWorkflowDefinition(t)
	require.NoError(t, store.SaveWorkflowDefinition(*workflowDefinition))

	return &sfnManagerTestController{
		manager:            NewSFNWorkflowManager(mockSFNAPI, mockSQSAPI, store, "", "", "", ""),
		mockController:     mockController,
		mockSFNAPI:         mockSFNAPI,
		mockSQSAPI:         mockSQSAPI,
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

func (c *sfnManagerTestController) updateWorkflow(t *testing.T, workflow *models.Workflow) {
	require.NoError(t, c.store.UpdateWorkflow(*workflow))
}

func (c *sfnManagerTestController) tearDown() {
	c.mockController.Finish()
}
