package executor

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/sfn"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/go-openapi/swag"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Clever/workflow-manager/executor/sfnconventions"
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
		output := sfnconventions.StateMachineName(
			test.input.wdName,
			test.input.wdVersion,
			test.input.namespace,
			test.input.startAt,
		)
		require.Equal(t, output, test.output, "input: %#v", test.input)
	}
}

type stateMachineWithFullActivityARNsAndParametersTest struct {
	name         string
	input        models.SLStateMachine
	wantSMStates map[string]models.SLState
}

var stateMachineWithFullActivityARNsAndParametersTests = []stateMachineWithFullActivityARNsAndParametersTest{
	stateMachineWithFullActivityARNsAndParametersTest{
		name: "happy path",
		input: models.SLStateMachine{
			States: map[string]models.SLState{
				"foostate": models.SLState{
					Type:     models.SLStateTypeTask,
					Resource: "resource-name",
				},
				"foostatelambda": models.SLState{
					Type:     models.SLStateTypeTask,
					Resource: "lambda:resource-name",
				},
				"foostateglue": models.SLState{
					Type:     models.SLStateTypeTask,
					Resource: "glue:resource-name",
					Parameters: map[string]interface{}{
						"Arguments.$": "$.path",
					},
				},
			},
		},
		wantSMStates: map[string]models.SLState{
			"foostate": models.SLState{
				Type:     models.SLStateTypeTask,
				Resource: "arn:aws:states:region:accountID:activity:namespace--resource-name",
			},
			"foostatelambda": models.SLState{
				Type:     models.SLStateTypeTask,
				Resource: "arn:aws:lambda:region:accountID:function:namespace--resource-name",
			},
			"foostateglue": models.SLState{
				Type:     models.SLStateTypeTask,
				Resource: "arn:aws:states:::glue:startJobRun.sync",
				Parameters: map[string]interface{}{
					"JobName":     "namespace--resource-name",
					"Arguments.$": "$.path",
				},
			},
		},
	},
	stateMachineWithFullActivityARNsAndParametersTest{
		name: "glue state with no extra arguments",
		input: models.SLStateMachine{
			States: map[string]models.SLState{
				"foostateglue": models.SLState{
					Type:     models.SLStateTypeTask,
					Resource: "glue:resource-name",
				},
			},
		},
		wantSMStates: map[string]models.SLState{
			"foostateglue": models.SLState{
				Type:     models.SLStateTypeTask,
				Resource: "arn:aws:states:::glue:startJobRun.sync",
				Parameters: map[string]interface{}{
					"JobName": "namespace--resource-name",
				},
			},
		},
	},
	stateMachineWithFullActivityARNsAndParametersTest{
		name: "glue state with explicit arguments",
		input: models.SLStateMachine{
			States: map[string]models.SLState{
				"foostateglue": models.SLState{
					Type:     models.SLStateTypeTask,
					Resource: "glue:resource-name",
					Parameters: map[string]interface{}{
						"Arguments": map[string]interface{}{
							"arg1": "val1",
							"arg2": 1,
						},
					},
				},
			},
		},
		wantSMStates: map[string]models.SLState{
			"foostateglue": models.SLState{
				Type:     models.SLStateTypeTask,
				Resource: "arn:aws:states:::glue:startJobRun.sync",
				Parameters: map[string]interface{}{
					"JobName": "namespace--resource-name",
					"Arguments": map[string]interface{}{
						"arg1": "val1",
						"arg2": 1,
					},
				},
			},
		},
	},
	stateMachineWithFullActivityARNsAndParametersTest{
		name: "map state adds EXECUTION_NAME and expands iterator states",
		input: models.SLStateMachine{
			States: map[string]models.SLState{
				"foostatemap": models.SLState{
					Type: models.SLStateTypeMap,
					Iterator: &models.SLStateMachine{
						StartAt: "foostate",
						States: map[string]models.SLState{
							"foostate": models.SLState{
								Type:     models.SLStateTypeTask,
								Resource: "resource-name",
							},
						},
					},
				},
			},
		},
		wantSMStates: map[string]models.SLState{
			"foostatemap": models.SLState{
				Type: models.SLStateTypeMap,
				Parameters: map[string]interface{}{
					"_EXECUTION_NAME.$": "$._EXECUTION_NAME",
				},
				Iterator: &models.SLStateMachine{
					StartAt: "foostate",
					States: map[string]models.SLState{
						"foostate": models.SLState{
							Type:     models.SLStateTypeTask,
							Resource: "arn:aws:states:region:accountID:activity:namespace--resource-name",
						},
					},
				},
			},
		},
	},
}

