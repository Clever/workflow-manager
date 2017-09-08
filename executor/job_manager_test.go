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
	tasks map[string]*resources.Task
}

func (be *mockBatchClient) SubmitWorkflow(name string, definition string, dependencies, input []string, queue string, attempts int64) (string, error) {
	for _, d := range dependencies {
		if _, ok := be.tasks[d]; !ok {
			return "", fmt.Errorf("Dependency %s not found", d)
		}
	}
	taskID := uuid.NewV4().String()
	be.tasks[taskID] = &resources.Task{
		ID:    taskID,
		Name:  name,
		Input: input,
	}

	return taskID, nil
}

func (be *mockBatchClient) Status(tasks []*resources.Task) []error {
	errs := []error{}
	for _, t := range tasks {
		if _, ok := be.tasks[t.ID]; !ok {
			errs = append(errs, fmt.Errorf("%s", t.ID))
		} else {
			t.SetStatus(be.tasks[t.ID].Status)
		}
	}
	return errs
}

func (be *mockBatchClient) Cancel(tasks []*resources.Task, reason string) []error {
	// mark first task as Cancelled
	if len(tasks) > 0 {
		tasks[0].Status = resources.TaskStatusUserAborted
	}

	return nil
}

func TestUpdateWorkflowStatus(t *testing.T) {
	mockClient := &mockBatchClient{
		map[string]*resources.Task{},
	}
	jm := BatchWorkflowManager{
		mockClient,
		memory.New(),
	}
	wf := resources.KitchenSinkWorkflowDefinition(t)
	input := []string{"test-start-input"}

	workflow, err := jm.CreateWorkflow(wf, input, "", "")
	assert.NoError(t, err)

	t.Log("Workflow is QUEUED till a task starts RUNNING")
	assert.Equal(t, resources.Queued, workflow.Status)

	// mark one task as running
	for _, task := range workflow.Tasks {
		mockClient.tasks[task.ID].SetStatus(resources.TaskStatusRunning)
		break
	}
	err = jm.UpdateWorkflowStatus(workflow)
	t.Log("Workflow is RUNNING when a task starts RUNNING")
	assert.NoError(t, err)
	assert.Equal(t, resources.Running, workflow.Status)

	// mark one task as failed
	for _, task := range workflow.Tasks {
		mockClient.tasks[task.ID].SetStatus(resources.TaskStatusFailed)
		break
	}
	err = jm.UpdateWorkflowStatus(workflow)
	t.Log("Workflow is FAILED if a task is FAILED")
	assert.NoError(t, err)
	assert.Equal(t, workflow.Status, resources.Failed)

	// mark one task as success. should not mean success
	for _, task := range workflow.Tasks {
		mockClient.tasks[task.ID].SetStatus(resources.TaskStatusSucceeded)
		break
	}
	err = jm.UpdateWorkflowStatus(workflow)
	t.Log("One task SUCCESS does not result in workflow SUCCESS")
	assert.NoError(t, err)
	assert.NotEqual(t, workflow.Status, resources.Succeeded)

	// mark all tasks as success. should mean workflow success
	for _, task := range workflow.Tasks {
		mockClient.tasks[task.ID].SetStatus(resources.TaskStatusSucceeded)
	}
	err = jm.UpdateWorkflowStatus(workflow)
	t.Log("Workflow is SUCCESSFUL if all tasks are SUCCESSFUL")
	assert.NoError(t, err)
	assert.Equal(t, workflow.Status, resources.Succeeded)

	// mark one task as failed, others are successful. Still means failed
	for _, task := range workflow.Tasks {
		mockClient.tasks[task.ID].SetStatus(resources.TaskStatusFailed)
		break
	}
	workflow.Status = resources.Running
	err = jm.UpdateWorkflowStatus(workflow)
	t.Log("Workflow is FAILED if any task FAILS")
	assert.NoError(t, err)
	assert.Equal(t, workflow.Status, resources.Failed)

	// mark tasks as aborted, this should still mean the workflow is failed
	for _, task := range workflow.Tasks {
		mockClient.tasks[task.ID].SetStatus(resources.TaskStatusAborted)
	}
	workflow.Status = resources.Running
	err = jm.UpdateWorkflowStatus(workflow)
	t.Log("Workflow is FAILED if all tasks are aborted")
	assert.NoError(t, err)
	assert.Equal(t, workflow.Status, resources.Failed)

	// mark a task as user-aborted, this should still mean the workflow is cancelled
	for _, task := range workflow.Tasks {
		mockClient.tasks[task.ID].SetStatus(resources.TaskStatusUserAborted)
		break
	}
	workflow.Status = resources.Running
	err = jm.UpdateWorkflowStatus(workflow)
	t.Log("Workflow is CANCELLED if any task is user aborted")
	assert.NoError(t, err)
	assert.Equal(t, workflow.Status, resources.Cancelled)

	t.Log("Workflows in a final'd state still return status after tasks no longer exists")
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
	// NOTE: the task is NOT added to the mockClient, so it is unknown
	for _, task := range workflow.Tasks {
		task.Status = resources.TaskStatusUserAborted
	}

	err = jm.UpdateWorkflowStatus(workflow)
	assert.NoError(t, err)
	assert.Equal(t, workflow.Status, resources.Cancelled)
}

