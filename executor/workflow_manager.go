package executor

import (
	"context"
	"fmt"
	"time"

	"github.com/Clever/workflow-manager/gen-go/models"
	"github.com/Clever/workflow-manager/resources"
	"github.com/Clever/workflow-manager/store"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
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

// PollForPendingWorkflowsAndUpdateStore polls an SQS queue for workflows needing an update.
// It will stop polling when the context is done.
func PollForPendingWorkflowsAndUpdateStore(ctx context.Context, wm WorkflowManager, thestore store.Store, sqsapi sqsiface.SQSAPI, sqsQueueURL string) {
	tracer := otel.GetTracerProvider().Tracer("workflow-update-loop")
	for {
		select {
		case <-ctx.Done():
			log.Info("poll-for-pending-workflows-done")
			return
		default:
			innerCtx, span := tracer.Start(ctx, "update-pending-workflows")
			out, err := sqsapi.ReceiveMessageWithContext(innerCtx, &sqs.ReceiveMessageInput{
				MaxNumberOfMessages: aws.Int64(10),
				QueueUrl:            aws.String(sqsQueueURL),
				WaitTimeSeconds:     aws.Int64(10),
			})
			if err != nil {
				log.ErrorD("poll-for-pending-workflows", logger.M{"error": err.Error()})
				span.SetStatus(codes.Error, err.Error())
			}

			for _, message := range out.Messages {
				if id, err := updatePendingWorkflow(innerCtx, message, wm, thestore, sqsapi, sqsQueueURL); err != nil {
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
			span.End()
		}
	}
}

// updateLoopDelay is the minimum amount of time between each update to a
// workflow's state. State is sync'd from workflow manager's backend, Step Functions.
const updateLoopDelay = 60

func createPendingWorkflow(ctx context.Context, workflowID string, sqsapi sqsiface.SQSAPI, sqsQueueURL string) error {
	message, err := sqsapi.SendMessageWithContext(ctx, &sqs.SendMessageInput{
		MessageBody:  aws.String(workflowID),
		QueueUrl:     aws.String(sqsQueueURL),
		DelaySeconds: aws.Int64(updateLoopDelay),
	})
	if err != nil {
		log.ErrorD("sqs-send-message",
			logger.M{
				"error":       err.Error(),
				"workflow-id": workflowID,
				"message-id":  aws.StringValue(message.MessageId),
			})
	}
	return err
}

func updatePendingWorkflow(ctx context.Context, m *sqs.Message, wm WorkflowManager, thestore store.Store, sqsapi sqsiface.SQSAPI, sqsQueueURL string) (string, error) {
	tracer := otel.GetTracerProvider().Tracer("workflow-update-loop")
	ctx, span := tracer.Start(ctx, "workflow-update")
	defer span.End()
	deleteMsg := func() {
		if _, err := sqsapi.DeleteMessageWithContext(ctx, &sqs.DeleteMessageInput{
			QueueUrl:      aws.String(sqsQueueURL),
			ReceiptHandle: m.ReceiptHandle,
		}); err != nil {
			log.ErrorD("delete-message", logger.M{"error": err.Error()})
		}
	}
	requeueMsg := func() {
		if _, err := sqsapi.SendMessageWithContext(ctx, &sqs.SendMessageInput{
			MessageBody:  m.Body,
			QueueUrl:     aws.String(sqsQueueURL),
			DelaySeconds: aws.Int64(updateLoopDelay),
		}); err != nil {
			log.ErrorD("send-message", logger.M{"error": err.Error(), "workflow-id": aws.StringValue(m.Body)})
		}
	}

	wfID := *m.Body
	span.SetAttributes(attribute.String("workflow-id", wfID))
	wf, err := thestore.GetWorkflowByID(ctx, wfID)
	if err != nil {
		if _, ok := err.(models.NotFound); ok {
			// workflow has disappeared from our DB. No sense in
			// trying to update it again, so delete the SQS message
			// since the update loop starts before starting execution,
			// this could indicate an error in starting the workflow.
			// if that happened, it is logged separately.
			deleteMsg()
			span.SetAttributes(attribute.String("result", "workflow-not-found"))
			return "", fmt.Errorf("workflow id not found: %s", wfID)
		}
		// other error, e.g. throttling. Try again later
		requeueMsg()
		deleteMsg()
		span.SetAttributes(attribute.String("result", "database-error"))
		return "", err
	}

	logPendingWorkflowUpdateLag(wf)

	var storeSaveFailed = true
	// Attempt to update the workflow, i.e. sync data from SFN into our workflow object.
	// Whether or not we are successful at this, we should delete the sqs message
	// and re-queue a new message if the workflow remains pending.
	defer func() {
		if storeSaveFailed {
			span.SetAttributes(attribute.String("result", "requeue-store-save-failed"))
			requeueMsg()
		} else if !resources.WorkflowStatusIsDone(&wf) {
			span.SetAttributes(attribute.String("result", "requeue-workflow-not-done"))
			requeueMsg()
		}
		deleteMsg()
	}()
	if err := wm.UpdateWorkflowSummary(ctx, &wf); err != nil {
		return "", err
	}
	storeSaveFailed = false
	return wfID, nil
}
