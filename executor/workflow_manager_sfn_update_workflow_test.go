package executor

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Clever/workflow-manager/gen-go/models"
	"github.com/Clever/workflow-manager/mocks/mock_sfniface"
	"github.com/Clever/workflow-manager/store/memory"
	"github.com/aws/aws-sdk-go/private/protocol/json/jsonutil"
	"github.com/aws/aws-sdk-go/service/sfn"
	"github.com/golang/mock/gomock"
	"github.com/mohae/deepcopy"
	"github.com/stretchr/testify/require"
)

type updateWorkflowStatusSFNState struct {
	DescribeExecutionOutput   *sfn.DescribeExecutionOutput
	GetExecutionHistoryOutput *sfn.GetExecutionHistoryOutput
}

type updateWorkflowStatusInput struct {
	Workflow models.Workflow
}

type updateWorkflowStatusOutput struct {
	Workflow models.Workflow
	Error    error
}

type updateWorkflowStatusTest struct {
	SFNState updateWorkflowStatusSFNState
	Input    updateWorkflowStatusInput
	Output   updateWorkflowStatusOutput
}

func (uwst updateWorkflowStatusTest) Run(t *testing.T) {
	store := memory.New()
	mockController := gomock.NewController(t)
	defer mockController.Finish()
	mockSFNAPI := mock_sfniface.NewMockSFNAPI(mockController)
	mockSFNAPI.EXPECT().
		DescribeExecution(gomock.Any()).
		Return(uwst.SFNState.DescribeExecutionOutput, nil)
	mockSFNAPI.EXPECT().
		GetExecutionHistoryPages(gomock.Any(), gomock.Any()).
		Do(func(
			input *sfn.GetExecutionHistoryInput,
			cb func(historyOutput *sfn.GetExecutionHistoryOutput, lastPage bool) bool,
		) {
			cb(uwst.SFNState.GetExecutionHistoryOutput, true)
		})
	sfnwm := NewSFNWorkflowManager(mockSFNAPI, store, "", "", "")
	require.Nil(t, store.SaveWorkflow(uwst.Input.Workflow))
	err := sfnwm.UpdateWorkflowStatus(&uwst.Input.Workflow)
	require.Equal(t, uwst.Output.Error, err)
	updatedWorkflow, err := store.GetWorkflowByID(uwst.Input.Workflow.ID)
	require.Nil(t, err)
	require.Equal(t, uwst.Output.Workflow.Jobs, updatedWorkflow.Jobs)
	// TODO more assertions on workflow state beyond jobs array
}

func describeExecution(jason string) *sfn.DescribeExecutionOutput {
	var output sfn.DescribeExecutionOutput
	_ = jsonutil.UnmarshalJSON(&output, strings.NewReader(jason))
	//fmt.Printf("--- sfn.DescribeExecutionOutput ---\n%#v\n------\n", output)
	return &output
}

func getExecutionHistory(jason string) *sfn.GetExecutionHistoryOutput {
	var output sfn.GetExecutionHistoryOutput
	_ = jsonutil.UnmarshalJSON(&output, strings.NewReader(jason))
	//fmt.Printf("--- sfn.GetExecutionHistoryOutput ---\n%#v\n------\n", output)
	return &output
}

func workflow(jason string) models.Workflow {
	var output models.Workflow
	_ = json.Unmarshal([]byte(jason), &output)
	return output
}

