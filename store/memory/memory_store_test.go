package memory

import (
	"testing"

	"github.com/Clever/workflow-manager/store/tests"
)

func TestMemoryStore(t *testing.T) {
	s := New()
	// GetWorkflows does a primitive length check of the number of workflows received
	// We should run it first for it to pass
	t.Run("GetWorkflows", tests.GetWorkflows(s, t))
	t.Run("UpdateWorkflow", tests.UpdateWorkflow(s, t))
	t.Run("GetWorkflow", tests.GetWorkflow(s, t))
	t.Run("SaveWorkflow", tests.SaveWorkflow(s, t))
	t.Run("SaveStateResource", tests.SaveStateResource(s, t))
	t.Run("GetStateResource", tests.GetStateResource(s, t))
	t.Run("DeleteStateResource", tests.DeleteStateResource(s, t))
	t.Run("SaveJob", tests.SaveJob(s, t))
	t.Run("UpdateJob", tests.UpdateJob(s, t))
	t.Run("GetJob", tests.GetJob(s, t))
	t.Run("GetJobsForWorkflow", tests.GetJobsForWorkflow(s, t))
}
