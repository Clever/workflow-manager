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
