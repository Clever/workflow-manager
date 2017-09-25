package executor

import (
	"fmt"

	"github.com/Clever/workflow-manager/resources"
	"github.com/Clever/workflow-manager/store"
)

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

// UpdateWorkflowStatus ensures that the status of the jobs is in-sync with AWS Batch and sets Workflow status
func (wm BatchWorkflowManager) UpdateWorkflowStatus(workflow *resources.Workflow) error {
	// Because batch only keeps a 24hr history of jobs, don't look them up if
	// they are already in a final state. NOTE: workflow status should already be in
	// its final state as well at this point since it gets updated after jobs
	// statuses are read from batch
	if workflow.IsDone() {
		return nil
	}

	// copy current status
	jobStatus := map[string]resources.JobStatus{}
	jobAttempts := map[string]int{}
	for _, job := range workflow.Jobs {
		jobStatus[job.ID] = job.Status
		jobAttempts[job.ID] = len(job.Attempts)
	}

	// fetch new status from batch
	errs := wm.executor.Status(workflow.Jobs)
	if len(errs) > 0 {
		return fmt.Errorf("Failed to update status for %d jobs. errors: %s", len(errs), errs)
	}

	// If no job status has changed then there is no need to update the workflow status
	noChanges := true
	for _, job := range workflow.Jobs {
		if job.Status != jobStatus[job.ID] || len(job.Attempts) != jobAttempts[job.ID] {
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
	for _, job := range workflow.Jobs {
		logJobStatus(job, workflow)
		if job.Status != resources.JobStatusSucceeded {
			// all jobs should be successful for workflow success
			workflowSuccess = false
		}
		if job.Status == resources.JobStatusRunning {
			// any job running means running
			workflowRunning = true
		}
		if job.Status == resources.JobStatusFailed ||
			job.Status == resources.JobStatusAborted {
			// any job failure results in the workflow being failed
			workflowFailed = true
		}
		if job.Status == resources.JobStatusUserAborted {
			// if any job is aborted by user, we should mark the workflow as cancelled
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
	workflow := resources.NewWorkflow(def, input, namespace, queue)
	logWorkflowStatusChange(workflow, "")

	stateResources, err := wm.getStateResources(workflow, namespace)
	if err != nil {
		return &resources.Workflow{}, err
	}

	err = wm.scheduleJobs(workflow, stateResources, input, queue)
	if err != nil {
		return &resources.Workflow{}, err
	}

	// TODO: fails we should either
	// 1. reconcile somehow with the scheduled jobs
	// 2. kill the running jobs so that we don't have orphan jobs in AWS Batch
	err = wm.store.SaveWorkflow(*workflow)

	return workflow, err
}

func (wm BatchWorkflowManager) CancelWorkflow(workflow *resources.Workflow, reason string) error {
	// TODO: don't cancel already succeeded jobs
	jobs := []*resources.Job{}
	for _, job := range workflow.Jobs {
		switch job.Status {
		case resources.JobStatusCreated,
			resources.JobStatusQueued,
			resources.JobStatusRunning,
			resources.JobStatusWaiting:

			jobs = append(jobs, job)
		}
	}

	errs := wm.executor.Cancel(jobs, reason)
	if len(errs) < len(jobs) {
		// TODO: this assumes that a workflow is linear. One job cancellation
		// will lead to all subsequent jobs failing
		previousStatus := workflow.Status
		workflow.Status = resources.Cancelled
		logWorkflowStatusChange(workflow, previousStatus)
	}
	wm.store.UpdateWorkflow(*workflow)

	if len(errs) > 0 {
		return fmt.Errorf("%d of %d jobs were not canceled", len(errs), len(jobs))
	}

	return nil
}

func (wm BatchWorkflowManager) scheduleJobs(workflow *resources.Workflow,
	stateResources map[string]resources.StateResource, input []string, queue string) error {

	jobs := map[string]*resources.Job{}

	for i, state := range workflow.WorkflowDefinition.OrderedStates() {
		deps := []string{}

		for _, d := range state.Dependencies() {
			if _, ok := jobs[d]; !ok {
				return fmt.Errorf("Failed to start state %s. Dependency job for `%s` not found", state.Name(), d)
			}
			deps = append(deps, jobs[d].ID)
		}
		var jobID, jobName string
		var jobInput []string
		var err error
		// TODO: this should be limited to 50 characters due to a bug in the interaction between Batch
		// and ECS. {namespace--app} for now, to enable easy parsing across workflows
		if stateResources[state.Name()].Namespace == "" {
			jobName = fmt.Sprintf("default--%s", stateResources[state.Name()].Name)
		} else {
			jobName = fmt.Sprintf("%s--%s",
				stateResources[state.Name()].Namespace, stateResources[state.Name()].Name)
		}
		jobDefinition := stateResources[state.Name()].URI

		// TODO: use workflow.WorkflowDefinition.StartAt
		// if first workflow pass in an input
		if i == 0 {
			jobInput = input
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

		jobID, err = wm.executor.SubmitWorkflow(jobName, jobDefinition, deps, jobInput, queue, attempts)
		if err != nil {
			// TODO: cancel workflows that have already been posted for idempotency
			return err
		}
		// create a Job with the Id returned by AWS
		job := resources.NewJob(jobID, jobName, state.Name(), stateResources[state.Name()], jobInput)
		jobs[state.Name()] = job
		workflow.AddJob(job)
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
