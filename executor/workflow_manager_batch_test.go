package executor

import (
	"fmt"
	"testing"

	"github.com/Clever/workflow-manager/gen-go/models"
	"github.com/Clever/workflow-manager/resources"
	"github.com/Clever/workflow-manager/store/memory"
	uuid "github.com/satori/go.uuid"
	"github.com/stretchr/testify/assert"
)

type mockBatchClient struct {
	jobs map[string]*models.Job
}

func (be *mockBatchClient) SubmitWorkflow(name string, definition string, dependencies []string, input string, queue string, attempts int64) (string, error) {
	for _, d := range dependencies {
		if _, ok := be.jobs[d]; !ok {
			return "", fmt.Errorf("Dependency %s not found", d)
		}
	}
	jobID := uuid.NewV4().String()
	be.jobs[jobID] = &models.Job{
		ID:    jobID,
		Name:  name,
		Input: input,
	}

	return jobID, nil
}

func (be *mockBatchClient) Status(jobs []*models.Job) []error {
	errs := []error{}
	for i, job := range jobs {
		if _, ok := be.jobs[job.ID]; !ok {
			errs = append(errs, fmt.Errorf("%s", job.ID))
		} else {
			jobs[i].Status = be.jobs[job.ID].Status
		}
	}
	return errs
}

func (be *mockBatchClient) Cancel(jobs []*models.Job, reason string) []error {
	// mark first job as Cancelled
	if len(jobs) > 0 {
		jobs[0].Status = models.JobStatusAbortedByUser
	}

	return nil
}

func TestUpdateWorkflowStatus(t *testing.T) {
	mockClient := &mockBatchClient{
		map[string]*models.Job{},
	}
	jm := BatchWorkflowManager{
		mockClient,
		memory.New(),
	}
	wf := resources.KitchenSinkWorkflowDefinition(t)
	input := `["test-start-input"]`
	tags := map[string]interface{}{"team": "infra", "tag2": "value2"}
	workflow, err := jm.CreateWorkflow(*wf, input, "", "", tags)
	assert.NoError(t, err)

	t.Log("Workflow is QUEUED till a job starts RUNNING")
	assert.Equal(t, models.WorkflowStatusQueued, workflow.Status)

	// mark one job as running
	for _, job := range workflow.Jobs {
		mockClient.jobs[job.ID].Status = models.JobStatusRunning
		break
	}
	err = jm.UpdateWorkflowStatus(workflow)
	t.Log("Workflow is RUNNING when a job starts RUNNING")
	assert.NoError(t, err)
	assert.Equal(t, models.WorkflowStatusRunning, workflow.Status)

	// mark one job as failed
	for _, job := range workflow.Jobs {
		mockClient.jobs[job.ID].Status = models.JobStatusFailed
		break
	}
	err = jm.UpdateWorkflowStatus(workflow)
	t.Log("Workflow is FAILED if a job is FAILED")
	assert.NoError(t, err)
	assert.Equal(t, workflow.Status, models.WorkflowStatusFailed)

	// mark one job as success. should not mean success
	for _, job := range workflow.Jobs {
		mockClient.jobs[job.ID].Status = models.JobStatusSucceeded
		break
	}
	err = jm.UpdateWorkflowStatus(workflow)
	t.Log("One job SUCCESS does not result in workflow SUCCESS")
	assert.NoError(t, err)
	assert.NotEqual(t, workflow.Status, models.WorkflowStatusSucceeded)

	// mark all jobs as success. should mean workflow success
	for _, job := range workflow.Jobs {
		mockClient.jobs[job.ID].Status = models.JobStatusSucceeded
	}
	err = jm.UpdateWorkflowStatus(workflow)
	t.Log("Workflow is SUCCESSFUL if all jobs are SUCCESSFUL")
	assert.NoError(t, err)
	assert.Equal(t, workflow.Status, models.WorkflowStatusSucceeded)

	// mark one job as failed, others are successful. Still means failed
	for _, job := range workflow.Jobs {
		mockClient.jobs[job.ID].Status = models.JobStatusFailed
		break
	}
	workflow.Status = models.WorkflowStatusRunning
	err = jm.UpdateWorkflowStatus(workflow)
	t.Log("Workflow is FAILED if any job FAILS")
	assert.NoError(t, err)
	assert.Equal(t, workflow.Status, models.WorkflowStatusFailed)

	// mark jobs as aborted, this should still mean the workflow is failed
	for _, job := range workflow.Jobs {
		mockClient.jobs[job.ID].Status = models.JobStatusAbortedDepsFailed
	}
	workflow.Status = models.WorkflowStatusRunning
	err = jm.UpdateWorkflowStatus(workflow)
	t.Log("Workflow is FAILED if all jobs are aborted")
	assert.NoError(t, err)
	assert.Equal(t, workflow.Status, models.WorkflowStatusFailed)

	// mark a job as user-aborted, this should still mean the workflow is cancelled
	for _, job := range workflow.Jobs {
		mockClient.jobs[job.ID].Status = models.JobStatusAbortedByUser
		break
	}
	workflow.Status = models.WorkflowStatusRunning
	err = jm.UpdateWorkflowStatus(workflow)
	t.Log("Workflow is CANCELLED if any job is user aborted")
	assert.NoError(t, err)
	assert.Equal(t, workflow.Status, models.WorkflowStatusCancelled)

	t.Log("Workflows in a final'd state still return status after jobs no longer exists")
	stateMachine := &models.SLStateMachine{
		Comment: "description",
		StartAt: "only-state",
		States: map[string]models.SLState{
			"only-state": models.SLState{
				Next:     "",
				Resource: "fake-test-resource-1",
				End:      true,
			},
		},
	}
	wf, err = resources.NewWorkflowDefinition("test-worfklow", models.ManagerBatch, stateMachine)
	workflow, err = jm.CreateWorkflow(*wf, input, "", "", tags)
	assert.Nil(t, err)

	workflow.Status = models.WorkflowStatusCancelled
	// NOTE: the job is NOT added to the mockClient, so it is unknown
	for _, job := range workflow.Jobs {
		job.Status = models.JobStatusAbortedByUser
	}

	err = jm.UpdateWorkflowStatus(workflow)
	assert.NoError(t, err)
	assert.Equal(t, workflow.Status, models.WorkflowStatusCancelled)
}

