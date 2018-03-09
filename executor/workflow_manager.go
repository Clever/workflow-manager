package executor

import (
	"context"
	"fmt"
	"time"

	"github.com/Clever/workflow-manager/gen-go/models"
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
func PollForPendingWorkflowsAndUpdateStore(ctx context.Context, wm WorkflowManager, thestore store.Store, sqsapi sqsiface.SQSAPI) {
	ticker := time.NewTicker(500 * time.Millisecond)
	for {
		select {
		case <-ctx.Done():
			log.InfoD("poll-for-pending-workflows-done", logger.M{})
			ticker.Stop()
			return
		case <-ticker.C:
			if id, err := checkPendingWorkflows(wm, thestore, sqsapi); err != nil {
				log.ErrorD("poll-for-pending-workflows", logger.M{"id": id, "error": err.Error()})
			} else {
				log.InfoD("poll-for-pending-workflows", logger.M{"id": id})
			}
		}
	}
}

func checkPendingWorkflows(wm WorkflowManager, thestore store.Store, sqsapi sqsiface.SQSAPI) (string, error) {
	// TODO: use context?
	out, err := sqsapi.ReceiveMessage(&sqs.ReceiveMessageInput{
		MaxNumberOfMessages: aws.Int64(1),
		QueueUrl:            aws.String("https://sqs.us-west-1.amazonaws.com/589690932525/workflow-manager-update-loop-dev"), // TODO
	})
	if err != nil {
		return "", err
	}

	if len(out.Messages) == 0 {
		return "n/a", nil
	}

	// TODO: update this + MaxNumberOfMessages to loop over many
	m := out.Messages[0]

	fmt.Println(m)
	for key, val := range m.MessageAttributes {
		fmt.Println(key, *val)
	}
	//wfID, ok := m.MessageAttributes["workflow-id"]
	wfID := *m.Body
	//if !ok {
	//// Cleanup malformed messages
	//_, err = sqsapi.DeleteMessage(&sqs.DeleteMessageInput{
	//QueueUrl:      aws.String("https://sqs.us-west-1.amazonaws.com/589690932525/workflow-manager-update-loop-dev"), // TODO
	//ReceiptHandle: m.ReceiptHandle,
	//})
	//if err != nil {
	//return "", err
	//}

	//return "", fmt.Errorf("SQS message lacks 'workflow-id' attribute")
	//}

	wf, err := thestore.GetWorkflowByID(wfID)
	if err != nil {
		return "", err
	}

	err = wm.UpdateWorkflowSummary(&wf)
	if err != nil {
		return "", err
	}

	// Delete message from queue
	_, err = sqsapi.DeleteMessage(&sqs.DeleteMessageInput{
		QueueUrl:      aws.String("https://sqs.us-west-1.amazonaws.com/589690932525/workflow-manager-update-loop-dev"), // TODO
		ReceiptHandle: m.ReceiptHandle,
	})
	if err != nil {
		return "", err
	}

	return wfID, nil
}