func TestStateMachineWithFullActivityARNsAndParameters(t *testing.T) {
	for _, test := range stateMachineWithFullActivityARNsAndParametersTests {
		smWithFullActivityARNs, err := stateMachineWithFullActivityARNsAndParameters(test.input, "region", "accountID", "namespace")
		assert.Equal(t, test.wantSMStates, smWithFullActivityARNs.States)
		assert.NoError(t, err)
	}
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
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		c := newSFNManagerTestController(t)
		defer c.tearDown()
		stateMachineArn := sfnconventions.StateMachineArn(c.manager.region, c.manager.accountID,
			c.workflowDefinition.Name,
			c.workflowDefinition.Version,
			"namespace",
			c.workflowDefinition.StateMachine.StartAt,
		)
		c.mockSFNAPI.EXPECT().
			DescribeStateMachineWithContext(gomock.Any(), &sfn.DescribeStateMachineInput{
				StateMachineArn: aws.String(stateMachineArn),
			}).
			Return(&sfn.DescribeStateMachineOutput{
				StateMachineArn: aws.String(stateMachineArn),
			}, nil)
		c.mockSFNAPI.EXPECT().
			StartExecutionWithContext(gomock.Any(), gomock.Any()).
			Return(&sfn.StartExecutionOutput{}, nil)
		c.mockSQSAPI.EXPECT().
			SendMessageWithContext(gomock.Any(), gomock.Any()).
			Return(&sqs.SendMessageOutput{}, nil)

		defaultTags := map[string]interface{}{
			"tag1": "val1",
			"tag2": "val2",
			"tag3": "val3",
		}
		assert.Equal(t, c.workflowDefinition.DefaultTags, defaultTags)
		workflow, err := c.manager.CreateWorkflow(ctx, *c.workflowDefinition,
			input,
			"namespace",
			"queue",
			map[string]interface{}{},
		)
		assert.Nil(t, err)
		assert.NotNil(t, workflow)
		assert.Equal(t, workflow.Namespace, "namespace")
		assert.Equal(t, workflow.Input, input)

		// Create called without tags, so tags should match c.workflowDefinition.DefaultTags
		assert.Equal(t, workflow.Tags, defaultTags)
		// Ensure workflow definition tags not modified by CreateWorkflow()
		assert.Equal(t, c.workflowDefinition.DefaultTags, defaultTags)

		savedWorkflow, err := c.store.GetWorkflowByID(ctx, workflow.ID)
		assert.Nil(t, err)
		assert.Equal(t, workflow.CreatedAt.String(), savedWorkflow.CreatedAt.String())
		assert.Equal(t, workflow.ID, savedWorkflow.ID)

		t.Log("Verify updatePendingWorkflow causes in-progress workflow to be put back into the update queue")
		sfnExecutionARN := c.manager.executionArn(workflow, c.workflowDefinition)
		c.mockSFNAPI.EXPECT().
			DescribeExecutionWithContext(gomock.Any(), &sfn.DescribeExecutionInput{
				ExecutionArn: aws.String(sfnExecutionARN),
			}).
			Return(&sfn.DescribeExecutionOutput{
				Status: aws.String(sfn.ExecutionStatusRunning),
			}, nil)

		// These calls mean the message is processed and then put back into the queue
		msg := &sqs.Message{
			Body:          aws.String(workflow.ID),
			ReceiptHandle: aws.String("first-message"),
		}

		c.mockSQSAPI.EXPECT().
			SendMessageWithContext(gomock.Any(), &sqs.SendMessageInput{
				QueueUrl:     aws.String(""),
				DelaySeconds: aws.Int64(updateLoopDelay),
				MessageBody:  aws.String(workflow.ID),
			}).
			Return(&sqs.SendMessageOutput{}, nil)
		c.mockSQSAPI.EXPECT().
			DeleteMessageWithContext(gomock.Any(), &sqs.DeleteMessageInput{
				QueueUrl:      aws.String(""),
				ReceiptHandle: aws.String("first-message"),
			}).
			Return(&sqs.DeleteMessageOutput{}, nil)

		wfID, err := updatePendingWorkflow(ctx, msg, c.manager, c.store, c.mockSQSAPI, "")
		assert.Nil(t, err)
		assert.Equal(t, workflow.ID, wfID)
	})

	t.Run("CreateWorkflow with added tags", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		c := newSFNManagerTestController(t)
		defer c.tearDown()
		stateMachineArn := sfnconventions.StateMachineArn(c.manager.region, c.manager.accountID,
			c.workflowDefinition.Name,
			c.workflowDefinition.Version,
			"namespace",
			c.workflowDefinition.StateMachine.StartAt,
		)
		c.mockSFNAPI.EXPECT().
			DescribeStateMachineWithContext(gomock.Any(), &sfn.DescribeStateMachineInput{
				StateMachineArn: aws.String(stateMachineArn),
			}).
			Return(&sfn.DescribeStateMachineOutput{
				StateMachineArn: aws.String(stateMachineArn),
			}, nil)
		c.mockSFNAPI.EXPECT().
			StartExecutionWithContext(gomock.Any(), gomock.Any()).
			Return(&sfn.StartExecutionOutput{}, nil)
		c.mockSQSAPI.EXPECT().
			SendMessageWithContext(gomock.Any(), gomock.Any()).
			Return(&sqs.SendMessageOutput{}, nil)

		defaultTags := map[string]interface{}{
			"tag1": "val1",
			"tag2": "val2",
			"tag3": "val3",
		}
		workflow, err := c.manager.CreateWorkflow(ctx, *c.workflowDefinition,
			input,
			"namespace",
			"queue",
			map[string]interface{}{"newTag1": "newVal1", "newTag2": "newVal2"},
		)
		assert.Nil(t, err)
		// Create called with tags, so they should be added to c.workflowDefinition.DefaultTags
		// in the new workflow
		assert.Equal(t, workflow.Tags, map[string]interface{}{
			"tag1":    "val1",
			"tag2":    "val2",
			"tag3":    "val3",
			"newTag1": "newVal1",
			"newTag2": "newVal2",
		})
		// Ensure workflow definition tags not modified by CreateWorkflow()
		assert.Equal(t, c.workflowDefinition.DefaultTags, defaultTags)
	})

	t.Run("CreateWorkflow deletes workflow on StartExecution failure", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		c := newSFNManagerTestController(t)
		defer c.tearDown()
		stateMachineArn := sfnconventions.StateMachineArn(c.manager.region, c.manager.accountID,
			c.workflowDefinition.Name,
			c.workflowDefinition.Version,
			"namespace",
			c.workflowDefinition.StateMachine.StartAt,
		)
		awsError := awserr.New("test", "test", errors.New(""))

		c.mockSQSAPI.EXPECT().
			SendMessageWithContext(gomock.Any(), gomock.Any()).
			Return(&sqs.SendMessageOutput{}, nil)
		c.mockSFNAPI.EXPECT().
			DescribeStateMachineWithContext(gomock.Any(), &sfn.DescribeStateMachineInput{
				StateMachineArn: aws.String(stateMachineArn),
			}).
			Return(&sfn.DescribeStateMachineOutput{
				StateMachineArn: aws.String(stateMachineArn),
			}, nil)
		c.mockSFNAPI.EXPECT().
			StartExecutionWithContext(gomock.Any(), gomock.Any()).
			Return(nil, awsError)

		workflow, err := c.manager.CreateWorkflow(ctx, *c.workflowDefinition,
			input,
			"namespace",
			"queue",
			map[string]interface{}{},
		)
		assert.NotNil(t, err)
		assert.Nil(t, workflow)
		assert.IsType(t, awsError, err)
		assert.Equal(t, "test", err.(awserr.Error).Code()) // ensure this error came from sfn api
	})
}