// TestCancelUpdates ensures that a cancelling a workflow works
// and that the following updates behave as expected
func TestCancelUpdates(t *testing.T) {
	mockClient := &mockBatchClient{
		map[string]*models.Job{},
	}
	jm := BatchWorkflowManager{
		mockClient,
		memory.New(),
	}
	wf := resources.KitchenSinkWorkflowDefinition(t)
	input := `["test-start-input"]`

	tags := map[string]interface{}{"team": "infra", "tag2": "value2"}
	workflow, err := jm.CreateWorkflow(*wf, input, "", "", tags)
	assert.Nil(t, err)

	// mark all jobs as running
	for _, job := range workflow.Jobs {
		mockClient.jobs[job.ID].Status = models.JobStatusRunning
	}
	err = jm.UpdateWorkflowStatus(workflow)
	assert.NotEqual(t, models.WorkflowStatusCancelled, workflow.Status)

	// cancel the workflow
	t.Log("CancelWorkflow marks a workflow as Cancelled")
	err = jm.CancelWorkflow(workflow, "testing")
	assert.Nil(t, err)
	assert.Equal(t, models.WorkflowStatusCancelled, workflow.Status)

	// Status ensures that workflow is still marked as Cancelled
	err = jm.UpdateWorkflowStatus(workflow)
	assert.NoError(t, err)
	assert.Equal(t, models.WorkflowStatusCancelled, workflow.Status)

	t.Log("Canceled workflows don't un-cancel")
	// This depends on the previous case above it
	for _, job := range workflow.Jobs {
		mockClient.jobs[job.ID].Status = models.JobStatusSucceeded
	}
	err = jm.UpdateWorkflowStatus(workflow)
	assert.NoError(t, err)
	assert.Equal(t, models.WorkflowStatusCancelled, workflow.Status)
}

// TestCreateWorkflow tests that jobs are created for a workflow in the right order
// with the appropriate settings
func TestCreateWorkflow(t *testing.T) {
	mockClient := &mockBatchClient{
		map[string]*models.Job{},
	}
	store := memory.New()
	jm := BatchWorkflowManager{
		mockClient,
		store,
	}

	wf := resources.KitchenSinkWorkflowDefinition(t)
	input := `["test-start-input", "arg2"]`

	t.Log("CreateWorkflow without namespace")
	tags := map[string]interface{}{"team": "infra", "tag2": "value2"}
	workflow, err := jm.CreateWorkflow(*wf, input, "", "", tags)
	assert.Nil(t, err)
	assert.Equal(t, len(workflow.Jobs), len(workflow.WorkflowDefinition.StateMachine.States))

	t.Log("Input data is passed to the first job only")
	assert.NotEmpty(t, workflow.Jobs[0].Input, mockClient.jobs[workflow.Jobs[0].ID].Input)
	assert.Empty(t, workflow.Jobs[1].Input, mockClient.jobs[workflow.Jobs[1].ID].Input)
	assert.Equal(t, workflow.Jobs[0].Input, mockClient.jobs[workflow.Jobs[0].ID].Input)

	t.Log("CreateWorkflow using namespaces")
	for _, i := range []int{1, 2, 3} {
		stateResource := resources.NewBatchResource(
			fmt.Sprintf("fake-resource-%d", i),
			"my-env",
			fmt.Sprintf("arn:batch:jobdefinition:%d", i))
		store.SaveStateResource(*stateResource)
	}

	workflow, err = jm.CreateWorkflow(*wf, input, "my-env", "", tags)
	assert.Nil(t, err)
	assert.Equal(t, workflow.Jobs[0].Input, mockClient.jobs[workflow.Jobs[0].ID].Input)

	t.Log("CreateWorkflow using specific queue")
	for _, i := range []int{1, 2, 3} {
		stateResource := resources.NewBatchResource(
			fmt.Sprintf("fake-resource-%d", i),
			"my-env",
			fmt.Sprintf("arn:batch:jobdefinition:%d", i))
		store.SaveStateResource(*stateResource)
	}

	workflow, err = jm.CreateWorkflow(*wf, input, "", "custom-queue", tags)
	assert.Nil(t, err)
	assert.Equal(t, workflow.Jobs[0].Input, mockClient.jobs[workflow.Jobs[0].ID].Input)

	t.Log("CreateWorkflow uses startAt in workflow definition")
	wf.StateMachine.StartAt = "second-state"

	workflow, err = jm.CreateWorkflow(*wf, input, "my-env", "", tags)
	assert.Equal(t, workflow.Jobs[0].State, "second-state")
	assert.Equal(t, mockClient.jobs[workflow.Jobs[0].ID].Name, "my-env--fake-resource-2")
}

