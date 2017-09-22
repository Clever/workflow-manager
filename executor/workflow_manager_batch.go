package executor

import (
	"fmt"

	"github.com/Clever/kayvee-go/logger"
	"github.com/Clever/workflow-manager/gen-go/models"
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
func (wm BatchWorkflowManager) UpdateWorkflowStatus(workflow *models.Workflow) error {
	// Because batch only keeps a 24hr history of jobs, don't look them up if
	// they are already in a final state. NOTE: workflow status should already be in
	// its final state as well at this point since it gets updated after jobs
	// statuses are read from batch
	if resources.WorkflowIsDone(workflow) {
		return nil
	}

	// copy current status
	jobStatus := map[string]models.JobStatus{}
	jobAttempts := map[string]int{}
	for _, job := range workflow.Jobs {
		jobStatus[job.ID] = job.Status
		jobAttempts[job.ID] = len(job.Attempts)
	}

	// fetch new status from batch
	errs := wm.executor.Status(workflow.Jobs)
	if len(errs) > 0 {
		log.ErrorD("executor-status-errs", logger.M{"errs": fmt.Sprintf("%s", errs)})
		workflow.Status = models.WorkflowStatusFailed
		return wm.store.UpdateWorkflow(*workflow)
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
	if workflow.Status == models.WorkflowStatusCancelled {
		return wm.store.UpdateWorkflow(*workflow)
	}

	workflowSuccess := true
	workflowRunning := false
	workflowFailed := false
	workflowCancelled := false
	for _, job := range workflow.Jobs {
		logJobStatus(job, workflow)
		if job.Status != models.JobStatusSucceeded {
			// all jobs should be successful for workflow success
			workflowSuccess = false
		}
		if job.Status == models.JobStatusRunning {
			// any job running means running
			workflowRunning = true
		}
		if job.Status == models.JobStatusFailed ||
			job.Status == models.JobStatusAbortedDepsFailed {
			// any job failure results in the workflow being failed
			workflowFailed = true
		}
		if job.Status == models.JobStatusAbortedByUser {
			// if any job is aborted by user, we should mark the workflow as cancelled
			workflowCancelled = true
		}
	}

	previousStatus := workflow.Status
	if workflowCancelled {
		workflow.Status = models.WorkflowStatusCancelled
	} else if workflowFailed {
		workflow.Status = models.WorkflowStatusFailed
	} else if workflowRunning {
		workflow.Status = models.WorkflowStatusRunning
	} else if workflowSuccess {
		workflow.Status = models.WorkflowStatusSucceeded
	}

	// log if changed and save to datastore
	logWorkflowStatusChange(workflow, previousStatus)
	return wm.store.UpdateWorkflow(*workflow)
}

// CreateWorkflow can be used to create a new workflow for a given WorkflowDefinition
func (wm BatchWorkflowManager) CreateWorkflow(def models.WorkflowDefinition, input string, namespace string, queue string, tags map[string]interface{}) (*models.Workflow, error) {
	workflow := resources.NewWorkflow(&def, input, namespace, queue, tags)
	logWorkflowStatusChange(workflow, "")

	stateResources, err := wm.getStateResources(workflow, namespace)
	if err != nil {
		return &models.Workflow{}, err
	}

	err = wm.scheduleJobs(workflow, stateResources, input, queue)
	if err != nil {
		return &models.Workflow{}, err
	}

	// TODO: fails we should either
	// 1. reconcile somehow with the scheduled jobs
	// 2. kill the running jobs so that we don't have orphan jobs in AWS Batch
	err = wm.store.SaveWorkflow(*workflow)

	return workflow, err
}

// RetryWorkflow is used to create a new workflow starting at a custom state given an existing Workflow
func (wm BatchWorkflowManager) RetryWorkflow(ogWorkflow models.Workflow, startAt, input string) (*models.Workflow, error) {
	if !resources.WorkflowIsDone(&ogWorkflow) {
		return &models.Workflow{}, fmt.Errorf("Retry not allowed. Workflow state is %s", ogWorkflow.Status)
	}

	// modify the StateMachine with the custom StartState by making a new WorkflowDefinition (no pointer copy)
	newDef := *ogWorkflow.WorkflowDefinition
	newDef.StateMachine = &(*ogWorkflow.WorkflowDefinition.StateMachine)
	newDef.StateMachine.StartAt = startAt

	workflow := resources.NewWorkflow(&newDef, input, ogWorkflow.Namespace, ogWorkflow.Queue, ogWorkflow.Tags)
	logWorkflowStatusChange(workflow, "")

	stateResources, err := wm.getStateResources(workflow, ogWorkflow.Namespace)
	if err != nil {
		return &models.Workflow{}, err
	}

	err = wm.scheduleJobs(workflow, stateResources, input, ogWorkflow.Queue)
	if err != nil {
		return &models.Workflow{}, err
	}

	ogWorkflow.Retries = append(ogWorkflow.Retries, workflow.ID)
	workflow.RetryFor = ogWorkflow.ID

	// TODO: fails we should either
	// 1. reconcile somehow with the scheduled jobs
	// 2. kill the running jobs so that we don't have orphan jobs in AWS Batch
	err = wm.store.SaveWorkflow(*workflow)
	if err != nil {
		return workflow, err
	}

	// update originalWorfklow
	err = wm.store.UpdateWorkflow(ogWorkflow)
	if err != nil {
		// TODO: should we be failing here or just log?
		return workflow, err
	}

	return workflow, nil
}

func (wm BatchWorkflowManager) CancelWorkflow(workflow *models.Workflow, reason string) error {
	// TODO: don't cancel already succeeded jobs
	jobs := []*models.Job{}
	for _, job := range workflow.Jobs {
		switch job.Status {
		case models.JobStatusCreated,
			models.JobStatusQueued,
			models.JobStatusRunning,
			models.JobStatusWaitingForDeps:

			jobs = append(jobs, job)
		}
	}

	errs := wm.executor.Cancel(jobs, reason)
	if len(errs) < len(jobs) {
		// TODO: this assumes that a workflow is linear. One job cancellation
		// will lead to all subsequent jobs failing
		previousStatus := workflow.Status
		workflow.Status = models.WorkflowStatusCancelled
		logWorkflowStatusChange(workflow, previousStatus)
	}
	wm.store.UpdateWorkflow(*workflow)

	if len(errs) > 0 {
		return fmt.Errorf("%d of %d jobs were not canceled", len(errs), len(jobs))
	}

	return nil
}

func (wm BatchWorkflowManager) scheduleJobs(workflow *models.Workflow,
	stateResources map[string]*models.StateResource, input string, queue string) error {

	if err := resources.RemoveInactiveStates(workflow.WorkflowDefinition.StateMachine); err != nil {
		return err
	}

	orderedStates, err := resources.OrderedStates(workflow.WorkflowDefinition.StateMachine.States)
	if err != nil {
		return err
	}

	jobs := map[string]*models.Job{}

	for i, stateAndDeps := range orderedStates {
		stateName := stateAndDeps.StateName
		state := stateAndDeps.State
		deps := []string{}

		for _, d := range stateAndDeps.Deps {
			if _, ok := jobs[d]; !ok {
				return fmt.Errorf("Failed to start state %s. Dependency job for `%s` not found", stateName, d)
			}
			deps = append(deps, jobs[d].ID)
		}
		var jobID, jobName string
		var jobInput string
		var err error
		// TODO: this should be limited to 50 characters due to a bug in the interaction between Batch
		// and ECS. {namespace--app} for now, to enable easy parsing across workflows
		if stateResources[stateName].Namespace == "" {
			jobName = fmt.Sprintf("default--%s", stateResources[stateName].Name)
		} else {
			jobName = fmt.Sprintf("%s--%s",
				stateResources[stateName].Namespace, stateResources[stateName].Name)
		}
		jobDefinition := stateResources[stateName].URI

		// use workflow input to first job
		if i == 0 {
			jobInput = input
		}

		// determine if the state has a retry strategy
		// currently we just support a retry strategy corresponding to a number of attempts
		var attempts int64
		if len(state.Retry) == 1 {
			retrier := state.Retry[0]
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
		job := resources.NewJob(jobID, jobName, stateName, stateResources[stateName], jobInput)
		jobs[stateName] = job
		workflow.Jobs = append(workflow.Jobs, job)
	}

	return nil
}

// getStateResources fetches WorkflowDefinition URIs for each state
// from store.StateResource if namespace is set. If namespace is NOT
// defined then StateResource objects are created with URI = state.Resource
//
// This behavior allows to shortcircuit use of the StateResource database and provide
// Resource URIs directly in the WorkflowDefinition
func (wm BatchWorkflowManager) getStateResources(workflow *models.Workflow,
	namespace string) (map[string]*models.StateResource, error) {

	stateResources := map[string]*models.StateResource{}

	if namespace == "" {
		// assume State.Resource is a URI
		for stateName, state := range workflow.WorkflowDefinition.StateMachine.States {
			stateResources[stateName] = resources.NewBatchResource(
				stateName,
				"",
				state.Resource,
			)
		}

		return stateResources, nil
	}

	// fetch each of the StateResource objects using the namespace
	// and State.Resource name.
	// Could be faster with the store supporting a BatchGetStateResource([]names, namespace)
	orderedStates, err := resources.OrderedStates(workflow.WorkflowDefinition.StateMachine.States)
	if err != nil {
		return nil, err
	}
	for _, stateAndDeps := range orderedStates {
		state := stateAndDeps.State
		stateName := stateAndDeps.StateName
		stateResource, err := wm.store.GetStateResource(state.Resource, namespace)
		if err != nil {
			return stateResources, fmt.Errorf("StateResource `%s:%s` Not Found: %s",
				namespace, state.Resource, err)
		}
		stateResources[stateName] = &stateResource
	}

	return stateResources, nil
}
