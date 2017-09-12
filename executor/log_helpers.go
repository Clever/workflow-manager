package executor

import (
	"github.com/Clever/kayvee-go/logger"
	"github.com/Clever/workflow-manager/resources"
)

var log = logger.New("workflow-manager")

func logJobStatus(job *resources.Job, workflow *resources.Workflow) {
	log.InfoD("job-status", logger.M{
		"id":            job.ID,
		"workflow-id":   workflow.ID,
		"workflow-name": workflow.WorkflowDefinition.Name(),
		"state":         job.State,
		"status":        job.Status,
		// 0 -> running; 1 -> failed;
		// -1 -> cancelled by user; -2 -> abort due to dependecy failure
		"value": job.StatusToInt(),
	})
}

func logWorkflowStatusChange(workflow *resources.Workflow, previousStatus resources.WorkflowStatus) {
	// If the status was not changed, ignore logging
	if previousStatus == workflow.Status {
		return
	}

	log.InfoD("workflow-status-change", logger.M{
		"id":              workflow.ID,
		"name":            workflow.WorkflowDefinition.Name(),
		"version":         workflow.WorkflowDefinition.Version(),
		"previous-status": previousStatus,
		"status":          workflow.Status,
		// 0 -> running; 1 -> failed; -1 -> cancelled
		"value": workflow.StatusToInt(),
	})
}
