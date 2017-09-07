package memory

import (
	"testing"

	"github.com/Clever/workflow-manager/store/tests"
)

func TestMemoryStore(t *testing.T) {
	s := New()
	// GetWorkflowDefinitions does a primitive length check of the number of workflows received
	// We should run it first for it to pass
	t.Run("GetWorkflowDefinitions", tests.GetWorkflowDefinitions(s, t))
	t.Run("UpdateWorkflowDefinition", tests.UpdateWorkflowDefinition(s, t))
	t.Run("GetWorkflowDefinition", tests.GetWorkflowDefinition(s, t))
	t.Run("SaveWorkflowDefinition", tests.SaveWorkflowDefinition(s, t))
	t.Run("SaveStateResource", tests.SaveStateResource(s, t))
	t.Run("GetStateResource", tests.GetStateResource(s, t))
	t.Run("DeleteStateResource", tests.DeleteStateResource(s, t))
	t.Run("SaveJob", tests.SaveJob(s, t))
	t.Run("UpdateJob", tests.UpdateJob(s, t))
	t.Run("GetJob", tests.GetJob(s, t))
	t.Run("GetJobsForWorkflowDefinition", tests.GetJobsForWorkflowDefinition(s, t))
}
