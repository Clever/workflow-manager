package executor

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Clever/workflow-manager/executor/sfnconventions"
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
	executionEventsPerPage  = 200
	sfncliCommandTerminated = "sfncli.CommandTerminated"

	jobNameField = "JobName"
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

// stateMachineWithFullActivityARNsAndParameters returns a new state machine.
// Our workflow definitions contain state machine definitions with short-hand for resource names and parameter fields, e.g. "Resource": "name-of-worker"
// Convert this shorthand into a new state machine with full details, e.g. "Resource": "arn:aws:states:us-west-2:589690932525:activity:production--name-of-worker"
func stateMachineWithFullActivityARNsAndParameters(oldSM models.SLStateMachine, region, accountID, namespace string) (*models.SLStateMachine, error) {
	sm := deepcopy.Copy(oldSM).(models.SLStateMachine)
	var err error
	for stateName, s := range sm.States {
		state := deepcopy.Copy(s).(models.SLState)
		if state.Type == models.SLStateTypeParallel {
			for index, branch := range state.Branches {
				if branch == nil {
					return nil, fmt.Errorf("Nil branch found in parallel state")
				}
				state.Branches[index], err = stateMachineWithFullActivityARNsAndParameters(*branch, region, accountID, namespace)
				if err != nil {
					return nil, err
				}
			}
			sm.States[stateName] = state
			continue
		} else if state.Type == models.SLStateTypeMap {
			if state.Iterator == nil {
				return nil, fmt.Errorf("Nil iterator found in map state")
			}
			if state.Parameters == nil {
				state.Parameters = map[string]interface{}{}
			}
			if _, copyingExecutionName := state.Parameters["_EXECUTION_NAME.$"]; !copyingExecutionName {
				// Copies over of _EXECUTION_NAME so that its absence in input will not break things
				state.Parameters["_EXECUTION_NAME.$"] = "$._EXECUTION_NAME"
			}
			state.Iterator, err = stateMachineWithFullActivityARNsAndParameters(*state.Iterator, region, accountID, namespace)
			if err != nil {
				return nil, err
			}
			sm.States[stateName] = state
			continue
		} else if state.Type != models.SLStateTypeTask {
			continue
		}
		if strings.HasPrefix(state.Resource, "lambda:") {
			state.Resource = sfnconventions.LambdaResource(state.Resource, region, accountID, namespace)
		} else if strings.HasPrefix(state.Resource, "glue:") {
			var jobName string
			state.Resource, jobName = sfnconventions.GlueResourceAndJobName(state.Resource, namespace)
			if state.Parameters == nil {
				state.Parameters = map[string]interface{}{}
			}
			state.Parameters[jobNameField] = jobName
		} else {
			state.Resource = sfnconventions.SFNCLIResource(state.Resource, region, accountID, namespace)
		}
		sm.States[stateName] = state
	}
	return &sm, nil
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

func toSFNTags(wmTags map[string]interface{}) []*sfn.Tag {
	sfnTags := []*sfn.Tag{}
	for k, v := range wmTags {
		vs, ok := v.(string)
		if ok {
			sfnTags = append(sfnTags, &sfn.Tag{
				Key:   aws.String(k),
				Value: aws.String(vs),
			})
		}
	}
	return sfnTags
}

func (wm *SFNWorkflowManager) describeOrCreateStateMachine(ctx context.Context, wd models.WorkflowDefinition, namespace, queue string) (*sfn.DescribeStateMachineOutput, error) {
	describeOutput, err := wm.sfnapi.DescribeStateMachineWithContext(ctx,
		&sfn.DescribeStateMachineInput{
			StateMachineArn: aws.String(sfnconventions.StateMachineArn(wm.region, wm.accountID, wd.Name, wd.Version, namespace, wd.StateMachine.StartAt)),
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
	awsStateMachine, err := stateMachineWithFullActivityARNsAndParameters(*wd.StateMachine, wm.region, wm.accountID, namespace)
	if err != nil {
		return nil, err
	}
	awsStateMachine = stateMachineWithDefaultRetriers(*awsStateMachine)
	awsStateMachineDefBytes, err := json.MarshalIndent(awsStateMachine, "", "  ")
	if err != nil {
		return nil, err
	}
	awsStateMachineDef := string(awsStateMachineDefBytes)
	// the name must be unique. Use workflow definition name + version + namespace + queue to uniquely identify a state machine
	// this effectively creates a new workflow definition in each namespace we deploy into
	awsStateMachineName := sfnconventions.StateMachineName(wd.Name, wd.Version, namespace, wd.StateMachine.StartAt)
	// we use the 'application' tag to attribute costs, so if it wasn't explicitly specified, set it to the workflow name
	if _, ok := wd.DefaultTags["application"]; !ok {
		wd.DefaultTags["application"] = wd.Name
	}
	log.InfoD("create-state-machine", logger.M{"definition": awsStateMachineDef, "name": awsStateMachineName})
	_, err = wm.sfnapi.CreateStateMachineWithContext(ctx,
		&sfn.CreateStateMachineInput{
			Name:       aws.String(awsStateMachineName),
			Definition: aws.String(awsStateMachineDef),
			RoleArn:    aws.String(wm.roleARN),
			Tags: append([]*sfn.Tag{
				{Key: aws.String("environment"), Value: aws.String(namespace)},
				{Key: aws.String("workflow-definition-name"), Value: aws.String(wd.Name)},
				{Key: aws.String("workflow-definition-version"), Value: aws.String(fmt.Sprintf("%d", wd.Version))},
				{Key: aws.String("workflow-definition-start-at"), Value: aws.String(wd.StateMachine.StartAt)},
			}, toSFNTags(wd.DefaultTags)...),
		})
	if err != nil {
		return nil, fmt.Errorf("CreateStateMachine error: %s", err.Error())
	}

	return wm.describeOrCreateStateMachine(ctx, wd, namespace, queue)
}

func (wm *SFNWorkflowManager) startExecution(ctx context.Context, stateMachineArn *string, workflowID, input string) error {
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
	_, err = wm.sfnapi.StartExecutionWithContext(ctx, &sfn.StartExecutionInput{
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

	describeOutput, err := wm.describeOrCreateStateMachine(ctx, wd, namespace, queue)
	if err != nil {
		return nil, err
	}

	mergedTags := map[string]interface{}{}
	for k, v := range wd.DefaultTags {
		mergedTags[k] = v
	}
	// tags passed to CreateWorkflow overwrite wd.DefaultTags upon key conflict
	for k, v := range tags {
		mergedTags[k] = v
	}

	// start the update loop and save the workflow before starting execution to ensure we don't have
	// untracked executions i.e. execution was started but we failed to save workflow
	// If we fail starting the execution, we can resolve this out of band (TODO: should support cancelling)

	workflow := resources.NewWorkflow(&wd, input, namespace, queue, mergedTags)
	logger.FromContext(ctx).AddContext("workflow-id", workflow.ID)

	err = createPendingWorkflow(ctx, workflow.ID, wm.sqsapi, wm.sqsQueueURL)
	if err != nil {
		// this error is logged in createPendingWorkflow with title "sqs-send-message"
		return nil, err
	}

	if err := wm.store.SaveWorkflow(ctx, *workflow); err != nil {
		return nil, err
	}

	// submit an execution using input, set execution name == our workflow GUID
	err = wm.startExecution(ctx, describeOutput.StateMachineArn, workflow.ID, input)
	if err != nil {
		// since we failed to start execution, remove Workflow from store
		if delErr := wm.store.DeleteWorkflowByID(ctx, workflow.ID); delErr != nil {
			log.ErrorD("create-workflow", logger.M{
				"workflow-id":              workflow.ID,
				"workflow-definition-name": workflow.WorkflowDefinition.Name,
				"message":                  "failed to delete stray workflow",
				"error":                    fmt.Sprintf("SFNError: %s;StoreError: %s", err, delErr),
			})
		}

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
	describeOutput, err := wm.describeOrCreateStateMachine(ctx, newDef, ogWorkflow.Namespace, ogWorkflow.Queue)
	if err != nil {
		return nil, err
	}

	workflow := resources.NewWorkflow(&newDef, input, ogWorkflow.Namespace, ogWorkflow.Queue, ogWorkflow.Tags)
	workflow.RetryFor = ogWorkflow.ID
	ogWorkflow.Retries = append(ogWorkflow.Retries, workflow.ID)

	// start the update loop and save the workflow before starting execution to ensure we don't have
	// untracked executions i.e. execution was started but we failed to save workflow
	// If we fail starting the execution, we can resolve this out of band (TODO: should support cancelling)

	err = createPendingWorkflow(ctx, workflow.ID, wm.sqsapi, wm.sqsQueueURL)
	if err != nil {
		// this error is logged in createPendingWorkflow with title "sqs-send-message"
		return nil, err
	}

	if err = wm.store.SaveWorkflow(ctx, *workflow); err != nil {
		return nil, err
	}
	// also update ogWorkflow
	if err = wm.store.UpdateWorkflow(ctx, ogWorkflow); err != nil {
		return nil, err
	}

	// submit an execution using input, set execution name == our workflow GUID
	err = wm.startExecution(ctx, describeOutput.StateMachineArn, workflow.ID, input)
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
	execARN := wm.executionArn(workflow, wd)
	if _, err := wm.sfnapi.StopExecutionWithContext(ctx, &sfn.StopExecutionInput{
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

func (wm *SFNWorkflowManager) executionArn(
	workflow *models.Workflow,
	definition *models.WorkflowDefinition,
) string {
	return sfnconventions.ExecutionArn(
		wm.region,
		wm.accountID,
		sfnconventions.StateMachineName(definition.Name, definition.Version, workflow.Namespace, definition.StateMachine.StartAt),
		workflow.ID,
	)
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
	execARN := sfnconventions.ExecutionArn(
		wm.region,
		wm.accountID,
		sfnconventions.StateMachineName(wd.Name, wd.Version, workflow.Namespace, wd.StateMachine.StartAt),
		workflow.ID,
	)
	describeOutput, err := wm.sfnapi.DescribeExecutionWithContext(ctx, &sfn.DescribeExecutionInput{
		ExecutionArn: aws.String(execARN),
	})
	if err != nil {
		log.ErrorD("describe-execution", logger.M{"workflow-id": workflow.ID, "error": err.Error()})
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case sfn.ErrCodeExecutionDoesNotExist:
				// since the update loop starts before starting execution,
				// this could mean that the workflow didn't start properly
				// which would be logged separately
				log.ErrorD("execution-not-found", logger.M{
					"workflow-id":  workflow.ID,
					"execution-id": execARN,
					"error":        err.Error()})

				// only fail after 5 minutes to see if this is an eventual-consistency thing
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
	workflow.Status = resources.SFNStatusToWorkflowStatus(*describeOutput.Status)
	if *describeOutput.Status == sfn.ExecutionStatusTimedOut {
		workflow.StatusReason = resources.StatusReasonWorkflowTimedOut
	}
	if describeOutput.StopDate != nil {
		workflow.StoppedAt = strfmt.DateTime(aws.TimeValue(describeOutput.StopDate))
	}
	if workflow.Status == models.WorkflowStatusSucceeded || workflow.Status == models.WorkflowStatusCancelled {
		workflow.ResolvedByUser = true
	}
	// Populate the last job within WorkflowSummary on failures so that workflows can be
	// more easily searched for and bucketed by failure state.
	if workflow.Status == models.WorkflowStatusFailed {
		if err := wm.updateWorkflowLastJob(ctx, workflow); err != nil {
			return err
		}
		failedJob := ""
		failedJobResource := ""
		if workflow.LastJob != nil {
			failedJob = workflow.LastJob.State
			if workflow.LastJob.StateResource != nil {
				failedJobResource = workflow.LastJob.StateResource.Name
			}
		}
		log.CounterD("workflow-failed", 1, logger.M{
			"workflow-name":       workflow.WorkflowDefinition.Name,
			"workflow-version":    workflow.WorkflowDefinition.Version,
			"workflow-id":         workflow.ID,
			"failed-job-name":     failedJob,
			"failed-job-resource": failedJobResource,
		})
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
	execARN := sfnconventions.ExecutionArn(
		wm.region,
		wm.accountID,
		sfnconventions.StateMachineName(wd.Name, wd.Version, workflow.Namespace, wd.StateMachine.StartAt),
		workflow.ID,
	)

	// Setup a context with a timeout of one minute since
	// we don't want to pull very large workflow histories
	ctx, cancel := context.WithTimeout(ctx, durationToFetchHistoryPages)
	defer cancel()

	var jobs []*models.Job
	eventIDToJob := map[int64]*models.Job{}
	if err := wm.sfnapi.GetExecutionHistoryPagesWithContext(ctx, &sfn.GetExecutionHistoryInput{
		ExecutionArn: aws.String(execARN),
		MaxResults:   aws.Int64(executionEventsPerPage),
	}, func(historyOutput *sfn.GetExecutionHistoryOutput, lastPage bool) bool {
		// NOTE: if pulling the entire execution history becomes infeasible, we can:
		// 1) limit the results with `maxResults`
		// 2) set `reverseOrder` to true to get most recent events first
		// 3) stop paging once we get to to the smallest job ID (aka event ID) that is still pending
		jobs = append(jobs, processEvents(historyOutput.Events, execARN, workflow, eventIDToJob)...)
		return true
	}); err != nil {
		return err
	}
	workflow.Jobs = jobs

	return wm.store.UpdateWorkflow(ctx, *workflow)
}

// updateWorkflowLastJob queries AWS sfn for the latest events of the given state machine,
// and it uses these events to populate LastJob within the WorkflowSummary.
// This method is heavily inspired by UpdateWorkflowHistory, however updateWorkflowLastJob only
// finds the last Job which is all that is needed to determine the state where a workflow has failed.
// We try to use the sfn GetExecutionHistory API endpoint somewhat sparingly because
// it has a relatively low rate limit, and it can take multiple calls per workflow
// to retrieve the full event history for some workflows.
func (wm *SFNWorkflowManager) updateWorkflowLastJob(ctx context.Context, workflow *models.Workflow) error {
	wd := workflow.WorkflowSummary.WorkflowDefinition
	execARN := sfnconventions.ExecutionArn(
		wm.region,
		wm.accountID,
		sfnconventions.StateMachineName(wd.Name, wd.Version, workflow.Namespace, wd.StateMachine.StartAt),
		workflow.ID,
	)

	historyOutput, err := wm.sfnapi.GetExecutionHistoryWithContext(ctx, &sfn.GetExecutionHistoryInput{
		ExecutionArn: aws.String(execARN),
		ReverseOrder: aws.Bool(true),
	})
	if err != nil {
		return err
	}

	eventIDToJob := map[int64]*models.Job{}
	// Events are processed chronologically, so the output of GetExecutionHistory is reversed when
	// the events have been returned in reverse order.
	jobs := processEvents(reverseHistory(historyOutput.Events), execARN, workflow, eventIDToJob)
	if len(jobs) > 0 {
		workflow.LastJob = jobs[len(jobs)-1]
	} else {
		log.ErrorD("empty-jobs", logger.M{
			"workflow": *workflow,
			"exec_arn": execARN,
		})
	}

	return nil
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

// eventToJob returns a Job based on a AWS step-function events.
func eventToJob(
	evt *sfn.HistoryEvent,
	execARN string, jobs []*models.Job,
	eventIDToJob map[int64]*models.Job,
) (*models.Job, map[int64]*models.Job) {
	eventID := aws.Int64Value(evt.Id)
	parentEventID := aws.Int64Value(evt.PreviousEventId)
	switch *evt.Type {
	// very first event for an execution, so there are no jobs yet
	case sfn.HistoryEventTypeExecutionStarted,
		// only create Jobs for Task, Choice and Succeed states
		sfn.HistoryEventTypePassStateEntered, sfn.HistoryEventTypePassStateExited,
		sfn.HistoryEventTypeParallelStateEntered, sfn.HistoryEventTypeParallelStateExited,
		sfn.HistoryEventTypeWaitStateEntered, sfn.HistoryEventTypeWaitStateExited,
		sfn.HistoryEventTypeFailStateEntered:
		return nil, eventIDToJob
	case sfn.HistoryEventTypeTaskStateEntered,
		sfn.HistoryEventTypeChoiceStateEntered,
		sfn.HistoryEventTypeSucceedStateEntered:
		// a job is created when a supported state is entered
		job := &models.Job{}
		eventIDToJob[eventID] = job
		return job, eventIDToJob
	case sfn.HistoryEventTypeExecutionAborted,
		sfn.HistoryEventTypeExecutionFailed,
		sfn.HistoryEventTypeExecutionTimedOut:
		// Execution-level event - update last seen job.
		if len(jobs) > 0 {
			return jobs[len(jobs)-1], eventIDToJob
		}
		log.ErrorD("event-with-unknown-job", logger.M{"event-id": eventID, "execution-arn": execARN})
		return nil, eventIDToJob
	default:
		// associate this event with the same job as its parent event
		jobOfParent, ok := eventIDToJob[parentEventID]
		if !ok {
			// we should investigate these cases, since it means we have a gap in our interpretation of the event history
			log.ErrorD("event-with-unknown-job", logger.M{"event-id": eventID, "execution-arn": execARN})
			return nil, eventIDToJob
		}
		eventIDToJob[eventID] = jobOfParent
		return jobOfParent, eventIDToJob
	}
}

// processEvents constructs the Jobs array from the AWS step-functions event history.
func processEvents(
	events []*sfn.HistoryEvent,
	execARN string,
	workflow *models.Workflow,
	eventIDToJob map[int64]*models.Job,
) []*models.Job {

	jobs := []*models.Job{}

	for _, evt := range events {
		job, updatedEventToJob := eventToJob(evt, execARN, jobs, eventIDToJob)
		eventIDToJob = updatedEventToJob
		if isSupportedStateEnteredEvent(evt) {
			jobs = append(jobs, job)
		}

		if job == nil {
			continue
		}
		switch aws.StringValue(evt.Type) {
		case sfn.HistoryEventTypeTaskStateEntered,
			sfn.HistoryEventTypeChoiceStateEntered,
			sfn.HistoryEventTypeSucceedStateEntered:
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
					} else if strings.HasPrefix(stateDef.Resource, "glue:") {
						stateResourceName = strings.TrimPrefix(stateDef.Resource, "glue:")
						stateResourceType = models.StateResourceTypeTaskARN
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
		case sfn.HistoryEventTypeTaskScheduled,
			sfn.HistoryEventTypeActivityScheduled,
			sfn.HistoryEventTypeLambdaFunctionScheduled:
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
		case sfn.HistoryEventTypeTaskStarted,
			sfn.HistoryEventTypeActivityStarted,
			sfn.HistoryEventTypeLambdaFunctionStarted:
			job.Status = models.JobStatusRunning
			job.StartedAt = strfmt.DateTime(aws.TimeValue(evt.Timestamp))
			if details := evt.ActivityStartedEventDetails; details != nil {
				job.Container = aws.StringValue(details.WorkerName)
			}
		case sfn.HistoryEventTypeActivityFailed,
			sfn.HistoryEventTypeLambdaFunctionFailed,
			sfn.HistoryEventTypeLambdaFunctionScheduleFailed,
			sfn.HistoryEventTypeLambdaFunctionStartFailed:
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
		case sfn.HistoryEventTypeTaskSucceeded,
			sfn.HistoryEventTypeActivitySucceeded,
			sfn.HistoryEventTypeLambdaFunctionSucceeded:
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

	return jobs
}