func TestRetryWorkflow(t *testing.T) {
	input := "{\"json\": true}"

	t.Run("RetryWorkflow for existing StateMachines", func(t *testing.T) {

		t.Log("Create a workflow")
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		c := newSFNManagerTestController(t)
		defer c.tearDown()
		stateMachineArn := sfnconventions.StateMachineArn(c.manager.region, c.manager.accountID,
			c.workflowDefinition.Name,
			c.workflowDefinition.Version,
			"namespace",
			c.workflowDefinition.StateMachine.StartAt,
		)
		c.mockSFNAPI.EXPECT().
			DescribeStateMachineWithContext(gomock.Any(), &sfn.DescribeStateMachineInput{
				StateMachineArn: aws.String(stateMachineArn),
			}).
			Return(&sfn.DescribeStateMachineOutput{
				StateMachineArn: aws.String(stateMachineArn),
			}, nil)
		c.mockSFNAPI.EXPECT().
			StartExecutionWithContext(gomock.Any(), gomock.Any()).
			Return(&sfn.StartExecutionOutput{}, nil)
		c.mockSQSAPI.EXPECT().
			SendMessageWithContext(gomock.Any(), gomock.Any()).
			Return(&sqs.SendMessageOutput{}, nil)

		workflow, err := c.manager.CreateWorkflow(ctx, *c.workflowDefinition,
			input,
			"namespace",
			"queue",
			map[string]interface{}{},
		)
		assert.Nil(t, err)
		assert.NotNil(t, workflow)
		assert.Equal(t, workflow.Namespace, "namespace")
		assert.Equal(t, workflow.Input, input)

		savedWorkflow, err := c.store.GetWorkflowByID(ctx, workflow.ID)
		assert.Nil(t, err)
		assert.Equal(t, workflow.CreatedAt.String(), savedWorkflow.CreatedAt.String())
		assert.Equal(t, workflow.ID, savedWorkflow.ID)

		sfnExecutionARN := c.manager.executionArn(workflow, c.workflowDefinition)

		t.Log("RetryWorkflow should fail if workflow is not yet done")
		_, err = c.manager.RetryWorkflow(ctx, *workflow, workflow.WorkflowDefinition.StateMachine.StartAt, input)
		assert.Error(t, err)

		t.Log("Set workflow to failed, then retry it")
		workflow.Status = models.WorkflowStatusFailed

		c.mockSFNAPI.EXPECT().
			DescribeStateMachineWithContext(gomock.Any(), gomock.Any()).
			Return(&sfn.DescribeStateMachineOutput{
				StateMachineArn: aws.String(stateMachineArn),
			}, nil)
		c.mockSFNAPI.EXPECT().
			StartExecutionWithContext(gomock.Any(), gomock.Any()).
			Return(&sfn.StartExecutionOutput{}, nil)
		c.mockSQSAPI.EXPECT().
			SendMessageWithContext(gomock.Any(), gomock.Any()).
			Return(&sqs.SendMessageOutput{}, nil)

		workflow2, err := c.manager.RetryWorkflow(ctx, *workflow, workflow.WorkflowDefinition.StateMachine.StartAt, input)
		assert.Nil(t, err)
		assert.NotNil(t, workflow2)
		assert.Equal(t, workflow2.Namespace, "namespace")
		assert.Equal(t, workflow2.Input, input)

		savedWorkflow2, err := c.store.GetWorkflowByID(ctx, workflow2.ID)
		assert.Nil(t, err)
		assert.Equal(t, workflow2.CreatedAt.String(), savedWorkflow2.CreatedAt.String())
		assert.Equal(t, workflow2.ID, savedWorkflow2.ID)

		sfnExecutionARN2 := c.manager.executionArn(workflow2, c.workflowDefinition)
		assert.NotEqual(t, sfnExecutionARN2, sfnExecutionARN)
	})
}

