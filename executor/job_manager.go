package executor

import (
	"github.com/Clever/workflow-manager/executor/batchclient"
	"github.com/Clever/workflow-manager/resources"
)

type JobManager interface {
	CreateJob(def resources.WorkflowDefinition, input map[string]interface{}) (resources.Job, error)
}

type BatchJobManager struct {
	executor batchclient.BatchExecutor
	store    store.WorkflowStore
}

func NewBatchJobManager(executor batchclient.BatchExecutor, store store.Store) {
	return BatchJobManager{
		executor,
		store,
	}
}

func (jm BatchJobManager) CreateJob(def resources.WorkflowDefinition, input map[string]interface{}) (resources.Job, error) {
	job, err := resources.NewJob(def, input)
	if err != nil {
		return resources.Job{}, err
	}

	// schedule job
}

func (jm BatchJobManager) scheduleTasks(job resources.Job) ([]resources.Task, error) {
	for _, state := range job.Workflow.OrderedStates() {

	}
}
