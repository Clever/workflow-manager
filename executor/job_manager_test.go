package executor

import (
	"fmt"
	"testing"

	"github.com/Clever/workflow-manager/resources"
	"github.com/Clever/workflow-manager/store"
	uuid "github.com/satori/go.uuid"
	"github.com/stretchr/testify/assert"
)

type mockBatchClient struct {
	tasks map[string]string
}

func (be *mockBatchClient) SubmitJob(name string, definition string, dependencies []string, input string) (string, error) {
	for _, d := range dependencies {
		if _, ok := be.tasks[d]; !ok {
			return "", fmt.Errorf("Dependency %s not found", d)
		}
	}
	taskID := uuid.NewV4().String()
	be.tasks[taskID] = name

	return taskID, nil
}

func (be *mockBatchClient) Status(tasks []*resources.Task) []error {
	return nil
}

func TestUpdateJobStatus(t *testing.T) {
	jm := BatchJobManager{
		&mockBatchClient{
			map[string]string{},
		},
		store.NewMemoryStore(),
	}
	wf := resources.KitchenSinkWorkflow(t)
	input := "test-start-input"

	job, err := jm.CreateJob(wf, input)
	assert.Nil(t, err)

	t.Log("Job is QUEUED till a task starts RUNNING")
	assert.Equal(t, job.Status, resources.Queued)

	// mark one task as running
	for _, task := range job.Tasks {
		task.SetStatus(resources.TaskStatusRunning)
	}
	jm.UpdateJobStatus(job)
	t.Log("Job is RUNNING when a task starts RUNNING")
	assert.Equal(t, job.Status, resources.Running)

	// mark one task as failed
	for _, task := range job.Tasks {
		task.SetStatus(resources.TaskStatusFailed)
		break
	}
	jm.UpdateJobStatus(job)
	t.Log("Job is FAILED if a task is FAILED")
	assert.Equal(t, job.Status, resources.Failed)

	// mark one task as success. should not mean success
	for _, task := range job.Tasks {
		task.SetStatus(resources.TaskStatusSucceeded)
		break
	}
	jm.UpdateJobStatus(job)
	t.Log("One task SUCCESS does not result in job SUCCESS")
	assert.NotEqual(t, job.Status, resources.Succeded)

	// mark all tasks as success. should mean job success
	for _, task := range job.Tasks {
		task.SetStatus(resources.TaskStatusSucceeded)
	}
	jm.UpdateJobStatus(job)
	t.Log("Job is SUCCESSFUL if all tasks are SUCCESSFUL")
	assert.Equal(t, job.Status, resources.Succeded)

	// mark one task as failed, others are successful. Still means failed
	for _, task := range job.Tasks {
		task.SetStatus(resources.TaskStatusFailed)
		break
	}
	jm.UpdateJobStatus(job)
	t.Log("Job is FAILED if any task FAILS")
	assert.Equal(t, job.Status, resources.Failed)
}

// TestCreateJob tests that tasks are created for a job in the right order
// with the appropriate settings
func TestCreateJob(t *testing.T) {
	jm := BatchJobManager{
		&mockBatchClient{
			map[string]string{},
		},
		store.NewMemoryStore(),
	}

	wf := resources.KitchenSinkWorkflow(t)
	input := "test-start-input"

	job, err := jm.CreateJob(wf, input)
	assert.Nil(t, err)

	assert.Equal(t, len(job.Tasks), len(job.Workflow.States()))
}