func TestCancelWorkflow(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c := newSFNManagerTestController(t)
	defer c.tearDown()

	workflow := c.newWorkflow()
	initialStatus := models.WorkflowStatusRunning
	workflow.Status = initialStatus
	c.saveWorkflow(ctx, t, workflow)
	assert.Equal(t, false, workflow.ResolvedByUser)

	t.Log("Verify execution is stopped and status reason is updated.")
	reason := "i have my reasons"
	sfnExecutionARN := c.manager.executionArn(workflow, c.workflowDefinition)
	c.mockSFNAPI.EXPECT().
		StopExecutionWithContext(gomock.Any(), &sfn.StopExecutionInput{
			ExecutionArn: aws.String(sfnExecutionARN),
			Cause:        aws.String(reason),
		}).
		Return(&sfn.StopExecutionOutput{}, nil)
	require.NoError(t, c.manager.CancelWorkflow(ctx, workflow, reason))
	assert.Equal(t, initialStatus, workflow.Status)
	assert.Equal(t, true, workflow.ResolvedByUser)
	assert.Equal(t, reason, workflow.StatusReason)

	t.Log("Failed Workflows cannot be cancelled.")
	workflow.Status = models.WorkflowStatusFailed
	c.updateWorkflow(ctx, t, workflow)
	require.Error(t, c.manager.CancelWorkflow(ctx, workflow, reason))

	t.Log("Successful Workflows cannot be cancelled.")
	workflow.Status = models.WorkflowStatusSucceeded
	c.updateWorkflow(ctx, t, workflow)
	require.Error(t, c.manager.CancelWorkflow(ctx, workflow, reason))
}

