package executor

import (
	"fmt"
	"testing"
	"time"

	"github.com/Clever/workflow-manager/resources"
	"github.com/Clever/workflow-manager/store/memory"
	uuid "github.com/satori/go.uuid"
	"github.com/stretchr/testify/assert"
)

type mockBatchClient struct {
	jobs map[string]*resources.Job
}

func (be *mockBatchClient) SubmitWorkflow(name string, definition string, dependencies, input []string, queue string, attempts int64) (string, error) {
	for _, d := range dependencies {
		if _, ok := be.jobs[d]; !ok {
			return "", fmt.Errorf("Dependency %s not found", d)
		}
	}
	jobID := uuid.NewV4().String()
	be.jobs[jobID] = &resources.Job{
		ID:   jobID,
		Name: name,
		JobDetail: resources.JobDetail{
			Input: input,
		},
	}

	return jobID, nil
}

func (be *mockBatchClient) Status(jobs []*resources.Job) []error {
	errs := []error{}
	for _, t := range jobs {
		if _, ok := be.jobs[t.ID]; !ok {
			errs = append(errs, fmt.Errorf("%s", t.ID))
		} else {
			t.SetStatus(be.jobs[t.ID].Status)
		}
	}
	return errs
}

func (be *mockBatchClient) Cancel(jobs []*resources.Job, reason string) []error {
	// mark first job as Cancelled
	if len(jobs) > 0 {
		jobs[0].Status = resources.JobStatusUserAborted
	}

	return nil
}

func TestUpdateWorkflowStatus(t *testing.T) {
	mockClient := &mockBatchClient{
		map[string]*resources.Job{},
	}
	jm := BatchWorkflowManager{
		mockClient,
		memory.New(),
	}
	wf := resources.KitchenSinkWorkflowDefinition(t)
	input := []string{"test-start-input"}

	workflow, err := jm.CreateWorkflow(wf, input, "", "")
	assert.NoError(t, err)

	t.Log("Workflow is QUEUED till a job starts RUNNING")
	assert.Equal(t, resources.Queued, workflow.Status)

	// mark one job as running
	for _, job := range workflow.Jobs {
		mockClient.jobs[job.ID].SetStatus(resources.JobStatusRunning)
		break
	}
	err = jm.UpdateWorkflowStatus(workflow)
	t.Log("Workflow is RUNNING when a job starts RUNNING")
	assert.NoError(t, err)
	assert.Equal(t, resources.Running, workflow.Status)

	// mark one job as failed
	for _, job := range workflow.Jobs {
		mockClient.jobs[job.ID].SetStatus(resources.JobStatusFailed)
		break
	}
	err = jm.UpdateWorkflowStatus(workflow)
	t.Log("Workflow is FAILED if a job is FAILED")
	assert.NoError(t, err)
	assert.Equal(t, workflow.Status, resources.Failed)

	// mark one job as success. should not mean success
	for _, job := range workflow.Jobs {
		mockClient.jobs[job.ID].SetStatus(resources.JobStatusSucceeded)
		break
	}
	err = jm.UpdateWorkflowStatus(workflow)
	t.Log("One job SUCCESS does not result in workflow SUCCESS")
	assert.NoError(t, err)
	assert.NotEqual(t, workflow.Status, resources.Succeeded)

	// mark all jobs as success. should mean workflow success
	for _, job := range workflow.Jobs {
		mockClient.jobs[job.ID].SetStatus(resources.JobStatusSucceeded)
	}
	err = jm.UpdateWorkflowStatus(workflow)
	t.Log("Workflow is SUCCESSFUL if all jobs are SUCCESSFUL")
	assert.NoError(t, err)
	assert.Equal(t, workflow.Status, resources.Succeeded)

	// mark one job as failed, others are successful. Still means failed
	for _, job := range workflow.Jobs {
		mockClient.jobs[job.ID].SetStatus(resources.JobStatusFailed)
		break
	}
	workflow.Status = resources.Running
	err = jm.UpdateWorkflowStatus(workflow)
	t.Log("Workflow is FAILED if any job FAILS")
	assert.NoError(t, err)
	assert.Equal(t, workflow.Status, resources.Failed)

	// mark jobs as aborted, this should still mean the workflow is failed
	for _, job := range workflow.Jobs {
		mockClient.jobs[job.ID].SetStatus(resources.JobStatusAborted)
	}
	workflow.Status = resources.Running
	err = jm.UpdateWorkflowStatus(workflow)
	t.Log("Workflow is FAILED if all jobs are aborted")
	assert.NoError(t, err)
	assert.Equal(t, workflow.Status, resources.Failed)

	// mark a job as user-aborted, this should still mean the workflow is cancelled
	for _, job := range workflow.Jobs {
		mockClient.jobs[job.ID].SetStatus(resources.JobStatusUserAborted)
		break
	}
	workflow.Status = resources.Running
	err = jm.UpdateWorkflowStatus(workflow)
	t.Log("Workflow is CANCELLED if any job is user aborted")
	assert.NoError(t, err)
	assert.Equal(t, workflow.Status, resources.Cancelled)

	t.Log("Workflows in a final'd state still return status after jobs no longer exists")
	states := map[string]resources.State{
		"only-state": &resources.WorkerState{
			NameStr:         "only-state",
			NextStr:         "",
			ResourceStr:     "fake-test-resource-1",
			DependenciesArr: []string{},
			End:             true,
		},
	}
	wf, err = resources.NewWorkflowDefinition("test-worfklow", "description", time.Now().Format(time.RFC3339Nano), states)
	workflow, err = jm.CreateWorkflow(wf, input, "", "")
	assert.Nil(t, err)

	workflow.Status = resources.Cancelled
	// NOTE: the job is NOT added to the mockClient, so it is unknown
	for _, job := range workflow.Jobs {
		job.Status = resources.JobStatusUserAborted
	}

	err = jm.UpdateWorkflowStatus(workflow)
	assert.NoError(t, err)
	assert.Equal(t, workflow.Status, resources.Cancelled)
}