// TestCancelUpdates ensures that a cancelling a workflow works
// and that the following updates behave as expected
func TestCancelUpdates(t *testing.T) {
	mockClient := &mockBatchClient{
		map[string]*resources.Task{},
	}
	jm := BatchWorkflowManager{
		mockClient,
		memory.New(),
	}
	wf := resources.KitchenSinkWorkflowDefinition(t)
	input := []string{"test-start-input"}

	workflow, err := jm.CreateWorkflow(wf, input, "", "")
	assert.Nil(t, err)

	// mark all tasks as running
	for _, task := range workflow.Tasks {
		mockClient.tasks[task.ID].SetStatus(resources.TaskStatusRunning)
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
	for _, task := range workflow.Tasks {
		mockClient.tasks[task.ID].SetStatus(resources.TaskStatusSucceeded)
	}
	err = jm.UpdateWorkflowStatus(workflow)
	assert.NoError(t, err)
	assert.Equal(t, resources.Cancelled, workflow.Status)
}

// TestCreateWorkflow tests that tasks are created for a workflow in the right order
// with the appropriate settings
func TestCreateWorkflow(t *testing.T) {
	mockClient := &mockBatchClient{
		map[string]*resources.Task{},
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

	assert.Equal(t, len(workflow.Tasks), len(workflow.WorkflowDefinition.States()))

	t.Log("Input data is passed to the first task only")
	assert.NotEmpty(t, workflow.Tasks[0].Input, mockClient.tasks[workflow.Tasks[0].ID].Input)
	assert.Empty(t, workflow.Tasks[1].Input, mockClient.tasks[workflow.Tasks[1].ID].Input)
	assert.Equal(t, workflow.Tasks[0].Input, mockClient.tasks[workflow.Tasks[0].ID].Input)

	t.Log("CreateWorkflow using namespaces")
	for _, i := range []int{1, 2, 3} {
		store.SaveStateResource(resources.NewBatchResource(
			fmt.Sprintf("fake-resource-%d", i),
			"my-env",
			fmt.Sprintf("arn:batch:jobdefinition:%d", i)))
	}

	workflow, err = jm.CreateWorkflow(wf, input, "my-env", "")
	assert.Nil(t, err)
	assert.Equal(t, workflow.Tasks[0].Input, mockClient.tasks[workflow.Tasks[0].ID].Input)

	t.Log("CreateWorkflow using specific queue")
	for _, i := range []int{1, 2, 3} {
		store.SaveStateResource(resources.NewBatchResource(
			fmt.Sprintf("fake-resource-%d", i),
			"my-env",
			fmt.Sprintf("arn:batch:jobdefinition:%d", i)))
	}

	workflow, err = jm.CreateWorkflow(wf, input, "", "custom-queue")
	assert.Nil(t, err)
	assert.Equal(t, workflow.Tasks[0].Input, mockClient.tasks[workflow.Tasks[0].ID].Input)

}

// TestGetStateResources tests that the correct stateResources are set for
// for a Worflow.
func TestGetStateResources(t *testing.T) {
	mockClient := &mockBatchClient{
		map[string]*resources.Task{},
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
