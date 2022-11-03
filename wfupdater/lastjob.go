package wfupdater

import (
	"context"

	"github.com/Clever/kayvee-go/v7/logger"
	"github.com/Clever/workflow-manager/gen-go/models"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/sfn"
	"github.com/aws/aws-sdk-go/service/sfn/sfniface"
)

// UpdateWorkflowLastJob queries AWS sfn for the latest events of the given state machine,
// and it uses these events to populate LastJob within the WorkflowSummary.
// This method is heavily inspired by UpdateWorkflowHistory, however updateWorkflowLastJob only
// finds the last Job which is all that is needed to determine the state where a workflow has failed.
// We try to use the sfn GetExecutionHistory API endpoint somewhat sparingly because
// it has a relatively low rate limit, and it can take multiple calls per workflow
// to retrieve the full event history for some workflows.
func UpdateWorkflowLastJob(ctx context.Context, sfnapi sfniface.SFNAPI, execARN string, workflow *models.Workflow) error {
	historyOutput, err := sfnapi.GetExecutionHistoryWithContext(ctx, &sfn.GetExecutionHistoryInput{
		ExecutionArn: aws.String(execARN),
		ReverseOrder: aws.Bool(true),
	})
	if err != nil {
		return err
	}

	eventIDToJob := map[int64]*models.Job{}
	// Events are processed chronologically, so the output of GetExecutionHistory is reversed when
	// the events have been returned in reverse order.
	jobs := ProcessEvents(ReverseHistory(historyOutput.Events), execARN, workflow, eventIDToJob)
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
