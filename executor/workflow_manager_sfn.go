package executor

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Clever/workflow-manager/gen-go/models"
	"github.com/Clever/workflow-manager/resources"
	"github.com/Clever/workflow-manager/store"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/sfn"
	"github.com/aws/aws-sdk-go/service/sfn/sfniface"
	"github.com/go-openapi/strfmt"
	"gopkg.in/Clever/kayvee-go.v6/logger"
)

// SFNWorkflowManager manages workflows run through AWS Step Functions.
type SFNWorkflowManager struct {
	sfnapi    sfniface.SFNAPI
	store     store.Store
	region    string
	roleARN   string
	accountID string
}

func NewSFNWorkflowManager(sfnapi sfniface.SFNAPI, store store.Store, roleARN, region, accountID string) WorkflowManager {
	return &SFNWorkflowManager{
		sfnapi:    sfnapi,
		store:     store,
		roleARN:   roleARN,
		region:    region,
		accountID: accountID,
	}
}

func wdResourceToSLResource(wdResource, region, accountID, namespace string) string {
	return fmt.Sprintf("arn:aws:states:%s:%s:activity:%s--%s", region, accountID, namespace, wdResource)
}

// stateMachineWithFullActivityARNs converts resource names in states to full activity ARNs. It returns a new state machine.
func stateMachineWithFullActivityARNs(stateMachine *models.SLStateMachine, region, accountID, namespace string) *models.SLStateMachine {

	newStates := map[string]models.SLState{}
	for stateName, state := range stateMachine.States {
		// declare the var in order to generate a compile error if the copy below ever becomes a pointer copy
		var newState models.SLState
		newState = state // copy
		newState.Resource = wdResourceToSLResource(state.Resource, region, accountID, namespace)
		if newState.Retry == nil {
			newState.Retry = []*models.SLRetrier{}
		}
		newStates[stateName] = newState
	}

	// make a copy (don't pointer copy!) of state machine replacing states map with modified states map
	newStateMachine := *stateMachine // copy
	newStateMachine.States = newStates

	return &newStateMachine
}

// https://docs.aws.amazon.com/step-functions/latest/apireference/API_CreateStateMachine.html#StepFunctions-CreateStateMachine-request-name
var stateMachineNameBadChars = []byte{' ', '<', '>', '{', '}', '[', ']', '?', '*', '"', '#', '%', '\\', '^', '|', '~', '`', '$', '&', ',', ';', ':', '/'}

func stateMachineName(wdName string, wdVersion int64, namespace string, queue string) string {
	name := fmt.Sprintf("%s--%s--%d--%s", namespace, wdName, wdVersion, queue)
	for _, badchar := range stateMachineNameBadChars {
		name = strings.Replace(name, string(badchar), "-", -1)
	}
	return name
}

func stateMachineARN(region, accountID, wdName string, wdVersion int64, namespace string, queue string) string {
	return fmt.Sprintf("arn:aws:states:%s:%s:stateMachine:%s", region, accountID, stateMachineName(wdName, wdVersion, namespace, queue))
}

func (wm *SFNWorkflowManager) describeOrCreateStateMachine(wd models.WorkflowDefinition, namespace, queue string) (*sfn.DescribeStateMachineOutput, error) {
	describeOutput, err := wm.sfnapi.DescribeStateMachine(&sfn.DescribeStateMachineInput{
		StateMachineArn: aws.String(stateMachineARN(wm.region, wm.accountID, wd.Name, wd.Version, namespace, queue)),
	})
	if err == nil {
		return describeOutput, nil
	}
	awserr, ok := err.(awserr.Error)
	if !ok {
		return nil, fmt.Errorf("non-AWS error in findOrCreateStateMachine: %s", err)
	}
	if awserr.Code() != sfn.ErrCodeStateMachineDoesNotExist {
		return nil, fmt.Errorf("unexpected AWS error in findOrCreateStateMachine: %s", awserr)
	}

	// state machine doesn't exist, create it
	// our state machine states don't contain full activity ARNs as resources
	// instead we use shorthand that is convenient when writing out state machine definitions, e.g. "Resource": "name-of-worker"
	// convert this shorthand into a new state machine with full activity ARNs, e.g. "Resource": "arn:aws:states:us-west-2:589690932525:activity:production--name-of-worker"
	awsStateMachine := stateMachineWithFullActivityARNs(wd.StateMachine, wm.region, wm.accountID, namespace)
	awsStateMachineDefBytes, err := json.MarshalIndent(awsStateMachine, "", "  ")
	if err != nil {
		return nil, err
	}
	awsStateMachineDef := string(awsStateMachineDefBytes)
	// the name must be unique. Use workflow definition name + version + namespace + queue to uniquely identify a state machine
	// this effectively creates named queues for each workflow definition in each namespace we deploy into
	awsStateMachineName := stateMachineName(wd.Name, wd.Version, namespace, queue)
	log.InfoD("create-state-machine", logger.M{"definition": awsStateMachineDef, "name": awsStateMachineName})
	_, err = wm.sfnapi.CreateStateMachine(&sfn.CreateStateMachineInput{
		Name:       aws.String(awsStateMachineName),
		Definition: aws.String(awsStateMachineDef),
		RoleArn:    aws.String(wm.roleARN),
	})
	if err != nil {
		return nil, fmt.Errorf("CreateStateMachine error: %s", err.Error())
	}

	return wm.describeOrCreateStateMachine(wd, namespace, queue)
}