func TestUpdateWorkflowStatusNoop(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c := newSFNManagerTestController(t)
	defer c.tearDown()

	workflow := c.newWorkflow()
	workflow.Status = models.WorkflowStatusCancelled
	workflow.ResolvedByUser = true
	c.saveWorkflow(ctx, t, workflow)

	require.NoError(t, c.manager.UpdateWorkflowSummary(ctx, workflow))
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
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c := newSFNManagerTestController(t)
	defer c.tearDown()

	workflow := c.newWorkflow()
	workflow.Status = models.WorkflowStatusQueued
	c.saveWorkflow(ctx, t, workflow)

	sfnExecutionARN := c.manager.executionArn(workflow, c.workflowDefinition)
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
			MaxResults:   aws.Int64(executionEventsPerPage),
		}, gomock.Any()).
		Do(func(
			ctx aws.Context,
			input *sfn.GetExecutionHistoryInput,
			cb func(historyOutput *sfn.GetExecutionHistoryOutput, lastPage bool) bool,
			options ...request.Option,
		) {
			cb(&sfn.GetExecutionHistoryOutput{Events: []*sfn.HistoryEvent{jobCreatedEvent}}, true)
		})

	require.NoError(t, c.manager.UpdateWorkflowSummary(ctx, workflow))
	require.NoError(t, c.manager.UpdateWorkflowHistory(ctx, workflow))
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
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c := newSFNManagerTestController(t)
	defer c.tearDown()

	workflow := c.newWorkflow()
	workflow.Status = models.WorkflowStatusQueued
	c.saveWorkflow(ctx, t, workflow)

	sfnExecutionARN := c.manager.executionArn(workflow, c.workflowDefinition)
	c.mockSFNAPI.EXPECT().
		DescribeExecutionWithContext(gomock.Any(), &sfn.DescribeExecutionInput{
			ExecutionArn: aws.String(sfnExecutionARN),
		}).
		Return(&sfn.DescribeExecutionOutput{
			Status: aws.String(sfn.ExecutionStatusFailed),
		}, nil)

	events := []*sfn.HistoryEvent{
		jobCreatedEvent,
		jobFailedEvent,
	}

	c.mockSFNAPI.EXPECT().
		GetExecutionHistoryPagesWithContext(gomock.Any(), &sfn.GetExecutionHistoryInput{
			ExecutionArn: aws.String(sfnExecutionARN),
			MaxResults:   aws.Int64(executionEventsPerPage),
		}, gomock.Any()).
		Do(func(
			ctx aws.Context,
			input *sfn.GetExecutionHistoryInput,
			cb func(historyOutput *sfn.GetExecutionHistoryOutput, lastPage bool) bool,
			options ...request.Option,
		) {
			cb(&sfn.GetExecutionHistoryOutput{Events: events}, true)
		})

	c.mockSFNAPI.EXPECT().
		GetExecutionHistoryWithContext(gomock.Any(), &sfn.GetExecutionHistoryInput{
			ExecutionArn: aws.String(sfnExecutionARN),
			ReverseOrder: aws.Bool(true),
		}).
		Return(&sfn.GetExecutionHistoryOutput{
			Events: reverseHistory(events),
		}, nil).
		Times(1)

	require.NoError(t, c.manager.UpdateWorkflowSummary(ctx, workflow))
	require.NoError(t, c.manager.UpdateWorkflowHistory(ctx, workflow))
	assert.Equal(t, models.WorkflowStatusFailed, workflow.Status)
	require.Len(t, workflow.Jobs, 1)
	assertBasicJobData(t, workflow.Jobs[0])
	assert.Equal(t, models.JobStatusFailed, workflow.Jobs[0].Status)
	assert.Equal(t, "line4\nline5\nline6", workflow.Jobs[0].StatusReason)
	assert.WithinDuration(
		t, jobFailedEventTimestamp, time.Time(workflow.Jobs[0].StoppedAt), 1*time.Second,
	)
	require.NotNil(t, workflow.LastJob)
	assertBasicJobData(t, workflow.LastJob)
	assert.Equal(t, models.JobStatusFailed, workflow.LastJob.Status)
	assert.Equal(t, "line4\nline5\nline6", workflow.LastJob.StatusReason)
	assert.WithinDuration(
		t, jobFailedEventTimestamp, time.Time(workflow.LastJob.StoppedAt), 1*time.Second,
	)
}

