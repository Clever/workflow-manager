package executor

import (
	"fmt"
	"time"

	"gopkg.in/Clever/kayvee-go.v6/logger"

	"github.com/Clever/workflow-manager/resources"
	"github.com/Clever/workflow-manager/store"
)

// JobManager in the interface for creating, stopping and checking status for Jobs (workflow runs)
type JobManager interface {
	CreateJob(def resources.WorkflowDefinition, input []string, namespace string, queue string) (*resources.Job, error)
	CancelJob(job *resources.Job, reason string) error
	UpdateJobStatus(job *resources.Job) error
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

// UpdateJobStatus ensures that the status of the tasks is in-sync with AWS Batch and sets Job status
func (jm BatchJobManager) UpdateJobStatus(job *resources.Job) error {
	// Because batch only keeps a 24hr history of tasks, don't look them up if
	// they are already in a final state. NOTE: job status should already be in
	// its final state as well at this point since it gets updated after tasks
	// statuses are read from batch
	if job.IsDone() {
		return nil
	}

	// copy current status
	taskStatus := map[string]resources.TaskStatus{}
	taskAttempts := map[string]int{}
	for _, task := range job.Tasks {
		taskStatus[task.ID] = task.Status
		taskAttempts[task.ID] = len(task.Attempts)
	}

	// fetch new status from batch
	errs := jm.executor.Status(job.Tasks)
	if len(errs) > 0 {
		return fmt.Errorf("Failed to update status for %d tasks. errors: %s", len(errs), errs)
	}

	// If no task status has changed then there is no need to update the job status
	noChanges := true
	for _, task := range job.Tasks {
		if task.Status != taskStatus[task.ID] || len(task.Attempts) != taskAttempts[task.ID] {
			noChanges = false
		}
	}
	if noChanges {
		return nil
	}

	// If the job has been canceled, do not update its status to something else
	if job.Status == resources.Cancelled {
		return jm.store.UpdateJob(*job)
	}

	jobSuccess := true
	jobRunning := false
	jobFailed := false
	jobCancelled := false
	for _, task := range job.Tasks {
		logTaskStatus(task, job)
		if task.Status != resources.TaskStatusSucceeded {
			// all tasks should be successful for job success
			jobSuccess = false
		}
		if task.Status == resources.TaskStatusRunning {
			// any task running means running
			jobRunning = true
		}
		if task.Status == resources.TaskStatusFailed ||
			task.Status == resources.TaskStatusAborted {
			// any task failure results in the job being failed
			jobFailed = true
		}
		if task.Status == resources.TaskStatusUserAborted {
			// if any task is aborted by user, we should mark the job as cancelled
			jobCancelled = true
		}
	}

	previousStatus := job.Status
	if jobCancelled {
		job.Status = resources.Cancelled
	} else if jobFailed {
		job.Status = resources.Failed
	} else if jobRunning {
		job.Status = resources.Running
	} else if jobSuccess {
		job.Status = resources.Succeeded
	}

	// log if changed and save to datastore
	logJobStatusChange(job, previousStatus)
	return jm.store.UpdateJob(*job)
}

// CreateJob can be used to create a new job for a workflow
func (jm BatchJobManager) CreateJob(def resources.WorkflowDefinition, input []string, namespace string, queue string) (*resources.Job, error) {
	job := resources.NewJob(def, input) // TODO: add namespace to Job struct
	logJobStatusChange(job, "")

	stateResources, err := jm.getStateResources(job, namespace)
	if err != nil {
		return &resources.Job{}, err
	}

	err = jm.scheduleTasks(job, stateResources, input, queue)
	if err != nil {
		return &resources.Job{}, err
	}

	// TODO: fails we should either
	// 1. reconcile somehow with the scheduled tasks
	// 2. kill the running tasks so that we don't have orphan tasks in AWS Batch
	err = jm.store.SaveJob(*job)

	// TODO: remove this polling and replace by ECS task event processing
	go jm.pollUpdateStatus(job)

	return job, err
}

func (jm BatchJobManager) pollUpdateStatus(job *resources.Job) {
	for {
		if job.IsDone() {
			// no need to poll anymore
			log.InfoD("job-polling-stop", logger.M{
				"id":     job.ID,
				"status": job.Status,
				// TODO: update logs from workflow=>workflow-definition (including kvconfig.yml routing)
				"workflow": job.WorkflowDefinition.Name(),
			})
			break
		}
		if err := jm.UpdateJobStatus(job); err != nil {
			log.ErrorD("job-polling-error", logger.M{
				"id":       job.ID,
				"status":   job.Status,
				"workflow": job.WorkflowDefinition.Name(),
				"error":    err.Error(),
			})
		}
		time.Sleep(time.Minute)
	}
}

func (jm BatchJobManager) CancelJob(job *resources.Job, reason string) error {
	// TODO: don't cancel already succeeded tasks
	tasks := []*resources.Task{}
	for _, task := range job.Tasks {
		switch task.Status {
		case resources.TaskStatusCreated,
			resources.TaskStatusQueued,
			resources.TaskStatusRunning,
			resources.TaskStatusWaiting:

			tasks = append(tasks, task)
		}
	}

	errs := jm.executor.Cancel(tasks, reason)
	if len(errs) < len(tasks) {
		// TODO: this assumes that a workflow is linear. One task cancellation
		// will lead to all subsequent tasks failing
		previousStatus := job.Status
		job.Status = resources.Cancelled
		logJobStatusChange(job, previousStatus)
	}
	jm.store.UpdateJob(*job)

	if len(errs) > 0 {
		return fmt.Errorf("%d of %d tasks were not canceled", len(errs), len(tasks))
	}

	return nil
}

func (jm BatchJobManager) scheduleTasks(job *resources.Job,
	stateResources map[string]resources.StateResource, input []string, queue string) error {

	tasks := map[string]*resources.Task{}

	for i, state := range job.WorkflowDefinition.OrderedStates() {
		deps := []string{}

		for _, d := range state.Dependencies() {
			if _, ok := tasks[d]; !ok {
				return fmt.Errorf("Failed to start state %s. Dependency task for `%s` not found", state.Name(), d)
			}
			deps = append(deps, tasks[d].ID)
		}
		var taskID, taskName string
		var taskInput []string
		var err error
		// TODO: this should be limited to 50 characters due to a bug in the interaction between Batch
		// and ECS. {namespace--app} for now, to enable easy parsing across workflows
		if stateResources[state.Name()].Namespace == "" {
			taskName = fmt.Sprintf("default--%s", stateResources[state.Name()].Name)
		} else {
			taskName = fmt.Sprintf("%s--%s",
				stateResources[state.Name()].Namespace, stateResources[state.Name()].Name)
		}
		taskDefinition := stateResources[state.Name()].URI

		// TODO: use job.WorkflowDefinition.StartAt
		// if first job pass in an input
		if i == 0 {
			taskInput = input
		}

		// determine if the state has a retry strategy
		// currently we just support a retry strategy corresponding to a number of attempts
		var attempts int64
		if len(state.Retry()) == 1 {
			retrier := state.Retry()[0]
			if retrier.MaxAttempts != nil {
				attempts = *retrier.MaxAttempts
			}
		}

		taskID, err = jm.executor.SubmitJob(taskName, taskDefinition, deps, taskInput, queue, attempts)
		if err != nil {
			// TODO: cancel jobs that have already been posted for idempotency
			return err
		}
		// create a Task with the Id returned by AWS
		task := resources.NewTask(taskID, taskName, state.Name(), stateResources[state.Name()], taskInput)
		tasks[state.Name()] = task
		job.AddTask(task)
	}

	return nil
}

// getStateResources fetches JobDefinition URIs for each state
// from store.StateResource if namespace is set. If namespace is NOT
// defined then StateResource objects are created with URI = state.Resource
//
// This behavior allows to shortcircuit use of the StateResource database and provide
// Resource URIs directly in the WorkflowDefinition
func (jm BatchJobManager) getStateResources(job *resources.Job,
	namespace string) (map[string]resources.StateResource, error) {

	stateResources := map[string]resources.StateResource{}

	if namespace == "" {
		// assume State.Resource is a URI
		for _, state := range job.WorkflowDefinition.States() {
			stateResources[state.Name()] = resources.NewBatchResource(
				state.Name(),
				"",
				state.Resource(),
			)
		}

		return stateResources, nil
	}

	// fetch each of the StateResource objects using the namespace
	// and State.Resource name.
	// Could be faster with the store supporting a BatchGetStateResource([]names, namespace)
	for _, state := range job.WorkflowDefinition.OrderedStates() {
		stateResource, err := jm.store.GetStateResource(state.Resource(), namespace)
		if err != nil {
			return stateResources, fmt.Errorf("StateResource `%s:%s` Not Found: %s",
				namespace, state.Resource(), err)
		}
		stateResources[state.Name()] = stateResource
	}

	return stateResources, nil
}
