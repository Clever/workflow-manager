package executor

import (
	"github.com/Clever/kayvee-go/logger"
	"github.com/Clever/workflow-manager/resources"
)

var log = logger.New("workflow-manager")

func logTaskStatus(task *resources.Task, job *resources.Job) {
	log.InfoD("task-status", logger.M{
		"id":       job.ID,
		"workflow": job.WorkflowDefinition.Name(),
		"state":    task.State,
		"status":   task.Status,
		// 0 -> running; 1 -> failed;
		// -1 -> cancelled by user; -2 -> abort due to dependecy failure
		"value": task.StatusToInt(),
	})
}

func logJobStatusChange(job *resources.Job, previousStatus resources.JobStatus) {
	// If the status was not changed, ignore logging
	if previousStatus == job.Status {
		return
	}

	log.InfoD("job-status-change", logger.M{
		"id":               job.ID,
		"workflow":         job.WorkflowDefinition.Name(),
		"workflow-version": job.WorkflowDefinition.Version(),
		"previous-status":  previousStatus,
		"status":           job.Status,
		// 0 -> running; 1 -> failed; -1 -> cancelled
		"value": job.StatusToInt(),
	})
}
