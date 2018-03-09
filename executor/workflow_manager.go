package executor

import (
	"context"
	"sync"
	"time"

	"github.com/Clever/workflow-manager/gen-go/models"
	"github.com/Clever/workflow-manager/resources"
	"github.com/Clever/workflow-manager/store"
	"gopkg.in/Clever/kayvee-go.v6/logger"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/aws/aws-sdk-go/service/sqs/sqsiface"
)

// WorkflowManager is the interface for creating, stopping and checking status for Workflows
type WorkflowManager interface {
	CreateWorkflow(def models.WorkflowDefinition, input string, namespace string, queue string, tags map[string]interface{}) (*models.Workflow, error)
	RetryWorkflow(workflow models.Workflow, startAt, input string) (*models.Workflow, error)
	CancelWorkflow(workflow *models.Workflow, reason string) error
	UpdateWorkflowSummary(workflow *models.Workflow) error
	UpdateWorkflowHistory(workflow *models.Workflow) error
}

// PollForPendingWorkflowsAndUpdateStore polls an SQS queue for workflows needing an update.
// It will stop polling when the context is done.
func PollForPendingWorkflowsAndUpdateStore(ctx context.Context, wm WorkflowManager, thestore store.Store, sqsapi sqsiface.SQSAPI, sqsQueueURL string) {
	ticker := time.NewTicker(500 * time.Millisecond)
	for {
		select {
		case <-ctx.Done():
			log.InfoD("poll-for-pending-workflows-done", logger.M{})
			ticker.Stop()
			return
		case <-ticker.C:
			out, err := sqsapi.ReceiveMessageWithContext(ctx, &sqs.ReceiveMessageInput{
				MaxNumberOfMessages: aws.Int64(10),
				QueueUrl:            aws.String(sqsQueueURL),
			})
			if err != nil {
				log.ErrorD("poll-for-pending-workflows", logger.M{"error": err.Error()})
			}

			var wg sync.WaitGroup
			wg.Add(len(out.Messages))
			for _, message := range out.Messages {
				go func(m *sqs.Message) {
					defer wg.Done()
					if id, err := updatePendingWorkflow(ctx, m, wm, thestore, sqsapi, sqsQueueURL); err != nil {
						log.ErrorD("update-pending-workflow", logger.M{"id": id, "error": err.Error()})
					} else {
						log.InfoD("update-pending-workflow", logger.M{"id": id})
					}
				}(message)
			}
			wg.Wait()
		}
	}
}

// updateLoopDelay is the minimum amount of time between each update to a
// workflow's state. State is sync'd from workflow manager's backend, Step Functions.
const updateLoopDelay = 30

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
	wf, err := thestore.GetWorkflowByID(wfID)
	if err != nil {
		return "", err
	}

	err = wm.UpdateWorkflowSummary(&wf)
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
