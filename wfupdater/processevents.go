package wfupdater

import (
	"fmt"
	"strings"

	"github.com/Clever/kayvee-go/v7/logger"
	"github.com/Clever/workflow-manager/gen-go/models"
	"github.com/Clever/workflow-manager/resources"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/sfn"
	"github.com/go-openapi/strfmt"
)

var log = logger.New("workflow-manager")

const (
	maxFailureReasonLines = 3
)

// ProcessEvents constructs the Jobs array from the AWS step-functions event history.
func ProcessEvents(
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
			sfn.HistoryEventTypeLambdaFunctionStartFailed,
			sfn.HistoryEventTypeTaskSubmitFailed:
			job.Status = models.JobStatusFailed
			job.StoppedAt = strfmt.DateTime(aws.TimeValue(evt.Timestamp))
			cause, errorName := causeAndErrorNameFromFailureEvent(evt)
			// TODO: need more natural place to put error name...
			job.StatusReason = strings.TrimSpace(fmt.Sprintf(
				"%s\n%s",
				getLastFewLines(cause),
				errorName,
			))
		case sfn.HistoryEventTypeActivityTimedOut,
			sfn.HistoryEventTypeLambdaFunctionTimedOut,
			sfn.HistoryEventTypeTaskTimedOut:
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
			"ExecutionSucceeded", // TODO: remove this and replace with a sfn.xxx once aws-sdk-go supports this event type
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
		default:
			// There are 50+ event types and all of them don't need to be handled
			// but it is a good idea to log the event type for debugging purposes
			log.InfoD("unhandled-sfn-event", logger.M{
				"event-type":  aws.StringValue(evt.Type),
				"workflow-id": workflow.ID,
			})
		}
	}

	return jobs
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
	case sfn.HistoryEventTypeTaskTimedOut:
		return aws.StringValue(evt.TaskTimedOutEventDetails.Cause), aws.StringValue(evt.TaskTimedOutEventDetails.Error)
	case sfn.HistoryEventTypeTaskSubmitFailed:
		return aws.StringValue(evt.TaskSubmitFailedEventDetails.Cause), aws.StringValue(evt.TaskSubmitFailedEventDetails.Error)
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
