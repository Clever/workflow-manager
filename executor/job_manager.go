package executor

import (
	"fmt"
	"time"

	"gopkg.in/Clever/kayvee-go.v6/logger"

	"github.com/Clever/workflow-manager/resources"
	"github.com/Clever/workflow-manager/store"
)

// WorkflowManager in the interface for creating, stopping and checking status for Workflows
type WorkflowManager interface {
	CreateWorkflow(def resources.WorkflowDefinition, input []string, namespace string, queue string) (*resources.Workflow, error)
	CancelWorkflow(workflow *resources.Workflow, reason string) error
	UpdateWorkflowStatus(workflow *resources.Workflow) error
}

// BatchWorkflowManager implements WorkflowManager using the AWS Batch client
type BatchWorkflowManager struct {
	executor Executor
	store    store.Store
}

// NewBatchWorkflowManager creates a WorkflowManager using the AWS Batch client and a Store
func NewBatchWorkflowManager(executor Executor, store store.Store) BatchWorkflowManager {
	return BatchWorkflowManager{
		executor,
		store,
	}
}

// UpdateWorkflowStatus ensures that the status of the tasks is in-sync with AWS Batch and sets Workflow status
func (wm BatchWorkflowManager) UpdateWorkflowStatus(workflow *resources.Workflow) error {
	// Because batch only keeps a 24hr history of tasks, don't look them up if
	// they are already in a final state. NOTE: workflow status should already be in
	// its final state as well at this point since it gets updated after tasks
	// statuses are read from batch
	if workflow.IsDone() {
		return nil
	}

	// copy current status
	taskStatus := map[string]resources.TaskStatus{}
	taskAttempts := map[string]int{}
	for _, task := range workflow.Tasks {
		taskStatus[task.ID] = task.Status
		taskAttempts[task.ID] = len(task.Attempts)
	}

	// fetch new status from batch
	errs := wm.executor.Status(workflow.Tasks)
	if len(errs) > 0 {
		return fmt.Errorf("Failed to update status for %d tasks. errors: %s", len(errs), errs)
	}

	// If no task status has changed then there is no need to update the workflow status
	noChanges := true
	for _, task := range workflow.Tasks {
		if task.Status != taskStatus[task.ID] || len(task.Attempts) != taskAttempts[task.ID] {
			noChanges = false
		}
	}
	if noChanges {
		return nil
	}

	// If the workflow has been canceled, do not update its status to something else
	if workflow.Status == resources.Cancelled {
		return wm.store.UpdateWorkflow(*workflow)
	}

	workflowSuccess := true
	workflowRunning := false
	workflowFailed := false
	workflowCancelled := false
	for _, task := range workflow.Tasks {
		logTaskStatus(task, workflow)
		if task.Status != resources.TaskStatusSucceeded {
			// all tasks should be successful for workflow success
			workflowSuccess = false
		}
		if task.Status == resources.TaskStatusRunning {
			// any task running means running
			workflowRunning = true
		}
		if task.Status == resources.TaskStatusFailed ||
			task.Status == resources.TaskStatusAborted {
			// any task failure results in the workflow being failed
			workflowFailed = true
		}
		if task.Status == resources.TaskStatusUserAborted {
			// if any task is aborted by user, we should mark the workflow as cancelled
			workflowCancelled = true
		}
	}

	previousStatus := workflow.Status
	if workflowCancelled {
		workflow.Status = resources.Cancelled
	} else if workflowFailed {
		workflow.Status = resources.Failed
	} else if workflowRunning {
		workflow.Status = resources.Running
	} else if workflowSuccess {
		workflow.Status = resources.Succeeded
	}

	// log if changed and save to datastore
	logWorkflowStatusChange(workflow, previousStatus)
	return wm.store.UpdateWorkflow(*workflow)
}