func TestRetryWorkflow(t *testing.T) {
	mockClient := &mockBatchClient{
		map[string]*models.Job{},
	}
	store := memory.New()
	jm := BatchWorkflowManager{
		mockClient,
		store,
	}

	wf := resources.KitchenSinkWorkflowDefinition(t)
	input := `["test-start-input", "arg2"]`
	tags := map[string]interface{}{"team": "infra", "tag2": "value2"}
	workflow, err := jm.CreateWorkflow(*wf, input, "", "test", tags)
	assert.Nil(t, err)

	t.Log("Retry for a running Workflow is not allowed")
	_, err = jm.RetryWorkflow(*workflow, "second-state", `["args"]`)
	assert.NotNil(t, err)

	workflow.Status = models.WorkflowStatusFailed
	for _, job := range workflow.Jobs {
		job.Status = models.JobStatusFailed
	}
	err = store.SaveWorkflow(*workflow)

	t.Log("Retry an existing Workflow if it is done")
	newWorkflow, err := jm.RetryWorkflow(*workflow, "second-state", `["args"]`)
	assert.Nil(t, err)
	ogWorkflow, err := store.GetWorkflowByID(workflow.ID)

	assert.Equal(t, workflow.ID, ogWorkflow.ID)
	assert.Equal(t, len(ogWorkflow.Jobs)-1, len(newWorkflow.Jobs))
	assert.Equal(t, ogWorkflow.ID, newWorkflow.RetryFor)
	assert.Contains(t, ogWorkflow.Retries, newWorkflow.ID)
}

// TestGetStateResources tests that the correct stateResources are set for
// for a Worflow.
func TestGetStateResources(t *testing.T) {
	mockClient := &mockBatchClient{
		map[string]*models.Job{},
	}
	store := memory.New()
	jm := BatchWorkflowManager{
		mockClient,
		store,
	}
	wf := resources.KitchenSinkWorkflowDefinition(t)
	input := `["test-start-input", "arg2"]`
	namespace := "my-env"
	queue := "queue"
	tags := map[string]interface{}{"team": "infra", "tag2": "value2"}

	t.Log("Works without providing a namespace for CreateWorkflow")
	stateResources, err := jm.getStateResources(resources.NewWorkflow(wf, input, namespace, queue, tags), "")
	assert.Nil(t, err)
	for k, stateResource := range stateResources {
		assert.Equal(t, wf.StateMachine.States[k].Resource, stateResource.URI)
	}

	t.Log("Fails when using a namespace for CreateWorkflow without StateResource")
	stateResources, err = jm.getStateResources(resources.NewWorkflow(wf, input, namespace, queue, tags), "does-not-exist")
	assert.Error(t, err, fmt.Sprintf("StateResource `%s:%s` Not Found: %s",
		"does-not-exist", "fake-resource-1", "does-not-exist--fake-resource-1"))

	t.Log("Works when using a namespace for CreateWorkflow")
	for _, i := range []int{1, 2, 3} {
		stateResource := resources.NewBatchResource(
			fmt.Sprintf("fake-resource-%d", i),
			namespace,
			fmt.Sprintf("arn:batch:jobdefinition:%d", i))
		store.SaveStateResource(*stateResource)
	}
	stateResources, err = jm.getStateResources(resources.NewWorkflow(wf, input, namespace, queue, tags), "my-env")
	assert.Nil(t, err)
	assert.Equal(t, stateResources["start-state"].Name, "fake-resource-1")
	assert.Equal(t, stateResources["start-state"].Namespace, "my-env")
	assert.Equal(t, stateResources["start-state"].URI, "arn:batch:jobdefinition:1")
	assert.Equal(t, stateResources["start-state"].Type, models.StateResourceTypeJobDefinitionARN)
}