func (wm *SFNWorkflowManager) CreateWorkflow(wd models.WorkflowDefinition, input string, namespace string, queue string, tags map[string]interface{}) (*models.Workflow, error) {
	describeOutput, err := wm.describeOrCreateStateMachine(wd, namespace, queue)
	if err != nil {
		return nil, err
	}

	// submit an execution using input, set execution name == our workflow GUID
	workflow := resources.NewWorkflow(&wd, input, namespace, queue, tags)
	// observed StartExecution behavior for different edge-case inputs:
	// - nil: AWS converts this to an input of an empty object "{}"
	// - aws.String(""): leads to InvalidExecutionInput AWS error
	// - aws.String("[]"): leads to an input of an empty array "[]"
	var startExecutionInput *string
	if len(input) > 0 {
		var inputJSON interface{}
		if err := json.Unmarshal([]byte(input), &inputJSON); err != nil {
			return nil, fmt.Errorf("input is not valid JSON: %s", err)
		}
		startExecutionInput = aws.String(input)
	}
	_, err = wm.sfnapi.StartExecution(&sfn.StartExecutionInput{
		StateMachineArn: describeOutput.StateMachineArn,
		Input:           startExecutionInput,
		Name:            aws.String(workflow.ID),
	})
	if err != nil {
		return nil, err
	}

	return workflow, wm.store.SaveWorkflow(*workflow)
}

func (wm *SFNWorkflowManager) CancelWorkflow(workflow *models.Workflow, reason string) error {
	// cancel execution
	wd := workflow.WorkflowDefinition
	execARN := executionARN(wm.region, wm.accountID, stateMachineName(wd.Name, wd.Version, workflow.Namespace, workflow.Queue), workflow.ID)
	_, err := wm.sfnapi.StopExecution(&sfn.StopExecutionInput{
		ExecutionArn: aws.String(execARN),
		Cause:        aws.String(reason),
		// Error: aws.String(""), // TODO: Can we use this? "An arbitrary error code that identifies the cause of the termination."
	})
	if err != nil {
		return err
	}

	workflow.Status = models.WorkflowStatusCancelled
	return wm.store.UpdateWorkflow(*workflow)
}

func executionARN(region, accountID, stateMachineName, executionName string) string {
	return fmt.Sprintf("arn:aws:states:%s:%s:execution:%s:%s", region, accountID, stateMachineName, executionName)
}

func sfnStatusToWorkflowStatus(sfnStatus string) models.WorkflowStatus {
	switch sfnStatus {
	case sfn.ExecutionStatusRunning:
		return models.WorkflowStatusRunning
	case sfn.ExecutionStatusSucceeded:
		return models.WorkflowStatusSucceeded
	case sfn.ExecutionStatusFailed:
		return models.WorkflowStatusFailed
	case sfn.ExecutionStatusTimedOut:
		return models.WorkflowStatusFailed
	case sfn.ExecutionStatusAborted:
		return models.WorkflowStatusCancelled
	default:
		return models.WorkflowStatusQueued // this should never happen, since all cases are covered above
	}
}

