package executor

import (
	"github.com/Clever/kayvee-go/logger"
	"github.com/Clever/workflow-manager/resources"
)

var log = logger.New("workflow-manager")

func logJobStatus(job *resources.Job, workflow *resources.Workflow) {
	// TODO: INFRA-2483 - Rename task => job and update kvconfig.yml routing
	log.InfoD("task-status", logger.M{
		"id":       workflow.ID,
		"workflow": workflow.WorkflowDefinition.Name(),
		"state":    job.State,
		"status":   job.Status,
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

	// TODO: INFRA-2483 - Rename job=>workflow and kvconfig.yml routing too
	log.InfoD("job-status-change", logger.M{
		"id":               workflow.ID,
		"workflow":         workflow.WorkflowDefinition.Name(),
		"workflow-version": workflow.WorkflowDefinition.Version(),
		"previous-status":  previousStatus,
		"status":           workflow.Status,
		// 0 -> running; 1 -> failed; -1 -> cancelled
		"value": workflow.StatusToInt(),
	})
}
