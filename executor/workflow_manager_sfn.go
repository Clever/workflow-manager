package executor

import (
	"context"
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
	"github.com/aws/aws-sdk-go/service/sqs/sqsiface"
	"github.com/go-openapi/strfmt"
	"github.com/go-openapi/swag"
	"github.com/mohae/deepcopy"
	"gopkg.in/Clever/kayvee-go.v6/logger"
)

const (
	maxFailureReasonLines   = 3
	sfncliCommandTerminated = "sfncli.CommandTerminated"
)

var durationToRetryDescribeExecutions = 5 * time.Minute
var durationToFetchHistoryPages = time.Minute

var defaultSFNCLICommandTerminatedRetrier = &models.SLRetrier{
	BackoffRate:     1.0,
	ErrorEquals:     []models.SLErrorEquals{sfncliCommandTerminated},
	IntervalSeconds: 10,
	MaxAttempts:     swag.Int64(10),
}

// SFNWorkflowManager manages workflows run through AWS Step Functions.
type SFNWorkflowManager struct {
	sfnapi      sfniface.SFNAPI
	sqsapi      sqsiface.SQSAPI
	store       store.Store
	region      string
	roleARN     string
	accountID   string
	sqsQueueURL string
}

func NewSFNWorkflowManager(sfnapi sfniface.SFNAPI, sqsapi sqsiface.SQSAPI, store store.Store, roleARN, region, accountID, sqsQueueURL string) *SFNWorkflowManager {
	return &SFNWorkflowManager{
		sfnapi:      sfnapi,
		sqsapi:      sqsapi,
		store:       store,
		roleARN:     roleARN,
		region:      region,
		accountID:   accountID,
		sqsQueueURL: sqsQueueURL,
	}
}

func wdResourceToSLResource(wdResource, region, accountID, namespace string) string {
	return fmt.Sprintf("arn:aws:states:%s:%s:activity:%s--%s", region, accountID, namespace, wdResource)
}