func TestUpdateWorkflowStatusJobFailedNotDeployed(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c := newSFNManagerTestController(t)
	defer c.tearDown()

	workflow := c.newWorkflow()
	workflow.Status = models.WorkflowStatusQueued
	c.saveWorkflow(ctx, t, workflow)

	sfnExecutionARN := c.manager.executionArn(workflow, c.workflowDefinition)
	c.mockSFNAPI.EXPECT().
		DescribeExecutionWithContext(gomock.Any(), &sfn.DescribeExecutionInput{
			ExecutionArn: aws.String(sfnExecutionARN),
		}).
		Return(&sfn.DescribeExecutionOutput{
			Status: aws.String(sfn.ExecutionStatusFailed),
		}, nil)

	events := []*sfn.HistoryEvent{
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
	}

	c.mockSFNAPI.EXPECT().
		GetExecutionHistoryPagesWithContext(gomock.Any(), &sfn.GetExecutionHistoryInput{
			ExecutionArn: aws.String(sfnExecutionARN),
			MaxResults:   aws.Int64(executionEventsPerPage),
		}, gomock.Any()).
		Do(func(
			ctx aws.Context,
			input *sfn.GetExecutionHistoryInput,
			cb func(historyOutput *sfn.GetExecutionHistoryOutput, lastPage bool) bool,
			options ...request.Option,
		) {
			cb(&sfn.GetExecutionHistoryOutput{Events: events}, true)
		})

	c.mockSFNAPI.EXPECT().
		GetExecutionHistoryWithContext(gomock.Any(), &sfn.GetExecutionHistoryInput{
			ExecutionArn: aws.String(sfnExecutionARN),
			ReverseOrder: aws.Bool(true),
		}).
		Return(&sfn.GetExecutionHistoryOutput{
			Events: reverseHistory(events),
		}, nil).
		Times(1)

	require.NoError(t, c.manager.UpdateWorkflowSummary(ctx, workflow))
	require.NoError(t, c.manager.UpdateWorkflowHistory(ctx, workflow))
	assert.Equal(t, models.WorkflowStatusFailed, workflow.Status)
	require.Len(t, workflow.Jobs, 1)
	assert.Equal(t, models.JobStatusFailed, workflow.Jobs[0].Status)
	assert.Equal(t, "State resource does not exist", workflow.Jobs[0].StatusReason)
	assert.WithinDuration(
		t, jobFailedEventTimestamp, time.Time(workflow.Jobs[0].StoppedAt), 1*time.Second,
	)
	require.NotNil(t, workflow.LastJob)
	assertBasicJobData(t, workflow.LastJob)
	assert.Equal(t, models.JobStatusFailed, workflow.LastJob.Status)
	assert.Equal(t, "State resource does not exist", workflow.LastJob.StatusReason)
	assert.WithinDuration(
		t, jobFailedEventTimestamp, time.Time(workflow.LastJob.StoppedAt), 1*time.Second,
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
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c := newSFNManagerTestController(t)
	defer c.tearDown()

	workflow := c.newWorkflow()
	workflow.Status = models.WorkflowStatusQueued
	c.saveWorkflow(ctx, t, workflow)

	executionOutput := `{"output": true}`
	sfnExecutionARN := c.manager.executionArn(workflow, c.workflowDefinition)
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
			MaxResults:   aws.Int64(executionEventsPerPage),
		}, gomock.Any()).
		Do(func(
			ctx aws.Context,
			input *sfn.GetExecutionHistoryInput,
			cb func(historyOutput *sfn.GetExecutionHistoryOutput, lastPage bool) bool,
			options ...request.Option,
		) {
			cb(&sfn.GetExecutionHistoryOutput{Events: []*sfn.HistoryEvent{
				jobCreatedEvent,
				jobSucceededEvent,
				jobExitedEvent,
			}}, true)
		})

	require.NoError(t, c.manager.UpdateWorkflowSummary(ctx, workflow))
	require.NoError(t, c.manager.UpdateWorkflowHistory(ctx, workflow))
	assert.Equal(t, models.WorkflowStatusSucceeded, workflow.Status)
	assert.Equal(t, executionOutput, workflow.Output)
	require.Len(t, workflow.Jobs, 1)
	assertBasicJobData(t, workflow.Jobs[0])
	assertSucceededJobData(t, workflow.Jobs[0])
	require.Nil(t, workflow.LastJob)
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
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c := newSFNManagerTestController(t)
	defer c.tearDown()

	workflow := c.newWorkflow()
	workflow.Status = models.WorkflowStatusQueued
	workflow.StatusReason = "cancelled by user"
	c.saveWorkflow(ctx, t, workflow)

	sfnExecutionARN := c.manager.executionArn(workflow, c.workflowDefinition)
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
			MaxResults:   aws.Int64(executionEventsPerPage),
		}, gomock.Any()).
		Do(func(
			ctx aws.Context,
			input *sfn.GetExecutionHistoryInput,
			cb func(historyOutput *sfn.GetExecutionHistoryOutput, lastPage bool) bool,
			options ...request.Option,
		) {
			cb(&sfn.GetExecutionHistoryOutput{Events: []*sfn.HistoryEvent{
				jobCreatedEvent,
				jobAbortedEvent,
			}}, true)
		})

	require.NoError(t, c.manager.UpdateWorkflowSummary(ctx, workflow))
	require.NoError(t, c.manager.UpdateWorkflowHistory(ctx, workflow))
	assert.Equal(t, models.WorkflowStatusCancelled, workflow.Status)
	assert.Equal(t, "cancelled by user", workflow.StatusReason)
	require.Len(t, workflow.Jobs, 1)
	assertBasicJobData(t, workflow.Jobs[0])
	assertCancelledJobData(t, workflow.Jobs[0])
}

func TestUpdateWorkflowStatusWorkflowCancelledAfterJobSucceeded(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c := newSFNManagerTestController(t)
	defer c.tearDown()

	workflow := c.newWorkflow()
	workflow.Status = models.WorkflowStatusQueued
	workflow.StatusReason = "cancelled by user"
	c.saveWorkflow(ctx, t, workflow)

	sfnExecutionARN := c.manager.executionArn(workflow, c.workflowDefinition)
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
			MaxResults:   aws.Int64(executionEventsPerPage),
		}, gomock.Any()).
		Do(func(
			ctx aws.Context,
			input *sfn.GetExecutionHistoryInput,
			cb func(historyOutput *sfn.GetExecutionHistoryOutput, lastPage bool) bool,
			options ...request.Option,
		) {
			cb(&sfn.GetExecutionHistoryOutput{Events: []*sfn.HistoryEvent{
				jobCreatedEvent,
				jobSucceededEvent,
				jobAbortedEvent,
			}}, true)
		})

	require.NoError(t, c.manager.UpdateWorkflowSummary(ctx, workflow))
	require.NoError(t, c.manager.UpdateWorkflowHistory(ctx, workflow))
	assert.Equal(t, models.WorkflowStatusCancelled, workflow.Status)
	assert.Equal(t, "cancelled by user", workflow.StatusReason)
	require.Len(t, workflow.Jobs, 1)
	assertBasicJobData(t, workflow.Jobs[0])
	assertCancelledJobData(t, workflow.Jobs[0])
}

