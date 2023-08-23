package executor

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs/cloudwatchlogsiface"
	"github.com/aws/aws-sdk-go/service/sfn"
	"github.com/aws/aws-sdk-go/service/sfn/sfniface"
	"github.com/go-openapi/strfmt"
	"github.com/go-openapi/swag"
	"github.com/mohae/deepcopy"

	"github.com/Clever/kayvee-go/v7/logger"
	"github.com/Clever/workflow-manager/executor/sfnconventions"
	"github.com/Clever/workflow-manager/gen-go/models"
	"github.com/Clever/workflow-manager/resources"
	"github.com/Clever/workflow-manager/store"
	"github.com/Clever/workflow-manager/wfupdater"
)

const (
	executionEventsPerPage  = 200
	sfncliCommandTerminated = "sfncli.CommandTerminated"

	jobNameField = "JobName"
)

var (
	durationToRetryDescribeExecutions = 5 * time.Minute
	durationToFetchHistoryPages       = time.Minute
)

var defaultSFNCLICommandTerminatedRetrier = &models.SLRetrier{
	BackoffRate:     1.0,
	ErrorEquals:     []models.SLErrorEquals{sfncliCommandTerminated},
	IntervalSeconds: 10,
	MaxAttempts:     swag.Int64(10),
}

// SFNWorkflowManager manages workflows run through AWS Step Functions.
type SFNWorkflowManager struct {
	sfnapi                   sfniface.SFNAPI
	cwlogsapi                cloudwatchlogsiface.CloudWatchLogsAPI
	store                    store.Store
	region                   string
	roleARN                  string
	accountID                string
	executionEventsStreamARN string
	cwLogsToKinesisRoleARN   string
	updatedLoggingConfig     sync.Map
}

