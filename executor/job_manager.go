package executor

import (
	"fmt"

	"github.com/Clever/workflow-manager/resources"
	"github.com/Clever/workflow-manager/store"
)

// JobManager in the interface for creating, stopping and checking status for Jobs (workflow runs)
type JobManager interface {
	CreateJob(def resources.WorkflowDefinition, input string) (*resources.Job, error)
}

// BatchJobManager implements JobManager using the AWS Batch client
type BatchJobManager struct {
	executor Executor
	store    store.Store
}

// NewBatchJobManager creates a JobManager using the AWS Batch client and a Store
func NewBatchJobManager(executor Executor, store store.Store) BatchJobManager {
	return BatchJobManager{
		executor,
		store,
	}
}

// CreateJob can be used to create a new job for a workflow
func (jm BatchJobManager) CreateJob(def resources.WorkflowDefinition, input string) (*resources.Job, error) {
	job := resources.NewJob(def, input)

	err := jm.scheduleTasks(job, input)
	if err != nil {
		return &resources.Job{}, err
	}

	return job, nil
}

func (jm BatchJobManager) scheduleTasks(job *resources.Job, input string) error {
	tasks := map[string]*resources.Task{}

	for i, state := range job.Workflow.OrderedStates() {
		deps := []string{}

		for _, d := range state.Dependencies() {
			if _, ok := tasks[d]; !ok {
				return fmt.Errorf("Failed to start state %s. Dependency task for `%s` not found", state.Name(), d)
			}
			deps = append(deps, tasks[d].ID)
		}
		var taskID string
		var taskInput string
		var err error
		taskName := fmt.Sprintf("%s--%s", job.ID, state.Name())

		// TODO: use job.Workflow.StartAt
		// if first job pass in an input
		if i == 0 {
			taskInput = input
		}
		taskID, err = jm.executor.SubmitJob(fmt.Sprintf("%s--%s", job.ID, state.Name()), state.Resource(), deps, taskInput)
		if err != nil {
			// TODO: cancel jobs that have already been posted for idempotency
			return err
		}
		// create a Task with the Id returned by AWS
		task := resources.NewTask(taskID, taskName, state.Name(), taskInput)
		tasks[state.Name()] = task
		job.AddTask(task)
	}

	return nil
}