// TestCancelUpdates ensures that a cancelling a workflow works
// and that the following updates behave as expected
func TestCancelUpdates(t *testing.T) {
	mockClient := &mockBatchClient{
		map[string]*resources.Job{},
	}
	jm := BatchWorkflowManager{
		mockClient,
		memory.New(),
	}
	wf := resources.KitchenSinkWorkflowDefinition(t)
	input := []string{"test-start-input"}

	workflow, err := jm.CreateWorkflow(wf, input, "", "")
	assert.Nil(t, err)

	// mark all jobs as running
	for _, job := range workflow.Jobs {
		mockClient.jobs[job.ID].SetStatus(resources.JobStatusRunning)
	}
	err = jm.UpdateWorkflowStatus(workflow)
	assert.NotEqual(t, resources.Cancelled, workflow.Status)

	// cancel the workflow
	t.Log("CancelWorkflow marks a workflow as Cancelled")
	err = jm.CancelWorkflow(workflow, "testing")
	assert.Nil(t, err)
	assert.Equal(t, resources.Cancelled, workflow.Status)

	// UpdateStatus ensures that workflow is still marked as Cancelled
	err = jm.UpdateWorkflowStatus(workflow)
	assert.NoError(t, err)
	assert.Equal(t, resources.Cancelled, workflow.Status)

	t.Log("Canceled workflows don't un-cancel")
	// This depends on the previous case above it
	for _, job := range workflow.Jobs {
		mockClient.jobs[job.ID].SetStatus(resources.JobStatusSucceeded)
	}
	err = jm.UpdateWorkflowStatus(workflow)
	assert.NoError(t, err)
	assert.Equal(t, resources.Cancelled, workflow.Status)
}