func TestUpdateWorkflowStatusExecutionNotFoundRetry(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c := newSFNManagerTestController(t)
	defer c.tearDown()

	workflow := c.newWorkflow()
	workflow.Status = models.WorkflowStatusQueued
	c.saveWorkflow(ctx, t, workflow)

	executionOutput := `{"output": true}`
	sfnExecutionARN := c.manager.executionArn(workflow, c.workflowDefinition)
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
			MaxResults:   aws.Int64(executionEventsPerPage),
		}, gomock.Any()).
		Do(func(
			ctx aws.Context,
			input *sfn.GetExecutionHistoryInput,
			cb func(historyOutput *sfn.GetExecutionHistoryOutput, lastPage bool) bool,
			options ...request.Option,
		) {
			cb(&sfn.GetExecutionHistoryOutput{Events: []*sfn.HistoryEvent{
				jobCreatedEvent,
				jobSucceededEvent,
				jobExitedEvent,
			}}, true)
		})

	require.NoError(t, c.manager.UpdateWorkflowSummary(ctx, workflow))
	assert.Equal(t, models.WorkflowStatusQueued, workflow.Status)
	require.Len(t, workflow.Jobs, 0)

	require.NoError(t, c.manager.UpdateWorkflowSummary(ctx, workflow))
	assert.Equal(t, models.WorkflowStatusSucceeded, workflow.Status)
	require.NoError(t, c.manager.UpdateWorkflowHistory(ctx, workflow))
	require.Len(t, workflow.Jobs, 1)
	assertBasicJobData(t, workflow.Jobs[0])
	assertSucceededJobData(t, workflow.Jobs[0])
}

