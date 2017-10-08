package executor

import (
	"github.com/Clever/workflow-manager/gen-go/models"
	"github.com/Clever/workflow-manager/resources"
	"gopkg.in/Clever/kayvee-go.v6/logger"
)

var log = logger.New("workflow-manager")

func logJobStatus(job *models.Job, workflow *models.Workflow) {
	log.InfoD("job-status", logger.M{
		"id":            job.ID,
		"workflow-id":   workflow.ID,
		"workflow-name": workflow.WorkflowDefinition.Name,
		"state":         job.State,
		"status":        job.Status,
		// 0 -> running; 1 -> failed;
		// -1 -> cancelled by user; -2 -> abort due to dependecy failure
		"value": resources.JobStatusToInt(job.Status),
	})
}

func logWorkflowStatusChange(workflow *models.Workflow, previousStatus models.WorkflowStatus) {
	// If the status was not changed, ignore logging
	if previousStatus == workflow.Status {
		return
	}

	log.InfoD("workflow-status-change", logger.M{
		"id":              workflow.ID,
		"name":            workflow.WorkflowDefinition.Name,
		"version":         workflow.WorkflowDefinition.Version,
		"previous-status": previousStatus,
		"status":          workflow.Status,
		// 0 -> running; 1 -> failed; -1 -> cancelled
		"value": resources.WorkflowStatusToInt(workflow.Status),
	})
}