// CreateWorkflow can be used to create a new workflow for a given WorkflowDefinition
func (wm BatchWorkflowManager) CreateWorkflow(def resources.WorkflowDefinition, input []string, namespace string, queue string) (*resources.Workflow, error) {
	workflow := resources.NewWorkflow(def, input) // TODO: add namespace to Workflow struct
	logWorkflowStatusChange(workflow, "")

	stateResources, err := wm.getStateResources(workflow, namespace)
	if err != nil {
		return &resources.Workflow{}, err
	}

	err = wm.scheduleTasks(workflow, stateResources, input, queue)
	if err != nil {
		return &resources.Workflow{}, err
	}

	// TODO: fails we should either
	// 1. reconcile somehow with the scheduled tasks
	// 2. kill the running tasks so that we don't have orphan tasks in AWS Batch
	err = wm.store.SaveWorkflow(*workflow)

	// TODO: remove this polling and replace by ECS task event processing
	go wm.pollUpdateStatus(workflow)

	return workflow, err
}

func (wm BatchWorkflowManager) pollUpdateStatus(workflow *resources.Workflow) {
	for {
		if workflow.IsDone() {
			// no need to poll anymore
			log.InfoD("job-polling-stop", logger.M{
				"id":     workflow.ID,
				"status": workflow.Status,
				// TODO: update logs from workflow=>workflow-definition (including kvconfig.yml routing)
				"workflow": workflow.WorkflowDefinition.Name(),
			})
			break
		}
		if err := wm.UpdateWorkflowStatus(workflow); err != nil {
			log.ErrorD("job-polling-error", logger.M{
				"id":       workflow.ID,
				"status":   workflow.Status,
				"workflow": workflow.WorkflowDefinition.Name(),
				"error":    err.Error(),
			})
		}
		time.Sleep(time.Minute)
	}
}

func (wm BatchWorkflowManager) CancelWorkflow(workflow *resources.Workflow, reason string) error {
	// TODO: don't cancel already succeeded tasks
	tasks := []*resources.Task{}
	for _, task := range workflow.Tasks {
		switch task.Status {
		case resources.TaskStatusCreated,
			resources.TaskStatusQueued,
			resources.TaskStatusRunning,
			resources.TaskStatusWaiting:

			tasks = append(tasks, task)
		}
	}

	errs := wm.executor.Cancel(tasks, reason)
	if len(errs) < len(tasks) {
		// TODO: this assumes that a workflow is linear. One task cancellation
		// will lead to all subsequent tasks failing
		previousStatus := workflow.Status
		workflow.Status = resources.Cancelled
		logWorkflowStatusChange(workflow, previousStatus)
	}
	wm.store.UpdateWorkflow(*workflow)

	if len(errs) > 0 {
		return fmt.Errorf("%d of %d tasks were not canceled", len(errs), len(tasks))
	}

	return nil
}

func (wm BatchWorkflowManager) scheduleTasks(workflow *resources.Workflow,
	stateResources map[string]resources.StateResource, input []string, queue string) error {

	tasks := map[string]*resources.Task{}

	for i, state := range workflow.WorkflowDefinition.OrderedStates() {
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

		// TODO: use workflow.WorkflowDefinition.StartAt
		// if first workflow pass in an input
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

		taskID, err = wm.executor.SubmitWorkflow(taskName, taskDefinition, deps, taskInput, queue, attempts)
		if err != nil {
			// TODO: cancel workflows that have already been posted for idempotency
			return err
		}
		// create a Task with the Id returned by AWS
		task := resources.NewTask(taskID, taskName, state.Name(), stateResources[state.Name()], taskInput)
		tasks[state.Name()] = task
		workflow.AddTask(task)
	}

	return nil
}

// getStateResources fetches WorkflowDefinition URIs for each state
// from store.StateResource if namespace is set. If namespace is NOT
// defined then StateResource objects are created with URI = state.Resource
//
// This behavior allows to shortcircuit use of the StateResource database and provide
// Resource URIs directly in the WorkflowDefinition
func (wm BatchWorkflowManager) getStateResources(workflow *resources.Workflow,
	namespace string) (map[string]resources.StateResource, error) {

	stateResources := map[string]resources.StateResource{}

	if namespace == "" {
		// assume State.Resource is a URI
		for _, state := range workflow.WorkflowDefinition.States() {
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
	for _, state := range workflow.WorkflowDefinition.OrderedStates() {
		stateResource, err := wm.store.GetStateResource(state.Resource(), namespace)
		if err != nil {
			return stateResources, fmt.Errorf("StateResource `%s:%s` Not Found: %s",
				namespace, state.Resource(), err)
		}
		stateResources[state.Name()] = stateResource
	}

	return stateResources, nil
}