func wdResourceToSLResourceLambda(wdResource, region, accountID, namespace string) string {
	return fmt.Sprintf("arn:aws:lambda:%s:%s:function:%s--%s", region, accountID, namespace, strings.TrimPrefix(wdResource, "lambda:"))
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
		if strings.HasPrefix(state.Resource, "lambda:") {
			state.Resource = wdResourceToSLResourceLambda(state.Resource, region, accountID, namespace)
		} else {
			state.Resource = wdResourceToSLResource(state.Resource, region, accountID, namespace)
		}
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
			// Add the default retry before user defined ones since Error=States.ALL must
			// always be the last in the retry list and could be defined in the workflow definition
			state.Retry = append([]*models.SLRetrier{defaultSFNCLICommandTerminatedRetrier}, state.Retry...)
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
		return models.BadRequest{
			Message: fmt.Sprintf("input is not a valid JSON object: %s", err),
		}
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

func (wm *SFNWorkflowManager) CreateWorkflow(ctx context.Context, wd models.WorkflowDefinition,
	input string,
	namespace string,
	queue string,
	tags map[string]interface{}) (*models.Workflow, error) {

	describeOutput, err := wm.describeOrCreateStateMachine(wd, namespace, queue)
	if err != nil {
		return nil, err
	}

	// save the workflow before starting execution to ensure we don't have untracked executions
	// i.e. execution was started but we failed to save workflow
	// If we fail starting the execution, we can resolve this out of band (TODO: should support cancelling)
	workflow := resources.NewWorkflow(&wd, input, namespace, queue, tags)
	if err := wm.store.SaveWorkflow(ctx, *workflow); err != nil {
		return nil, err
	}

	// submit an execution using input, set execution name == our workflow GUID
	err = wm.startExecution(describeOutput.StateMachineArn, workflow.ID, input)
	if err != nil {
		// since we failed to start execution, remove Workflow from store
		if delErr := wm.store.DeleteWorkflowByID(ctx, workflow.ID); delErr != nil {
			log.ErrorD("create-workflow", logger.M{
				"id": workflow.ID,
				"workflow-definition-name": workflow.WorkflowDefinition.Name,
				"message":                  "failed to delete stray workflow",
				"error":                    fmt.Sprintf("SFNError: %s;StoreError: %s", err, delErr),
			})
		}

		return nil, err
	}

	// start update loop for this workflow
	err = createPendingWorkflow(context.TODO(), workflow.ID, wm.sqsapi, wm.sqsQueueURL)
	if err != nil {
		return nil, err
	}

	return workflow, nil
}

func (wm *SFNWorkflowManager) RetryWorkflow(ctx context.Context, ogWorkflow models.Workflow, startAt, input string) (*models.Workflow, error) {
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
	if err = wm.store.SaveWorkflow(ctx, *workflow); err != nil {
		return nil, err
	}
	// also update ogWorkflow
	if err = wm.store.UpdateWorkflow(ctx, ogWorkflow); err != nil {
		return nil, err
	}

	// submit an execution using input, set execution name == our workflow GUID
	err = wm.startExecution(describeOutput.StateMachineArn, workflow.ID, input)
	if err != nil {
		return nil, err
	}

	// start update loop for this workflow
	err = createPendingWorkflow(context.TODO(), workflow.ID, wm.sqsapi, wm.sqsQueueURL)
	if err != nil {
		return nil, err
	}

	return workflow, nil
}

func (wm *SFNWorkflowManager) CancelWorkflow(ctx context.Context, workflow *models.Workflow, reason string) error {
	if workflow.Status == models.WorkflowStatusSucceeded || workflow.Status == models.WorkflowStatusFailed {
		return fmt.Errorf("Cancellation not allowed. Workflow %s is %s", workflow.ID, workflow.Status)
	}

	wd := workflow.WorkflowDefinition
	execARN := wm.executionARN(workflow, wd)
	if _, err := wm.sfnapi.StopExecution(&sfn.StopExecutionInput{
		ExecutionArn: aws.String(execARN),
		Cause:        aws.String(reason),
		// Error: aws.String(""), // TODO: Can we use this? "An arbitrary error code that identifies the cause of the termination."
	}); err != nil {
		return err
	}

	workflow.StatusReason = reason
	workflow.ResolvedByUser = true
	return wm.store.UpdateWorkflow(ctx, *workflow)
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

func (wm *SFNWorkflowManager) UpdateWorkflowSummary(ctx context.Context, workflow *models.Workflow) error {
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
	describeOutput, err := wm.sfnapi.DescribeExecutionWithContext(context.TODO(), &sfn.DescribeExecutionInput{
		ExecutionArn: aws.String(execARN),
	})
	if err != nil {
		log.ErrorD("describe-execution", logger.M{"workflow-id": workflow.ID, "error": err.Error()})
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case sfn.ErrCodeExecutionDoesNotExist:
				log.ErrorD("execution-not-found", logger.M{
					"workflow-id":  workflow.ID,
					"execution-id": execARN,
					"error":        err.Error()})

				// only fail after 10 minutes to see if this is an eventual-consistency thing
				if time.Time(workflow.LastUpdated).Before(time.Now().Add(-durationToRetryDescribeExecutions)) {
					workflow.LastUpdated = strfmt.DateTime(time.Now())
					workflow.Status = models.WorkflowStatusFailed
					return wm.store.UpdateWorkflow(ctx, *workflow)
				}
				// don't save since that updates worklow.LastUpdated; also no changes made here
				return nil
			}
		}
		return err
	}

	workflow.LastUpdated = strfmt.DateTime(time.Now())
	workflow.Status = sfnStatusToWorkflowStatus(*describeOutput.Status)
	if *describeOutput.Status == sfn.ExecutionStatusTimedOut {
		workflow.StatusReason = resources.StatusReasonWorkflowTimedOut
	}
	if describeOutput.StopDate != nil {
		workflow.StoppedAt = strfmt.DateTime(aws.TimeValue(describeOutput.StopDate))
	}
	if workflow.Status == models.WorkflowStatusSucceeded {
		workflow.ResolvedByUser = true
	}

	workflow.Output = aws.StringValue(describeOutput.Output) // use for error or success  (TODO: actually this is only sent for success)
	return wm.store.UpdateWorkflow(ctx, *workflow)
}

