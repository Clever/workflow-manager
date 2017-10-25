package executor

import (
	"context"
	"time"

	"github.com/Clever/workflow-manager/gen-go/models"
	"github.com/Clever/workflow-manager/store"
	"gopkg.in/Clever/kayvee-go.v6/logger"
)

// WorkflowManager is the interface for creating, stopping and checking status for Workflows
type WorkflowManager interface {
	CreateWorkflow(def models.WorkflowDefinition, input string, namespace string, queue string, tags map[string]interface{}) (*models.Workflow, error)
	RetryWorkflow(workflow models.Workflow, startAt, input string) (*models.Workflow, error)
	CancelWorkflow(workflow *models.Workflow, reason string) error
	UpdateWorkflowStatus(workflow *models.Workflow) error
}

// PollForPendingWorkflowsAndUpdateStore polls the store for workflows in a pending state and
// attempts to update them. It will stop polling when the context is done.
func PollForPendingWorkflowsAndUpdateStore(ctx context.Context, wm WorkflowManager, thestore store.Store) {
	ticker := time.NewTicker(5 * time.Second)
	for {
		select {
		case <-ctx.Done():
			log.InfoD("poll-for-pending-workflows-done", logger.M{})
			ticker.Stop()
			return
		case <-ticker.C:
			if id, err := checkPendingWorkflows(wm, thestore); err != nil {
				log.ErrorD("poll-for-pending-workflows", logger.M{"id": id, "error": err.Error()})
			} else {
				log.InfoD("poll-for-pending-workflows", logger.M{"id": id})
			}
		}
	}
}

func lockAvailableWorkflow(thestore store.Store, workflowIDs []string) (string, error) {
	for _, wfID := range workflowIDs {
		if err := thestore.LockWorkflow(wfID); err == nil {
			return wfID, nil
		} else if err != store.ErrWorkflowLocked {
			// an error reading from the Store
			return "", err
		}
	}
	return "", nil
}

func checkPendingWorkflows(wm WorkflowManager, thestore store.Store) (string, error) {
	wfIDs, err := thestore.GetPendingWorkflowIDs()
	if err != nil {
		return "", err
	}

	// attempt to lock one of the workflows for updating
	wfLockedID, err := lockAvailableWorkflow(thestore, wfIDs)
	if err != nil {
		return "", err
	}
	if wfLockedID == "" {
		log.InfoD("pending-workflows-noop", logger.M{"pending": len(wfIDs)})
		return "", nil
	}

	log.InfoD("pending-workflows-locked", logger.M{"id": wfLockedID})
	defer func() {
		log.InfoD("pending-workflows-unlocked", logger.M{"id": wfLockedID})
		if err := thestore.UnlockWorkflow(wfLockedID); err != nil {
			log.ErrorD("pending-workflows-unlock-error", logger.M{"id": wfLockedID, "error": err.Error()})
		}
	}()

	wf, err := thestore.GetWorkflowByID(wfLockedID)
	if err != nil {
		return wfLockedID, err
	}

	err = wm.UpdateWorkflowStatus(&wf)
	return wfLockedID, err
}