func (wm *SFNWorkflowManager) UpdateWorkflowStatus(workflow *models.Workflow) error {
	// get execution from AWS, pull in all the data into the workflow object
	wd := workflow.WorkflowDefinition
	execARN := executionARN(wm.region, wm.accountID, stateMachineName(wd.Name, wd.Version, workflow.Namespace, workflow.Queue), workflow.ID)
	describeOutput, err := wm.sfnapi.DescribeExecution(&sfn.DescribeExecutionInput{
		ExecutionArn: aws.String(execARN),
	})
	if err != nil {
		return err
	}

	workflow.LastUpdated = strfmt.DateTime(time.Now())
	previousStatus := workflow.Status
	workflow.Status = sfnStatusToWorkflowStatus(*describeOutput.Status)
	if previousStatus == models.WorkflowStatusCancelled {
		workflow.Status = previousStatus
	}
	// TODO: we should capture the output of a workflow
	//workflow.Output = describeStatus.Output

	// Pull in execution history to populate jobs array
	// Here we will begin to break the assumption that jobs are 1:1 with states, which is true in Batch
	// since (1) it doesn't support complex branching, and (2) we schedule all jobs for a workflow at
	// the time of  workflow submission.
	// Step Functions supports complex branching, so jobs might not be 1:1 with states.
	jobs := []*models.Job{}
	stateToJob := map[string]*models.Job{}
	// TODO: as soon as we have parallel states anything that relies on this might be incorrect
	var currentState *string
	if err := wm.sfnapi.GetExecutionHistoryPages(&sfn.GetExecutionHistoryInput{
		ExecutionArn: aws.String(execARN),
	}, func(historyOutput *sfn.GetExecutionHistoryOutput, lastPage bool) bool {
		// NOTE: if pulling the entire execution history becomes infeasible, we can:
		// 1) limit the results with `maxResults`
		// 2) set `reverseOrder` to true to get most recent events first
		// 3) store the last event we processed, and stop paging once we reach it
		for _, evt := range historyOutput.Events {
			switch *evt.Type {
			case sfn.HistoryEventTypeTaskStateEntered:
				stateEntered := evt.StateEnteredEventDetails
				var input string
				json.Unmarshal([]byte(*stateEntered.Input), &input)
				state, ok := workflow.WorkflowDefinition.StateMachine.States[*stateEntered.Name]
				var stateResourceName string
				if ok {
					stateResourceName = state.Resource
				}
				job := &models.Job{
					CreatedAt: strfmt.DateTime(*evt.Timestamp),
					StartedAt: strfmt.DateTime(*evt.Timestamp),
					// TODO: somehow match ActivityStarted events to state, capture worker name here
					//ContainerId: ""/
					Status: models.JobStatusCreated,
					Input:  input,
					// event IDs start at 1 and are only unique to the execution, so this might not be ideal
					ID:    fmt.Sprintf("%d", *evt.Id),
					State: *stateEntered.Name,
					StateResource: &models.StateResource{
						Name:        stateResourceName,
						Type:        models.StateResourceTypeActivityARN,
						Namespace:   workflow.Namespace,
						LastUpdated: strfmt.DateTime(*evt.Timestamp),
					},
				}
				currentState = stateEntered.Name
				stateToJob[job.State] = job
				jobs = append(jobs, job)
			case sfn.HistoryEventTypeActivityScheduled:
				if currentState != nil {
					stateToJob[*currentState].Status = models.JobStatusQueued
				}
			case sfn.HistoryEventTypeActivityStarted:
				if currentState != nil {
					stateToJob[*currentState].Status = models.JobStatusRunning
				}
			case sfn.HistoryEventTypeActivityFailed:
				if currentState != nil {
					stateToJob[*currentState].Status = models.JobStatusFailed
				}
			case sfn.HistoryEventTypeActivitySucceeded:
				if currentState != nil {
					stateToJob[*currentState].Status = models.JobStatusSucceeded
				}
			case sfn.HistoryEventTypeTaskStateExited:
				stateExited := evt.StateExitedEventDetails
				stateToJob[*stateExited.Name].StoppedAt = strfmt.DateTime(*evt.Timestamp)
				if stateExited.Output != nil {
					stateToJob[*stateExited.Name].Output = *stateExited.Output
				}
				currentState = nil
			}
		}
		return true
	}); err != nil {
		return err
	}
	workflow.Jobs = jobs

	return wm.store.UpdateWorkflow(*workflow)
}