func (wm *SFNWorkflowManager) UpdateWorkflowHistory(ctx context.Context, workflow *models.Workflow) error {
	// Pull in execution history to populate jobs array
	// Each Job corresponds to a type={Task,Choice,Succeed} state, i.e. States we have currently tested and supported completely
	// We only create a Job object if the State has been entered.
	// Execution history events contain a "previous" event ID which is the "parent" event within the execution tree.
	// E.g., if a state machine has two parallel Task states, the events for these states will overlap in the history, but the event IDs + previous event IDs will link together the parallel execution paths.
	// In order to correctly associate events with the job they correspond to, maintain a map from event ID to job.
	wd := workflow.WorkflowSummary.WorkflowDefinition
	execARN := executionARN(
		wm.region,
		wm.accountID,
		stateMachineName(wd.Name, wd.Version, workflow.Namespace, wd.StateMachine.StartAt),
		workflow.ID,
	)
	jobs := []*models.Job{}
	eventIDToJob := map[int64]*models.Job{}
	eventToJob := func(evt *sfn.HistoryEvent) *models.Job {
		eventID := aws.Int64Value(evt.Id)
		parentEventID := aws.Int64Value(evt.PreviousEventId)
		switch *evt.Type {
		case sfn.HistoryEventTypeExecutionStarted:
			// very first event for an execution, so there are no jobs yet
			return nil
		case sfn.HistoryEventTypePassStateEntered, sfn.HistoryEventTypePassStateExited,
			sfn.HistoryEventTypeParallelStateEntered, sfn.HistoryEventTypeParallelStateExited,
			sfn.HistoryEventTypeWaitStateEntered, sfn.HistoryEventTypeWaitStateExited,
			sfn.HistoryEventTypeFailStateEntered:
			// only create Jobs for Task, Choice and Succeed states
			return nil
		case sfn.HistoryEventTypeTaskStateEntered, sfn.HistoryEventTypeChoiceStateEntered, sfn.HistoryEventTypeSucceedStateEntered:
			// a job is created when a supported state is entered
			job := &models.Job{}
			jobs = append(jobs, job)
			eventIDToJob[eventID] = job
			return job
		case sfn.HistoryEventTypeExecutionAborted:
			// Execution-level event - update last seen job.
			return jobs[len(jobs)-1]
		case sfn.HistoryEventTypeExecutionFailed:
			// Execution-level event - update last seen job.
			return jobs[len(jobs)-1]
		case sfn.HistoryEventTypeExecutionTimedOut:
			// Execution-level event - update last seen job.
			return jobs[len(jobs)-1]
		default:
			// associate this event with the same job as its parent event
			job, ok := eventIDToJob[parentEventID]
			if !ok {
				// we should investigate these cases, since it means we have a gap in our interpretation of the event history
				log.ErrorD("event-with-unknown-job", logger.M{"event-id": eventID, "execution-arn": execARN})
				return nil
			}
			eventIDToJob[eventID] = job
			return job
		}
	}

	// Setup a context with a timeout of one minute since
	// we don't want to pull very large workflow histories
	// TODO: this should be a context passed by the handler
	ctx, cancel := context.WithTimeout(context.Background(), durationToFetchHistoryPages)
	defer cancel()

	if err := wm.sfnapi.GetExecutionHistoryPagesWithContext(ctx, &sfn.GetExecutionHistoryInput{
		ExecutionArn: aws.String(execARN),
	}, func(historyOutput *sfn.GetExecutionHistoryOutput, lastPage bool) bool {
		// NOTE: if pulling the entire execution history becomes infeasible, we can:
		// 1) limit the results with `maxResults`
		// 2) set `reverseOrder` to true to get most recent events first
		// 3) stop paging once we get to to the smallest job ID (aka event ID) that is still pending
		for _, evt := range historyOutput.Events {
			job := eventToJob(evt)
			if job == nil {
				continue
			}
			switch aws.StringValue(evt.Type) {
			case sfn.HistoryEventTypeTaskStateEntered, sfn.HistoryEventTypeChoiceStateEntered, sfn.HistoryEventTypeSucceedStateEntered:
				// event IDs start at 1 and are only unique to the execution, so this might not be ideal
				job.ID = fmt.Sprintf("%d", aws.Int64Value(evt.Id))
				job.Attempts = []*models.JobAttempt{}
				job.CreatedAt = strfmt.DateTime(aws.TimeValue(evt.Timestamp))
				if *evt.Type != sfn.HistoryEventTypeTaskStateEntered {
					// Non-task states technically start immediately, since they don't wait on resources:
					job.StartedAt = job.CreatedAt
				}
				job.Status = models.JobStatusCreated
				if details := evt.StateEnteredEventDetails; details != nil {
					stateName := aws.StringValue(details.Name)
					var stateResourceName string
					var stateResourceType models.StateResourceType
					stateDef, ok := workflow.WorkflowDefinition.StateMachine.States[stateName]
					if ok {
						if strings.HasPrefix(stateDef.Resource, "lambda:") {
							stateResourceName = strings.TrimPrefix(stateDef.Resource, "lambda:")
							stateResourceType = models.StateResourceTypeLambdaFunctionARN
						} else {
							stateResourceName = stateDef.Resource
							stateResourceType = models.StateResourceTypeActivityARN
						}
					}
					job.Input = aws.StringValue(details.Input)
					job.State = stateName

					job.StateResource = &models.StateResource{
						Name:        stateResourceName,
						Type:        stateResourceType,
						Namespace:   workflow.Namespace,
						LastUpdated: strfmt.DateTime(aws.TimeValue(evt.Timestamp)),
					}
				}
			case sfn.HistoryEventTypeActivityScheduled, sfn.HistoryEventTypeLambdaFunctionScheduled:
				if job.Status == models.JobStatusFailed {
					// this is a retry, copy job data to attempt array, re-initialize job data
					oldJobData := *job
					*job = models.Job{}
					job.ID = fmt.Sprintf("%d", aws.Int64Value(evt.Id))
					job.Attempts = append(oldJobData.Attempts, &models.JobAttempt{
						Reason:    oldJobData.StatusReason,
						CreatedAt: oldJobData.CreatedAt,
						StartedAt: oldJobData.StartedAt,
						StoppedAt: oldJobData.StoppedAt,
						TaskARN:   oldJobData.Container,
					})
					job.CreatedAt = strfmt.DateTime(aws.TimeValue(evt.Timestamp))
					job.Input = oldJobData.Input
					job.State = oldJobData.State
					job.StateResource = &models.StateResource{
						Name:        oldJobData.StateResource.Name,
						Type:        oldJobData.StateResource.Type,
						Namespace:   oldJobData.StateResource.Namespace,
						LastUpdated: strfmt.DateTime(aws.TimeValue(evt.Timestamp)),
					}
				}
				job.Status = models.JobStatusQueued
			case sfn.HistoryEventTypeActivityStarted, sfn.HistoryEventTypeLambdaFunctionStarted:
				job.Status = models.JobStatusRunning
				job.StartedAt = strfmt.DateTime(aws.TimeValue(evt.Timestamp))
				if details := evt.ActivityStartedEventDetails; details != nil {
					job.Container = aws.StringValue(details.WorkerName)
				}
			case sfn.HistoryEventTypeActivityFailed, sfn.HistoryEventTypeLambdaFunctionFailed, sfn.HistoryEventTypeLambdaFunctionScheduleFailed, sfn.HistoryEventTypeLambdaFunctionStartFailed:
				job.Status = models.JobStatusFailed
				job.StoppedAt = strfmt.DateTime(aws.TimeValue(evt.Timestamp))
				cause, errorName := causeAndErrorNameFromFailureEvent(evt)
				// TODO: need more natural place to put error name...
				job.StatusReason = strings.TrimSpace(fmt.Sprintf(
					"%s\n%s",
					getLastFewLines(cause),
					errorName,
				))
			case sfn.HistoryEventTypeActivityTimedOut, sfn.HistoryEventTypeLambdaFunctionTimedOut:
				job.Status = models.JobStatusFailed
				job.StoppedAt = strfmt.DateTime(aws.TimeValue(evt.Timestamp))
				cause, errorName := causeAndErrorNameFromFailureEvent(evt)
				job.StatusReason = strings.TrimSpace(fmt.Sprintf(
					"%s\n%s\n%s",
					resources.StatusReasonJobTimedOut,
					errorName,
					getLastFewLines(cause),
				))
			case sfn.HistoryEventTypeActivitySucceeded, sfn.HistoryEventTypeLambdaFunctionSucceeded:
				job.Status = models.JobStatusSucceeded
			case sfn.HistoryEventTypeExecutionAborted:
				job.Status = models.JobStatusAbortedByUser
				job.StoppedAt = strfmt.DateTime(aws.TimeValue(evt.Timestamp))
				if details := evt.ExecutionAbortedEventDetails; details != nil {
					job.StatusReason = aws.StringValue(details.Cause)
				}
			case sfn.HistoryEventTypeExecutionFailed:
				job.Status = models.JobStatusFailed
				job.StoppedAt = strfmt.DateTime(aws.TimeValue(evt.Timestamp))
				if details := evt.ExecutionFailedEventDetails; details != nil {
					if isActivityDoesntExistFailure(evt.ExecutionFailedEventDetails) {
						job.StatusReason = "State resource does not exist"
					} else if isActivityTimedOutFailure(evt.ExecutionFailedEventDetails) {
						// do not update job status reason -- it should already be updated based on the ActivityTimedOut event
					} else {
						// set unknown errors to StatusReason
						job.StatusReason = strings.TrimSpace(fmt.Sprintf(
							"%s\n%s",
							getLastFewLines(aws.StringValue(details.Cause)),
							aws.StringValue(details.Error),
						))
					}
				}
			case sfn.HistoryEventTypeExecutionTimedOut:
				job.Status = models.JobStatusFailed
				job.StoppedAt = strfmt.DateTime(aws.TimeValue(evt.Timestamp))
				if details := evt.ExecutionTimedOutEventDetails; details != nil {
					job.StatusReason = strings.TrimSpace(fmt.Sprintf(
						"%s\n%s\n%s",
						resources.StatusReasonWorkflowTimedOut,
						aws.StringValue(details.Error),
						getLastFewLines(aws.StringValue(details.Cause)),
					))
				} else {
					job.StatusReason = resources.StatusReasonWorkflowTimedOut
				}
			case sfn.HistoryEventTypeTaskStateExited:
				stateExited := evt.StateExitedEventDetails
				job.StoppedAt = strfmt.DateTime(aws.TimeValue(evt.Timestamp))
				if stateExited.Output != nil {
					job.Output = aws.StringValue(stateExited.Output)
				}
			case sfn.HistoryEventTypeChoiceStateExited, sfn.HistoryEventTypeSucceedStateExited:
				job.Status = models.JobStatusSucceeded
				job.StoppedAt = strfmt.DateTime(aws.TimeValue(evt.Timestamp))
				details := evt.StateExitedEventDetails
				if details.Output != nil {
					job.Output = aws.StringValue(details.Output)
				}
			}

		}
		return true
	}); err != nil {
		return err
	}
	workflow.Jobs = jobs

	return wm.store.UpdateWorkflow(ctx, *workflow)
}