func TestUpdateWorkflowStatusExecutionNotFoundStopRetry(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	durationToRetryDescribeExecutions = 1 * time.Second
	c := newSFNManagerTestController(t)
	defer c.tearDown()

	workflow := c.newWorkflow()
	workflow.Status = models.WorkflowStatusQueued
	c.saveWorkflow(ctx, t, workflow)

	sfnExecutionARN := c.manager.executionArn(workflow, c.workflowDefinition)
	c.mockSFNAPI.EXPECT().
		DescribeExecutionWithContext(gomock.Any(), &sfn.DescribeExecutionInput{
			ExecutionArn: aws.String(sfnExecutionARN),
		}).
		Return(nil, awserr.New(sfn.ErrCodeExecutionDoesNotExist,
			"test",
			errors.New("")),
		).
		Times(2)

	require.NoError(t, c.manager.UpdateWorkflowSummary(ctx, workflow))
	assert.Equal(t, models.WorkflowStatusQueued, workflow.Status)
	require.Len(t, workflow.Jobs, 0)
	time.Sleep(durationToRetryDescribeExecutions)

	require.NoError(t, c.manager.UpdateWorkflowSummary(ctx, workflow))
	assert.Equal(t, models.WorkflowStatusFailed, workflow.Status)
	require.Len(t, workflow.Jobs, 0)
	require.Nil(t, workflow.LastJob)
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
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c := newSFNManagerTestController(t)
	defer c.tearDown()

	workflow := c.newWorkflow()
	workflow.Status = models.WorkflowStatusRunning
	c.saveWorkflow(ctx, t, workflow)

	sfnExecutionARN := c.manager.executionArn(workflow, c.workflowDefinition)
	c.mockSFNAPI.EXPECT().
		DescribeExecutionWithContext(gomock.Any(), &sfn.DescribeExecutionInput{
			ExecutionArn: aws.String(sfnExecutionARN),
		}).
		Return(&sfn.DescribeExecutionOutput{
			Status: aws.String(sfn.ExecutionStatusFailed), // when activity times out, execution immediately fails
		}, nil)

	events := []*sfn.HistoryEvent{
		jobCreatedEvent,
		jobScheduledEvent,
		jobStartedEvent, // ActivityStarted - starts timer for timeouts & heartbeat checks
		jobTimedOutEvent,
		jobTimedOutWorkflowFailedEvent,
	}

	c.mockSFNAPI.EXPECT().
		GetExecutionHistoryPagesWithContext(gomock.Any(), &sfn.GetExecutionHistoryInput{
			ExecutionArn: aws.String(sfnExecutionARN),
			MaxResults:   aws.Int64(executionEventsPerPage),
		}, gomock.Any()).
		Do(func(
			ctx aws.Context,
			input *sfn.GetExecutionHistoryInput,
			cb func(historyOutput *sfn.GetExecutionHistoryOutput, lastPage bool) bool,
			options ...request.Option,
		) {
			cb(&sfn.GetExecutionHistoryOutput{Events: events}, true)
		})

	c.mockSFNAPI.EXPECT().
		GetExecutionHistoryWithContext(gomock.Any(), &sfn.GetExecutionHistoryInput{
			ExecutionArn: aws.String(sfnExecutionARN),
			ReverseOrder: aws.Bool(true),
		}).
		Return(&sfn.GetExecutionHistoryOutput{
			Events: reverseHistory(events),
		}, nil).
		Times(1)

	require.NoError(t, c.manager.UpdateWorkflowSummary(ctx, workflow))
	require.NoError(t, c.manager.UpdateWorkflowHistory(ctx, workflow))
	assert.Equal(t, models.WorkflowStatusFailed, workflow.Status)
	require.Len(t, workflow.Jobs, 1)
	assertBasicJobData(t, workflow.Jobs[0])
	assertTimedOutJobData(t, workflow.Jobs[0])
	require.NotNil(t, workflow.LastJob)
	assertBasicJobData(t, workflow.LastJob)
	assertTimedOutJobData(t, workflow.LastJob)
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
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c := newSFNManagerTestController(t)
	defer c.tearDown()

	workflow := c.newWorkflow()
	workflow.Status = models.WorkflowStatusRunning
	c.saveWorkflow(ctx, t, workflow)

	sfnExecutionARN := c.manager.executionArn(workflow, c.workflowDefinition)
	c.mockSFNAPI.EXPECT().
		DescribeExecutionWithContext(gomock.Any(), &sfn.DescribeExecutionInput{
			ExecutionArn: aws.String(sfnExecutionARN),
		}).
		Return(&sfn.DescribeExecutionOutput{
			Status: aws.String(sfn.ExecutionStatusTimedOut),
		}, nil)

	events := []*sfn.HistoryEvent{
		jobCreatedEvent,
		workflowTimedOutEvent,
	}

	c.mockSFNAPI.EXPECT().
		GetExecutionHistoryPagesWithContext(gomock.Any(), &sfn.GetExecutionHistoryInput{
			ExecutionArn: aws.String(sfnExecutionARN),
			MaxResults:   aws.Int64(executionEventsPerPage),
		}, gomock.Any()).
		Do(func(
			ctx aws.Context,
			input *sfn.GetExecutionHistoryInput,
			cb func(historyOutput *sfn.GetExecutionHistoryOutput, lastPage bool) bool,
			options ...request.Option,
		) {
			cb(&sfn.GetExecutionHistoryOutput{Events: events}, true)
		})

	c.mockSFNAPI.EXPECT().
		GetExecutionHistoryWithContext(gomock.Any(), &sfn.GetExecutionHistoryInput{
			ExecutionArn: aws.String(sfnExecutionARN),
			ReverseOrder: aws.Bool(true),
		}).
		Return(&sfn.GetExecutionHistoryOutput{
			Events: reverseHistory(events),
		}, nil).
		Times(1)

	require.NoError(t, c.manager.UpdateWorkflowSummary(ctx, workflow))
	require.NoError(t, c.manager.UpdateWorkflowHistory(ctx, workflow))
	assert.Equal(t, models.WorkflowStatusFailed, workflow.Status)
	require.Len(t, workflow.Jobs, 1)
	assertBasicJobData(t, workflow.Jobs[0])
	assertWorkflowTimedOutJobData(t, workflow.Jobs[0])
	require.NotNil(t, workflow.LastJob)
	assertBasicJobData(t, workflow.LastJob)
	assertWorkflowTimedOutJobData(t, workflow.LastJob)
}

func TestReverseHistory(t *testing.T) {
	events := []*sfn.HistoryEvent{
		jobCreatedEvent,
		jobScheduledEvent,
		jobStartedEvent,
		jobTimedOutEvent,
		jobTimedOutWorkflowFailedEvent,
	}
	reversedEvents := []*sfn.HistoryEvent{
		jobTimedOutWorkflowFailedEvent,
		jobTimedOutEvent,
		jobStartedEvent,
		jobScheduledEvent,
		jobCreatedEvent,
	}

	assert.Equal(t, reversedEvents, reverseHistory(events))
}

func newSFNManagerTestController(t *testing.T) *sfnManagerTestController {
	mockController := gomock.NewController(t)
	mockSFNAPI := mocks.NewMockSFNAPI(mockController)
	mockSQSAPI := mocks.NewMockSQSAPI(mockController)
	store := memory.New()

	workflowDefinition := resources.KitchenSinkWorkflowDefinition(t)
	require.NoError(t, store.SaveWorkflowDefinition(context.Background(), *workflowDefinition))

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

func (c *sfnManagerTestController) saveWorkflow(ctx context.Context, t *testing.T, workflow *models.Workflow) {
	require.NoError(t, c.store.SaveWorkflow(ctx, *workflow))
}

func (c *sfnManagerTestController) updateWorkflow(ctx context.Context, t *testing.T, workflow *models.Workflow) {
	require.NoError(t, c.store.UpdateWorkflow(ctx, *workflow))
}

func (c *sfnManagerTestController) tearDown() {
	c.mockController.Finish()
}
