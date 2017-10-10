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
	"github.com/aws/aws-sdk-go/service/cloudwatch"
	"github.com/aws/aws-sdk-go/service/cloudwatch/cloudwatchiface"
	"github.com/aws/aws-sdk-go/service/sfn"
	"github.com/aws/aws-sdk-go/service/sfn/sfniface"
	"github.com/go-openapi/strfmt"
	"github.com/go-openapi/swag"
	"github.com/mohae/deepcopy"
	"gopkg.in/Clever/kayvee-go.v6/logger"
)

const (
	maxFailureReasonLines = 3
)

const sfncliCommandTerminated = "sfncli.CommandTerminated"

var defaultSFNCLICommandTerminatedRetrier = &models.SLRetrier{
	BackoffRate:     1.0,
	ErrorEquals:     []models.SLErrorEquals{sfncliCommandTerminated},
	IntervalSeconds: 10,
	MaxAttempts:     swag.Int64(10),
}

// SFNWorkflowManager manages workflows run through AWS Step Functions.
type SFNWorkflowManager struct {
	sfnapi    sfniface.SFNAPI
	cwapi     cloudwatchiface.CloudWatchAPI
	store     store.Store
	region    string
	roleARN   string
	accountID string
}

func NewSFNWorkflowManager(sfnapi sfniface.SFNAPI,
	cwapi cloudwatchiface.CloudWatchAPI,
	store store.Store,
	roleARN, region,
	accountID string) *SFNWorkflowManager {

	return &SFNWorkflowManager{
		sfnapi:    sfnapi,
		cwapi:     cwapi,
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
// Our workflow definitions contain state machine definitions with short-hand for resource names, e.g. "Resource": "name-of-worker"
// Convert this shorthand into a new state machine with full activity ARNs, e.g. "Resource": "arn:aws:states:us-west-2:589690932525:activity:production--name-of-worker"
func stateMachineWithFullActivityARNs(oldSM models.SLStateMachine, region, accountID, namespace string) *models.SLStateMachine {
	sm := deepcopy.Copy(oldSM).(models.SLStateMachine)
	for stateName, s := range sm.States {
		state := deepcopy.Copy(s).(models.SLState)
		if state.Type != models.SLStateTypeTask {
			continue
		}
		state.Resource = wdResourceToSLResource(state.Resource, region, accountID, namespace)
		sm.States[stateName] = state
	}
	return &sm
}

// stateMachineWithDefaultRetriers creates a new state machine that has the following retry properties on every state's retry array:
// - non-nil. The default serialization of a nil slice is "null", which the AWS API dislikes.
// - sfncli.CommandTerminated for any Task state. See sfncli: https://github.com/clever/sfncli.
//   This is to ensure that states are retried on signaled termination of activities (e.g. deploys).
func stateMachineWithDefaultRetriers(oldSM models.SLStateMachine) *models.SLStateMachine {
	sm := deepcopy.Copy(oldSM).(models.SLStateMachine)
	for stateName, s := range sm.States {
		state := deepcopy.Copy(s).(models.SLState)
		if state.Retry == nil {
			state.Retry = []*models.SLRetrier{}
		}
		injectRetry := true
		for _, retry := range state.Retry {
			for _, errorEquals := range retry.ErrorEquals {
				if errorEquals == sfncliCommandTerminated {
					injectRetry = false
					break
				}
			}
		}
		if injectRetry && state.Type == models.SLStateTypeTask {
			state.Retry = append(state.Retry, defaultSFNCLICommandTerminatedRetrier)
		}
		sm.States[stateName] = state
	}
	return &sm
}

// https://docs.aws.amazon.com/step-functions/latest/apireference/API_CreateStateMachine.html#StepFunctions-CreateStateMachine-request-name
var stateMachineNameBadChars = []byte{' ', '<', '>', '{', '}', '[', ']', '?', '*', '"', '#', '%', '\\', '^', '|', '~', '`', '$', '&', ',', ';', ':', '/'}

func stateMachineName(wdName string, wdVersion int64, namespace string, startAt string) string {
	name := fmt.Sprintf("%s--%s--%d--%s", namespace, wdName, wdVersion, startAt)
	for _, badchar := range stateMachineNameBadChars {
		name = strings.Replace(name, string(badchar), "-", -1)
	}
	return name
}

func stateMachineARN(region, accountID, wdName string, wdVersion int64, namespace string, startAt string) string {
	return fmt.Sprintf("arn:aws:states:%s:%s:stateMachine:%s", region, accountID, stateMachineName(wdName, wdVersion, namespace, startAt))
}

func (wm *SFNWorkflowManager) describeOrCreateStateMachine(wd models.WorkflowDefinition, namespace, queue string) (*sfn.DescribeStateMachineOutput, error) {
	describeOutput, err := wm.sfnapi.DescribeStateMachine(&sfn.DescribeStateMachineInput{
		StateMachineArn: aws.String(stateMachineARN(wm.region, wm.accountID, wd.Name, wd.Version, namespace, wd.StateMachine.StartAt)),
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
	awsStateMachine := stateMachineWithFullActivityARNs(*wd.StateMachine, wm.region, wm.accountID, namespace)
	awsStateMachine = stateMachineWithDefaultRetriers(*awsStateMachine)
	awsStateMachineDefBytes, err := json.MarshalIndent(awsStateMachine, "", "  ")
	if err != nil {
		return nil, err
	}
	awsStateMachineDef := string(awsStateMachineDefBytes)
	// the name must be unique. Use workflow definition name + version + namespace + queue to uniquely identify a state machine
	// this effectively creates a new workflow definition in each namespace we deploy into
	awsStateMachineName := stateMachineName(wd.Name, wd.Version, namespace, wd.StateMachine.StartAt)
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

func (wm *SFNWorkflowManager) startExecution(stateMachineArn *string, workflowID, input string) error {
	executionName := aws.String(workflowID)

	var inputJSON map[string]interface{}
	if err := json.Unmarshal([]byte(input), &inputJSON); err != nil {
		return fmt.Errorf("input is not a valid JSON object: %s", err)
	}
	inputJSON["_EXECUTION_NAME"] = *executionName

	marshaledInput, err := json.Marshal(inputJSON)
	if err != nil {
		return err
	}

	// We ensure the input is a json object, but for context the
	// observed StartExecution behavior for different edge-case inputs:
	// - nil: AWS converts this to an input of an empty object "{}"
	// - aws.String(""): leads to InvalidExecutionInput AWS error
	// - aws.String("[]"): leads to an input of an empty array "[]"
	startExecutionInput := aws.String(string(marshaledInput))
	_, err = wm.sfnapi.StartExecution(&sfn.StartExecutionInput{
		StateMachineArn: stateMachineArn,
		Input:           startExecutionInput,
		Name:            executionName,
	})

	return err
}

func (wm *SFNWorkflowManager) CreateWorkflow(wd models.WorkflowDefinition, input string, namespace string, queue string, tags map[string]interface{}) (*models.Workflow, error) {
	describeOutput, err := wm.describeOrCreateStateMachine(wd, namespace, queue)
	if err != nil {
		return nil, err
	}

	// save the workflow before starting execution to ensure we don't have untracked executions
	// i.e. execution was started but we failed to save workflow
	// If we fail starting the execution, we can resolve this out of band (TODO: should support cancelling)
	workflow := resources.NewWorkflow(&wd, input, namespace, queue, tags)
	if err := wm.store.SaveWorkflow(*workflow); err != nil {
		return nil, err
	}

	// submit an execution using input, set execution name == our workflow GUID
	err = wm.startExecution(describeOutput.StateMachineArn, workflow.ID, input)
	if err != nil {
		return nil, err
	}

	return workflow, nil
}

func (wm *SFNWorkflowManager) RetryWorkflow(ogWorkflow models.Workflow, startAt, input string) (*models.Workflow, error) {
	// don't allow resume if workflow is still active
	if !resources.WorkflowIsDone(&ogWorkflow) {
		return nil, fmt.Errorf("Workflow %s active: %s", ogWorkflow.ID, ogWorkflow.Status)
	}

	// modify the StateMachine with the custom StartState by making a new WorkflowDefinition (no pointer copy)
	newDef := resources.CopyWorkflowDefinition(*ogWorkflow.WorkflowDefinition)
	newDef.StateMachine.StartAt = startAt
	if err := resources.RemoveInactiveStates(newDef.StateMachine); err != nil {
		return nil, err
	}

	describeOutput, err := wm.describeOrCreateStateMachine(newDef, ogWorkflow.Namespace, ogWorkflow.Queue)
	if err != nil {
		return nil, err
	}

	workflow := resources.NewWorkflow(&newDef, input, ogWorkflow.Namespace, ogWorkflow.Queue, ogWorkflow.Tags)
	workflow.RetryFor = ogWorkflow.ID
	ogWorkflow.Retries = append(ogWorkflow.Retries, workflow.ID)

	// save the workflow before starting execution to ensure we don't have untracked executions
	// If we fail starting the execution, we can resolve this out of band (TODO: should support cancelling)
	if err = wm.store.SaveWorkflow(*workflow); err != nil {
		return nil, err
	}
	// also update ogWorkflow
	if err = wm.store.UpdateWorkflow(ogWorkflow); err != nil {
		return nil, err
	}

	// submit an execution using input, set execution name == our workflow GUID
	err = wm.startExecution(describeOutput.StateMachineArn, workflow.ID, input)
	if err != nil {
		return nil, err
	}

	return workflow, nil
}

func (wm *SFNWorkflowManager) CancelWorkflow(workflow *models.Workflow, reason string) error {
	// cancel execution
	wd := workflow.WorkflowDefinition
	execARN := wm.executionARN(workflow, wd)
	_, err := wm.sfnapi.StopExecution(&sfn.StopExecutionInput{
		ExecutionArn: aws.String(execARN),
		Cause:        aws.String(reason),
		// Error: aws.String(""), // TODO: Can we use this? "An arbitrary error code that identifies the cause of the termination."
	})
	if err != nil {
		return err
	}

	describeOutput, err := wm.sfnapi.DescribeExecution(&sfn.DescribeExecutionInput{
		ExecutionArn: aws.String(execARN),
	})
	if err != nil {
		return err
	}

	status := sfnStatusToWorkflowStatus(*describeOutput.Status)
	if status == models.WorkflowStatusFailed {
		// SFN executions can't be moved from the failed state to the aborted state, so if the user
		// cancels a failed workflow, we update the status in workflow-manager directly to preserve
		// user intent.
		workflow.Status = models.WorkflowStatusCancelled
	}

	workflow.StatusReason = reason
	return wm.store.UpdateWorkflow(*workflow)
}

func (wm *SFNWorkflowManager) executionARN(
	workflow *models.Workflow,
	definition *models.WorkflowDefinition,
) string {
	return executionARN(
		wm.region,
		wm.accountID,
		stateMachineName(definition.Name, definition.Version, workflow.Namespace, definition.StateMachine.StartAt),
		workflow.ID,
	)
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
	// Avoid the extraneous processing for executions that have already stopped.
	// This also prevents the WM "cancelled" state from getting overwritten for workflows cancelled
	// by the user after a failure.
	if resources.WorkflowIsDone(workflow) {
		return nil
	}

	// get execution from AWS, pull in all the data into the workflow object
	wd := workflow.WorkflowDefinition
	execARN := executionARN(
		wm.region,
		wm.accountID,
		stateMachineName(wd.Name, wd.Version, workflow.Namespace, wd.StateMachine.StartAt),
		workflow.ID,
	)
	describeOutput, err := wm.sfnapi.DescribeExecution(&sfn.DescribeExecutionInput{
		ExecutionArn: aws.String(execARN),
	})
	if err != nil {
		log.ErrorD("describe-execution", logger.M{"workflow-id": workflow.ID, "error": err.Error()})
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case sfn.ErrCodeExecutionDoesNotExist:
				workflow.LastUpdated = strfmt.DateTime(time.Now())
				workflow.Status = models.WorkflowStatusFailed
				return wm.store.UpdateWorkflow(*workflow)
			}
		}
		return err
	}

	workflow.LastUpdated = strfmt.DateTime(time.Now())
	workflow.Status = sfnStatusToWorkflowStatus(*describeOutput.Status)
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
				input := aws.StringValue(stateEntered.Input)
				state, ok := workflow.WorkflowDefinition.StateMachine.States[*stateEntered.Name]
				var stateResourceName string
				if ok {
					stateResourceName = state.Resource
				}
				job := &models.Job{
					CreatedAt: strfmt.DateTime(aws.TimeValue(evt.Timestamp)),
					StartedAt: strfmt.DateTime(aws.TimeValue(evt.Timestamp)),
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
						LastUpdated: strfmt.DateTime(aws.TimeValue(evt.Timestamp)),
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
					stateToJob[*currentState].StoppedAt = strfmt.DateTime(aws.TimeValue(evt.Timestamp))
					if details := evt.ActivityFailedEventDetails; details != nil {
						reasonLines := strings.Split(strings.TrimSpace(aws.StringValue(details.Cause)), "\n")
						if len(reasonLines) > maxFailureReasonLines {
							reasonLines = reasonLines[len(reasonLines)-maxFailureReasonLines:]
						}
						stateToJob[*currentState].StatusReason = strings.Join(reasonLines, "\n")
					}
				}
			case sfn.HistoryEventTypeActivitySucceeded:
				if currentState != nil {
					stateToJob[*currentState].Status = models.JobStatusSucceeded
				}
			case sfn.HistoryEventTypeExecutionAborted:
				if currentState != nil {
					job := stateToJob[*currentState]
					job.Status = models.JobStatusAbortedByUser
					job.StoppedAt = strfmt.DateTime(aws.TimeValue(evt.Timestamp))
					if details := evt.ExecutionAbortedEventDetails; details != nil {
						job.StatusReason = aws.StringValue(details.Cause)
					}
				}
			case sfn.HistoryEventTypeExecutionFailed:
				if currentState != nil {
					job := stateToJob[*currentState]
					job.Status = models.JobStatusFailed
					job.StoppedAt = strfmt.DateTime(*evt.Timestamp)

					if details := evt.ExecutionFailedEventDetails; details != nil {
						if isActivityDoesntExistFailure(evt.ExecutionFailedEventDetails) {
							job.StatusReason = "State resource does not exist"
						} else {
							// set unknown errors to StatusReason
							job.StatusReason = fmt.Sprintf("%s: %s", aws.StringValue(details.Error),
								aws.StringValue(details.Cause))
						}
					}
				}
			case sfn.HistoryEventTypeTaskStateExited:
				stateExited := evt.StateExitedEventDetails
				stateToJob[*stateExited.Name].StoppedAt = strfmt.DateTime(aws.TimeValue(evt.Timestamp))
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

// StateResourcesStatus fetches the status for all state resources given a workflow definition and namespace from Cloudwatch
func (wm *SFNWorkflowManager) StateResourcesStatus(def models.WorkflowDefinition, namespace string) ([]models.StateResource, error) {
	stateResources := []models.StateResource{}

	for _, state := range def.StateMachine.States {
		stateResource := resources.NewSFNResource(state.Resource, namespace,
			wdResourceToSLResource(state.Resource, wm.region, wm.accountID, namespace))

		status, err := wm.getStatus(stateResource.URI)
		if err != nil {
			// TODO: maybe don't fail if one metric fails?
			return stateResources, err
		}
		stateResource.Status = &status
		stateResources = append(stateResources, stateResource)
	}

	return stateResources, nil
}

// isActivityDoesntExistFailure checks if an execution failed because an activity doesn't exist.
// This currently results in a cryptic AWS error, so the logic is probably over-broad: https://console.aws.amazon.com/support/home?region=us-west-2#/case/?displayId=4514731511&language=en
// If SFN creates a more descriptive error event we should change this.
func isActivityDoesntExistFailure(details *sfn.ExecutionFailedEventDetails) bool {
	return aws.StringValue(details.Error) == "States.Runtime" &&
		strings.Contains(aws.StringValue(details.Cause), "Internal Error")
}

// getStatus fetches the queued and running activities given an activityArn
func (wm *SFNWorkflowManager) getStatus(activityArn string) (models.StateResourceStatus, error) {
	stateResourceStatus := models.StateResourceStatus{}
	activityDimension := &cloudwatch.Dimension{
		Name:  aws.String("ActivityArn"),
		Value: aws.String(activityArn),
	}
	metrics := map[string]int64{
		"ActivitiesStarted":   0,
		"ActivitiesScheduled": 0,
	}
	for metricName, _ := range metrics {
		res, err := wm.cwapi.GetMetricStatistics(&cloudwatch.GetMetricStatisticsInput{
			Dimensions: []*cloudwatch.Dimension{activityDimension},
			StartTime:  aws.Time(time.Now().Add(-4 * time.Minute)),
			EndTime:    aws.Time(time.Now()),
			MetricName: aws.String(metricName),
			Namespace:  aws.String("AWS/States"),
			Period:     aws.Int64(60), // this metric is available at a minimum of 1 minute
			Statistics: []*string{aws.String("Sum")},
		})
		if err != nil {
			return stateResourceStatus, err
		}
		if len(res.Datapoints) > 0 {
			datapoint := res.Datapoints[len(res.Datapoints)-1]
			metrics[metricName] = int64(*datapoint.Sum)
			// for now don't care about which datapoint's timestamp we store
			stateResourceStatus.LastUpdated = strfmt.DateTime(*datapoint.Timestamp)
		}
	}

	stateResourceStatus.Running = metrics["ActivitiesStarted"]
	stateResourceStatus.Queued = metrics["ActivitiesScheduled"] - metrics["ActivitiesStarted"]

	return stateResourceStatus, nil
}