// isActivityDoesntExistFailure checks if an execution failed because an activity doesn't exist.
// This currently results in a cryptic AWS error, so the logic is probably over-broad: https://console.aws.amazon.com/support/home?region=us-west-2#/case/?displayId=4514731511&language=en
// If SFN creates a more descriptive error event we should change this.
func isActivityDoesntExistFailure(details *sfn.ExecutionFailedEventDetails) bool {
	return aws.StringValue(details.Error) == "States.Runtime" &&
		strings.Contains(aws.StringValue(details.Cause), "Internal Error")
}

// isActivityTimedOutFailure checks if an execution failed because an activity timed out,
// based on the details of an 'execution failed' history event.
func isActivityTimedOutFailure(details *sfn.ExecutionFailedEventDetails) bool {
	return aws.StringValue(details.Error) == "States.Timeout"
}

func getLastFewLines(rawLines string) string {
	lastFewLines := strings.Split(strings.TrimSpace(rawLines), "\n")
	if len(lastFewLines) > maxFailureReasonLines {
		lastFewLines = lastFewLines[len(lastFewLines)-maxFailureReasonLines:]
	}

	return strings.Join(lastFewLines, "\n")
}

func causeAndErrorNameFromFailureEvent(evt *sfn.HistoryEvent) (string, string) {
	switch aws.StringValue(evt.Type) {
	case sfn.HistoryEventTypeActivityFailed:
		return aws.StringValue(evt.ActivityFailedEventDetails.Cause), aws.StringValue(evt.ActivityFailedEventDetails.Error)
	case sfn.HistoryEventTypeActivityScheduleFailed:
		return aws.StringValue(evt.ActivityScheduleFailedEventDetails.Cause), aws.StringValue(evt.ActivityScheduleFailedEventDetails.Error)
	case sfn.HistoryEventTypeActivityTimedOut:
		return aws.StringValue(evt.ActivityTimedOutEventDetails.Cause), aws.StringValue(evt.ActivityTimedOutEventDetails.Error)
	case sfn.HistoryEventTypeExecutionAborted:
		return aws.StringValue(evt.ExecutionAbortedEventDetails.Cause), aws.StringValue(evt.ExecutionAbortedEventDetails.Error)
	case sfn.HistoryEventTypeExecutionFailed:
		return aws.StringValue(evt.ExecutionFailedEventDetails.Cause), aws.StringValue(evt.ExecutionFailedEventDetails.Error)
	case sfn.HistoryEventTypeExecutionTimedOut:
		return aws.StringValue(evt.ExecutionTimedOutEventDetails.Cause), aws.StringValue(evt.ExecutionTimedOutEventDetails.Error)
	case sfn.HistoryEventTypeLambdaFunctionFailed:
		return aws.StringValue(evt.LambdaFunctionFailedEventDetails.Cause), aws.StringValue(evt.LambdaFunctionFailedEventDetails.Error)
	case sfn.HistoryEventTypeLambdaFunctionScheduleFailed:
		return aws.StringValue(evt.LambdaFunctionScheduleFailedEventDetails.Cause), aws.StringValue(evt.LambdaFunctionScheduleFailedEventDetails.Error)
	case sfn.HistoryEventTypeLambdaFunctionStartFailed:
		return aws.StringValue(evt.LambdaFunctionStartFailedEventDetails.Cause), aws.StringValue(evt.LambdaFunctionStartFailedEventDetails.Error)
	case sfn.HistoryEventTypeLambdaFunctionTimedOut:
		return aws.StringValue(evt.LambdaFunctionTimedOutEventDetails.Cause), aws.StringValue(evt.LambdaFunctionTimedOutEventDetails.Error)
	default:
		return "", ""
	}
}