func TestUpdateWorkflowStatusNew(t *testing.T) {
	t.Run("fills attempts array when activity is retried", func(t *testing.T) {
		testWorkflowOutput := workflow(`{
	"createdAt": "2017-10-09T22:31:02.497Z",
	"id": "3956fc73-d95a-43f6-9984-22f5e5f07ffd",
	"input": "{}",
	"jobs": [
		{
			"attempts": [
				{
					"reason": "sfncli.CommandTerminated",
					"startedAt": "2017-10-09T22:31:02.000Z",
					"stoppedAt": "2017-10-09T22:31:20.000Z",
					"taskARN": "ubuntu-xenial"
				}
			],
			"container": "ubuntu-xenial",
			"createdAt": "2017-10-09T22:31:30.000Z",
			"id": "6",
			"input": "{\"_EXECUTION_NAME\":\"3956fc73-d95a-43f6-9984-22f5e5f07ffd\"}",
			"output": "{\"_EXECUTION_NAME\":\"3956fc73-d95a-43f6-9984-22f5e5f07ffd\",\"done\":true}",
			"startedAt": "2017-10-09T22:31:30.000Z",
			"state": "start",
			"stateResource": {
				"lastUpdated": "2017-10-09T22:31:30.000Z",
				"name": "cil-reliability-dashboard",
				"namespace": "raf",
				"type": "ActivityARN"
			},
			"status": "succeeded",
			"stoppedAt": "2017-10-09T22:33:13.000Z"
		}
	],
	"lastUpdated": "2017-10-10T23:13:35.396Z",
	"namespace": "raf",
	"queue": "development",
	"retries": null,
	"status": "succeeded",
	"workflowDefinition": {
		"createdAt": "2017-10-02T22:20:48.497Z",
		"id": "872ea0df-0a09-4118-868d-bc2324af8078",
		"manager": "step-functions",
		"name": "cil-reliability-dashboard:master",
		"stateMachine": {
			"StartAt": "start",
			"States": {
				"start": {
					"End": true,
					"Resource": "cil-reliability-dashboard",
					"Retry": null,
					"Type": "Task"
				}
			},
			"Version": "1.0"
		},
		"version": 4
	}
}`)
		testWorkflowInput := deepcopy.Copy(testWorkflowOutput).(models.Workflow)
		testWorkflowInput.Status = models.WorkflowStatusRunning
		testWorkflowInput.Jobs = nil
		updateWorkflowStatusTest{
			SFNState: updateWorkflowStatusSFNState{
				DescribeExecutionOutput: describeExecution(`{
    "status": "SUCCEEDED", 
    "startDate": 1507588262.527, 
    "name": "3956fc73-d95a-43f6-9984-22f5e5f07ffd", 
    "executionArn": "arn:aws:states:us-west-2:589690932525:execution:raf--cil-reliability-dashboard-master--4--start:3956fc73-d95a-43f6-9984-22f5e5f07ffd", 
    "stateMachineArn": "arn:aws:states:us-west-2:589690932525:stateMachine:raf--cil-reliability-dashboard-master--4--start", 
    "stopDate": 1507588393.227, 
    "output": "{\"_EXECUTION_NAME\":\"3956fc73-d95a-43f6-9984-22f5e5f07ffd\",\"done\":true}", 
    "input": "{\"_EXECUTION_NAME\":\"3956fc73-d95a-43f6-9984-22f5e5f07ffd\"}"
}`),
				GetExecutionHistoryOutput: getExecutionHistory(`{
    "events": [
        {
            "timestamp": 1507588262.527, 
            "executionStartedEventDetails": {
                "input": "{\"_EXECUTION_NAME\":\"3956fc73-d95a-43f6-9984-22f5e5f07ffd\"}", 
                "roleArn": "arn:aws:iam::589690932525:role/raf-test-step-functions"
            }, 
            "type": "ExecutionStarted", 
            "id": 1, 
            "previousEventId": 0
        }, 
        {
            "timestamp": 1507588262.56, 
            "type": "TaskStateEntered", 
            "id": 2, 
            "stateEnteredEventDetails": {
                "input": "{\"_EXECUTION_NAME\":\"3956fc73-d95a-43f6-9984-22f5e5f07ffd\"}", 
                "name": "start"
            }, 
            "previousEventId": 0
        }, 
        {
            "timestamp": 1507588262.56, 
            "activityScheduledEventDetails": {
                "input": "{\"_EXECUTION_NAME\":\"3956fc73-d95a-43f6-9984-22f5e5f07ffd\"}", 
                "resource": "arn:aws:states:us-west-2:589690932525:activity:raf--cil-reliability-dashboard"
            }, 
            "type": "ActivityScheduled", 
            "id": 3, 
            "previousEventId": 2
        }, 
        {
            "timestamp": 1507588262.606, 
            "activityStartedEventDetails": {
                "workerName": "ubuntu-xenial"
            }, 
            "type": "ActivityStarted", 
            "id": 4, 
            "previousEventId": 3
        }, 
        {
            "activityFailedEventDetails": {
                "cause": "", 
                "error": "sfncli.CommandTerminated"
            }, 
            "timestamp": 1507588280.74, 
            "type": "ActivityFailed", 
            "id": 5, 
            "previousEventId": 4
        }, 
        {
            "timestamp": 1507588290.744, 
            "activityScheduledEventDetails": {
                "input": "{\"_EXECUTION_NAME\":\"3956fc73-d95a-43f6-9984-22f5e5f07ffd\"}", 
                "resource": "arn:aws:states:us-west-2:589690932525:activity:raf--cil-reliability-dashboard"
            }, 
            "type": "ActivityScheduled", 
            "id": 6, 
            "previousEventId": 5
        }, 
        {
            "timestamp": 1507588292.731, 
            "activityStartedEventDetails": {
                "workerName": "ubuntu-xenial"
            }, 
            "type": "ActivityStarted", 
            "id": 7, 
            "previousEventId": 6
        }, 
        {
            "activitySucceededEventDetails": {
                "output": "{\"_EXECUTION_NAME\":\"3956fc73-d95a-43f6-9984-22f5e5f07ffd\",\"done\":true}"
            }, 
            "timestamp": 1507588393.227, 
            "type": "ActivitySucceeded", 
            "id": 8, 
            "previousEventId": 7
        }, 
        {
            "timestamp": 1507588393.227, 
            "stateExitedEventDetails": {
                "output": "{\"_EXECUTION_NAME\":\"3956fc73-d95a-43f6-9984-22f5e5f07ffd\",\"done\":true}", 
                "name": "start"
            }, 
            "type": "TaskStateExited", 
            "id": 9, 
            "previousEventId": 8
        }, 
        {
            "executionSucceededEventDetails": {
                "output": "{\"_EXECUTION_NAME\":\"3956fc73-d95a-43f6-9984-22f5e5f07ffd\",\"done\":true}"
            }, 
            "timestamp": 1507588393.227, 
            "type": "ExecutionSucceeded", 
            "id": 10, 
            "previousEventId": 9
        }
    ]
}`),
			},
			Input: updateWorkflowStatusInput{
				Workflow: testWorkflowInput,
			},
			Output: updateWorkflowStatusOutput{
				Workflow: testWorkflowOutput,
				Error:    nil,
			},
		}.Run(t)
	})
}