func NewSFNWorkflowManager(sfnapi sfniface.SFNAPI, cwlogsapi cloudwatchlogsiface.CloudWatchLogsAPI, store store.Store, roleARN, region, accountID, executionEventsStreamARN, cwLogsToKinesisRoleARN string) *SFNWorkflowManager {
	return &SFNWorkflowManager{
		sfnapi:                   sfnapi,
		cwlogsapi:                cwlogsapi,
		store:                    store,
		roleARN:                  roleARN,
		region:                   region,
		accountID:                accountID,
		executionEventsStreamARN: executionEventsStreamARN,
		cwLogsToKinesisRoleARN:   cwLogsToKinesisRoleARN,
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
//   - non-nil. The default serialization of a nil slice is "null", which the AWS API dislikes.
//   - sfncli.CommandTerminated for any Task state. See sfncli: https://github.com/clever/sfncli.
//     This is to ensure that states are retried on signaled termination of activities (e.g. deploys).
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

func toTagMap(wdTags map[string]interface{}) map[string]string {
	tags := map[string]string{}
	for k, v := range wdTags {
		vs, ok := v.(string)
		if ok {
			tags[k] = vs
		}
	}
	return tags
}

func toCWLogGroupTags(tags map[string]string) map[string]*string {
	cwlogsTags := map[string]*string{}
	for k, v := range tags {
		cwlogsTags[k] = aws.String(v)
	}
	return cwlogsTags
}

func toSFNTags(tags map[string]string) []*sfn.Tag {
	sfnTags := []*sfn.Tag{}
	for k, v := range tags {
		sfnTags = append(sfnTags, &sfn.Tag{
			Key:   aws.String(k),
			Value: aws.String(v),
		})
	}
	return sfnTags
}

func loggingConfiguration(logGroupARN string) *sfn.LoggingConfiguration {
	return &sfn.LoggingConfiguration{
		Destinations: []*sfn.LogDestination{{
			CloudWatchLogsLogGroup: &sfn.CloudWatchLogsLogGroup{
				LogGroupArn: aws.String(logGroupARN),
			},
		}},
		Level:                aws.String(sfn.LogLevelAll),
		IncludeExecutionData: aws.Bool(true), // we need execution output data specifically when setting workflow.Output
	}
}

func (wm *SFNWorkflowManager) createLogGroupsForLoggingConfiguration(ctx context.Context, tags map[string]string, lc *sfn.LoggingConfiguration) error {
	// for now there's only one log group destination in our state machine logging configuration, but loop over them for completeness
	for _, ld := range lc.Destinations {
		parts := strings.Split(*ld.CloudWatchLogsLogGroup.LogGroupArn, ":")
		if len(parts) != 8 {
			return fmt.Errorf("unexpected log group ARN format: %s", *ld.CloudWatchLogsLogGroup.LogGroupArn)
		}
		lgname := parts[6]
		if _, err := wm.cwlogsapi.CreateLogGroupWithContext(ctx, &cloudwatchlogs.CreateLogGroupInput{
			LogGroupName: aws.String(lgname),
			Tags:         toCWLogGroupTags(tags),
		}); err != nil {
			// ignore already exists error since in that case the log group is created and we're all set
			if awsErr, ok := err.(awserr.Error); !ok || awsErr.Code() != cloudwatchlogs.ErrCodeResourceAlreadyExistsException {
				return err
			}
		}
		if _, err := wm.cwlogsapi.PutRetentionPolicyWithContext(ctx, &cloudwatchlogs.PutRetentionPolicyInput{
			LogGroupName:    aws.String(lgname),
			RetentionInDays: aws.Int64(7),
		}); err != nil {
			return err
		}
		if _, err := wm.cwlogsapi.PutSubscriptionFilterWithContext(ctx, &cloudwatchlogs.PutSubscriptionFilterInput{
			DestinationArn: aws.String(wm.executionEventsStreamARN),
			Distribution:   aws.String(cloudwatchlogs.DistributionByLogStream), // need to shard by log stream in order to guarantee ordered processing w/in log stream
			FilterName:     aws.String("to-kinesis"),
			FilterPattern:  aws.String(""), // empty string means everything
			LogGroupName:   aws.String(lgname),
			RoleArn:        aws.String(wm.cwLogsToKinesisRoleARN),
		}); err != nil {
			return err
		}
	}
	return nil
}

func (wm *SFNWorkflowManager) updateLoggingConfiguration(ctx context.Context, stateMachineARN string, tags map[string]string, lc *sfn.LoggingConfiguration) error {
	// must create the log group before creating or updating a state machine referencing the log group
	if err := wm.createLogGroupsForLoggingConfiguration(ctx, tags, lc); err != nil {
		return err
	}
	logger.FromContext(ctx).InfoD("update-logging-configuration", logger.M{"state-machine-arn": stateMachineARN})
	if _, err := wm.sfnapi.UpdateStateMachineWithContext(ctx, &sfn.UpdateStateMachineInput{
		LoggingConfiguration: lc,
		StateMachineArn:      aws.String(stateMachineARN),
	}); err != nil {
		return err
	}
	// seeing some eventual consistency in the update behavior... e.g. if an execution is started
	// shortly after calling UpdateStateMachine, the logging configuration may not have taken hold yet.
	// wait a bit to make sure the update is complete
	time.Sleep(2 * time.Second)
	return nil
}

func (wm *SFNWorkflowManager) describeOrCreateStateMachine(ctx context.Context, wd models.WorkflowDefinition, namespace, queue string) (*sfn.DescribeStateMachineOutput, error) {
	// the name must be unique. Use workflow definition name + version + namespace + queue to uniquely identify a state machine
	// this effectively creates a new workflow definition in each namespace we deploy into
	awsStateMachineName := sfnconventions.StateMachineName(wd.Name, wd.Version, namespace, wd.StateMachine.StartAt)
	tags := sfnconventions.StateMachineTags(namespace, wd.Name, wd.Version, wd.StateMachine.StartAt, toTagMap(wd.DefaultTags))
	describeOutput, err := wm.sfnapi.DescribeStateMachineWithContext(ctx,
		&sfn.DescribeStateMachineInput{
			StateMachineArn: aws.String(sfnconventions.StateMachineArn(wm.region, wm.accountID, wd.Name, wd.Version, namespace, wd.StateMachine.StartAt)),
		})
	if err == nil {
		// logging configuration is something we have recently added and might not be present yet
		// describestatemachine is cached (see sfncache package in this repo) so we need to be careful about using
		// the output repeatedly. Use the updatedLoggingConfig map to track this
		_, alreadyUpdated := wm.updatedLoggingConfig.Load(*describeOutput.StateMachineArn)
		if !alreadyUpdated && (describeOutput.LoggingConfiguration == nil || aws.StringValue(describeOutput.LoggingConfiguration.Level) != sfn.LogLevelAll ||
			(len(describeOutput.LoggingConfiguration.Destinations) > 0 && !strings.Contains(aws.StringValue(describeOutput.LoggingConfiguration.Destinations[0].CloudWatchLogsLogGroup.LogGroupArn), "log-group:/aws/vendedlogs/states/"))) {
			if err := wm.updateLoggingConfiguration(ctx, *describeOutput.StateMachineArn, tags,
				loggingConfiguration(sfnconventions.LogGroupArn(wm.region, wm.accountID, awsStateMachineName))); err != nil {
				return nil, err
			}
			wm.updatedLoggingConfig.Store(*describeOutput.StateMachineArn, struct{}{})
			return wm.describeOrCreateStateMachine(ctx, wd, namespace, queue)
		}
		return describeOutput, nil
	} else {
		awserr, ok := err.(awserr.Error)
		if !ok {
			return nil, fmt.Errorf("non-AWS error in findOrCreateStateMachine: %s", err)
		}
		if awserr.Code() != sfn.ErrCodeStateMachineDoesNotExist {
			return nil, fmt.Errorf("unexpected AWS error in findOrCreateStateMachine: %s", awserr)
		}
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

	// we use the 'application' tag to attribute costs, so if it wasn't explicitly specified, set it to the workflow name
	if _, ok := wd.DefaultTags["application"]; !ok {
		wd.DefaultTags["application"] = wd.Name
	}
	log.InfoD("create-state-machine", logger.M{"definition": awsStateMachineDef, "name": awsStateMachineName})
	var lc *sfn.LoggingConfiguration
	lc = loggingConfiguration(sfnconventions.LogGroupArn(wm.region, wm.accountID, awsStateMachineName))
	// must create the log group before creating a state machine referencing the log group
	if err := wm.createLogGroupsForLoggingConfiguration(ctx, tags, lc); err != nil {
		return nil, err
	}
	_, err = wm.sfnapi.CreateStateMachineWithContext(ctx,
		&sfn.CreateStateMachineInput{
			Name:                 aws.String(awsStateMachineName),
			Definition:           aws.String(awsStateMachineDef),
			RoleArn:              aws.String(wm.roleARN),
			LoggingConfiguration: lc,
			Tags:                 toSFNTags(tags),
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
	tags map[string]interface{},
) (*models.Workflow, error) {
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

	if err = wm.store.SaveWorkflow(ctx, *workflow); err != nil {
		return nil, err
	}

	// submit an execution using input, set execution name == our workflow GUID
	err = wm.startExecution(ctx, describeOutput.StateMachineArn, workflow.ID, input)
	if err != nil {
		go func() {
			// We failed to start execution; remove the workflow from
			// store. Use a new context with a sane timeout, the
			// request context context may get canceled before we
			// complete this operation.
			asyncCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()
			if delErr := wm.store.DeleteWorkflowByID(asyncCtx, workflow.ID); delErr != nil {
				log.ErrorD("create-workflow", logger.M{
					"workflow-id":              workflow.ID,
					"workflow-definition-name": workflow.WorkflowDefinition.Name,
					"message":                  "failed to delete stray workflow",
					"error":                    fmt.Sprintf("sfn error: %s; store error: %s", err, delErr),
				})
			}
		}()

		return nil, err
	}

	return workflow, nil
}

func (wm *SFNWorkflowManager) RetryWorkflow(ctx context.Context, ogWorkflow models.Workflow, startAt, input string) (*models.Workflow, error) {
	// don't allow resume if workflow is still active
	if !resources.WorkflowIsDone(&ogWorkflow) {
		return nil, fmt.Errorf("workflow %s active: %s", ogWorkflow.ID, ogWorkflow.Status)
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

	// save the workflow before starting execution to ensure we don't have
	// untracked executions i.e. execution was started but we failed to save workflow
	// If we fail starting the execution, we can resolve this out of band (TODO: should support cancelling)
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
					"error":        err.Error(),
				})

				// only fail after 5 minutes to see if this is an eventual-consistency thing
				if time.Time(workflow.LastUpdated).Before(time.Now().Add(-durationToRetryDescribeExecutions)) {
					log.ErrorD("zombie-execution-found", logger.M{
						"workflow-id":  workflow.ID,
						"execution-id": execARN,
					})
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
		if err := wfupdater.UpdateWorkflowLastJob(ctx, wm.sfnapi, execARN, workflow); err != nil {
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
		jobs = append(jobs, wfupdater.ProcessEvents(historyOutput.Events, execARN, workflow, eventIDToJob)...)
		return true
	}); err != nil {
		log.ErrorD("describe-execution", logger.M{"workflow-id": workflow.ID, "error": err.Error()})
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case sfn.ErrCodeExecutionDoesNotExist:
				// either execution hasn't started yet or this is a zombie workflow
				// we just return with no error as there is no history to update
				return nil
			}
		}
		return err
	}
	workflow.Jobs = jobs

	return wm.store.UpdateWorkflow(ctx, *workflow)
}