// TestCreateWorkflow tests that jobs are created for a workflow in the right order
// with the appropriate settings
func TestCreateWorkflow(t *testing.T) {
	mockClient := &mockBatchClient{
		map[string]*resources.Job{},
	}
	store := memory.New()
	jm := BatchWorkflowManager{
		mockClient,
		store,
	}

	wf := resources.KitchenSinkWorkflowDefinition(t)
	input := []string{"test-start-input", "arg2"}

	t.Log("CreateWorkflow without namespace")
	workflow, err := jm.CreateWorkflow(wf, input, "", "")
	assert.Nil(t, err)

	assert.Equal(t, len(workflow.Jobs), len(workflow.WorkflowDefinition.States()))

	t.Log("Input data is passed to the first job only")
	assert.NotEmpty(t, workflow.Jobs[0].Input, mockClient.jobs[workflow.Jobs[0].ID].Input)
	assert.Empty(t, workflow.Jobs[1].Input, mockClient.jobs[workflow.Jobs[1].ID].Input)
	assert.Equal(t, workflow.Jobs[0].Input, mockClient.jobs[workflow.Jobs[0].ID].Input)

	t.Log("CreateWorkflow using namespaces")
	for _, i := range []int{1, 2, 3} {
		store.SaveStateResource(resources.NewBatchResource(
			fmt.Sprintf("fake-resource-%d", i),
			"my-env",
			fmt.Sprintf("arn:batch:jobdefinition:%d", i)))
	}

	workflow, err = jm.CreateWorkflow(wf, input, "my-env", "")
	assert.Nil(t, err)
	assert.Equal(t, workflow.Jobs[0].Input, mockClient.jobs[workflow.Jobs[0].ID].Input)

	t.Log("CreateWorkflow using specific queue")
	for _, i := range []int{1, 2, 3} {
		store.SaveStateResource(resources.NewBatchResource(
			fmt.Sprintf("fake-resource-%d", i),
			"my-env",
			fmt.Sprintf("arn:batch:jobdefinition:%d", i)))
	}

	workflow, err = jm.CreateWorkflow(wf, input, "", "custom-queue")
	assert.Nil(t, err)
	assert.Equal(t, workflow.Jobs[0].Input, mockClient.jobs[workflow.Jobs[0].ID].Input)

}

// TestGetStateResources tests that the correct stateResources are set for
// for a Worflow.
func TestGetStateResources(t *testing.T) {
	mockClient := &mockBatchClient{
		map[string]*resources.Job{},
	}
	store := memory.New()
	jm := BatchWorkflowManager{
		mockClient,
		store,
	}
	wf := resources.KitchenSinkWorkflowDefinition(t)
	input := []string{"test-start-input", "arg2"}

	t.Log("Works without providing a namespace for CreateWorkflow")
	stateResources, err := jm.getStateResources(resources.NewWorkflow(wf, input), "")
	assert.Nil(t, err)
	for k, stateResource := range stateResources {
		assert.Equal(t, wf.StatesMap[k].Resource(), stateResource.URI)
	}

	t.Log("Fails when using a namespace for CreateWorkflow without StateResource")
	stateResources, err = jm.getStateResources(resources.NewWorkflow(wf, input), "does-not-exist")
	assert.Error(t, err, fmt.Sprintf("StateResource `%s:%s` Not Found: %s",
		"does-not-exist", "fake-resource-1", "does-not-exist--fake-resource-1"))

	t.Log("Works when using a namespace for CreateWorkflow")
	for _, i := range []int{1, 2, 3} {
		store.SaveStateResource(resources.NewBatchResource(
			fmt.Sprintf("fake-resource-%d", i),
			"my-env",
			fmt.Sprintf("arn:batch:jobdefinition:%d", i)))
	}
	stateResources, err = jm.getStateResources(resources.NewWorkflow(wf, input), "my-env")
	assert.Nil(t, err)
	assert.Equal(t, stateResources["start-state"].Name, "fake-resource-1")
	assert.Equal(t, stateResources["start-state"].Namespace, "my-env")
	assert.Equal(t, stateResources["start-state"].URI, "arn:batch:jobdefinition:1")
	assert.Equal(t, stateResources["start-state"].Type, resources.AWSBatchJobDefinition)
}
