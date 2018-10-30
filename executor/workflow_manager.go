package executor

import (
	"context"
	"time"

	"github.com/Clever/workflow-manager/gen-go/models"
	"github.com/Clever/workflow-manager/resources"
	"github.com/Clever/workflow-manager/store"
	"gopkg.in/Clever/kayvee-go.v6/logger"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/aws/aws-sdk-go/service/sqs/sqsiface"
)

// WorkflowManager is the interface for creating, stopping and checking status for Workflows
type WorkflowManager interface {
	CreateWorkflow(ctx context.Context, def models.WorkflowDefinition, input string, namespace string, queue string, tags map[string]interface{}) (*models.Workflow, error)
	RetryWorkflow(ctx context.Context, workflow models.Workflow, startAt, input string) (*models.Workflow, error)
	CancelWorkflow(ctx context.Context, workflow *models.Workflow, reason string) error
	UpdateWorkflowSummary(ctx context.Context, workflow *models.Workflow) error
	UpdateWorkflowHistory(ctx context.Context, workflow *models.Workflow) error
}

var backoffDuration = time.Second * 1
var shouldBackoff = false

// PollForPendingWorkflowsAndUpdateStore polls an SQS queue for workflows needing an update.
// It will stop polling when the context is done.
func PollForPendingWorkflowsAndUpdateStore(ctx context.Context, wm WorkflowManager, thestore store.Store, sqsapi sqsiface.SQSAPI, sqsQueueURL string) {
	for {
		//if shouldBackoff {
		//	log.WarnD("poll-for-pending-workflows-backoff", logger.M{"duration": backoffDuration.String()})
		//	time.Sleep(backoffDuration)
		//}
		//shouldBackoff = false

		select {
		case <-ctx.Done():
			log.Info("poll-for-pending-workflows-done")
			return
		default:
			out, err := sqsapi.ReceiveMessageWithContext(ctx, &sqs.ReceiveMessageInput{
				MaxNumberOfMessages: aws.Int64(10),
				QueueUrl:            aws.String(sqsQueueURL),
			})
			if err != nil {
				log.ErrorD("poll-for-pending-workflows", logger.M{"error": err.Error()})
			}

			for _, message := range out.Messages {
				if id, err := updatePendingWorkflow(ctx, message, wm, thestore, sqsapi, sqsQueueURL); err != nil {
					log.ErrorD("update-pending-workflow", logger.M{"id": id, "error": err.Error()})

					// If we're seeing DynamoDB throttling, let's wait before running our next poll loop
					if aerr, ok := err.(awserr.Error); ok {
						switch aerr.Code() {
						case dynamodb.ErrCodeProvisionedThroughputExceededException:
							log.WarnD("poll-for-pending-workflows-backoff", logger.M{"duration": backoffDuration.String()})
							time.Sleep(backoffDuration)
						}
					}
				} else {
					log.InfoD("update-pending-workflow", logger.M{"id": id})
				}
			}
		}
	}
}

// updateLoopDelay is the minimum amount of time between each update to a
// workflow's state. State is sync'd from workflow manager's backend, Step Functions.
const updateLoopDelay = 60

func createPendingWorkflow(ctx context.Context, workflowID string, sqsapi sqsiface.SQSAPI, sqsQueueURL string) error {
	_, err := sqsapi.SendMessageWithContext(ctx, &sqs.SendMessageInput{
		MessageBody:  aws.String(workflowID),
		QueueUrl:     aws.String(sqsQueueURL),
		DelaySeconds: aws.Int64(updateLoopDelay),
	})
	return err
}

func updatePendingWorkflow(ctx context.Context, m *sqs.Message, wm WorkflowManager, thestore store.Store, sqsapi sqsiface.SQSAPI, sqsQueueURL string) (string, error) {
	wfID := *m.Body
	wf, err := thestore.GetWorkflowByID(ctx, wfID)
	if err != nil {
		return "", err
	}

	logPendingWorkflowUpdateLag(wf)

	err = wm.UpdateWorkflowSummary(ctx, &wf)
	if err != nil {
		return "", err
	}

	// If workflow is not yet complete, send message to SQS to request a future update.
	if !resources.WorkflowIsDone(&wf) {
		_, err = sqsapi.SendMessageWithContext(ctx, &sqs.SendMessageInput{
			MessageBody:  aws.String(wfID),
			QueueUrl:     aws.String(sqsQueueURL),
			DelaySeconds: aws.Int64(updateLoopDelay),
		})
		if err != nil {
			return "", err
		}
	}

	// Delete processed message from queue
	_, err = sqsapi.DeleteMessageWithContext(ctx, &sqs.DeleteMessageInput{
		QueueUrl:      aws.String(sqsQueueURL),
		ReceiptHandle: m.ReceiptHandle,
	})
	if err != nil {
		return "", err
	}

	return wfID, nil
}
